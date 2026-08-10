package tui

import (
	"context"
	"os"
	"sync"
	"time"
)

const (
	// frameInterval is the animation clock. Twelve frames a second is enough
	// for a walk cycle and cheap enough to leave the socket alone.
	frameInterval = 80 * time.Millisecond
	// typingIdle is how long after the last keystroke the peer stops being
	// told that a line is in progress.
	typingIdle = 2 * time.Second
)

// Chat is the running UI. It owns the state and serialises every change through
// one goroutine, so the renderer never sees a half applied update.
type Chat struct {
	terminal *Terminal
	screen   *Screen

	mu    sync.Mutex
	state State

	editor    Editor
	events    chan func()
	sendLine  func(string)
	setTyping func(bool)
	command   func(string) bool
	quit      chan struct{}
}

func NewChat(terminal *Terminal, me, peer string) *Chat {
	width, height := terminal.Size()
	return &Chat{
		terminal: terminal,
		screen:   NewScreen(width, height),
		state:    State{Phase: PhaseLoading, Me: me, Peer: peer, Status: "starting"},
		events:   make(chan func(), 64),
		quit:     make(chan struct{}),
	}
}

// OnSend and OnTyping are how the session is driven from the keyboard. They are
// injected rather than imported so the UI stays testable and knows nothing
// about sockets.
func (c *Chat) OnSend(send func(string))   { c.sendLine = send }
func (c *Chat) OnTyping(typing func(bool)) { c.setTyping = typing }

// OnCommand gets each line before it becomes a message. Returning true consumes
// it, so a command is neither shown in the conversation nor sent to the peer.
func (c *Chat) OnCommand(handler func(string) bool) { c.command = handler }

// Quit ends the UI from anywhere, which is what a command needs to do.
func (c *Chat) Quit() {
	select {
	case <-c.quit:
	default:
		close(c.quit)
	}
}

func (c *Chat) dispatch(mutate func()) {
	select {
	case c.events <- mutate:
	default: // a full queue means the UI is gone; dropping beats blocking a reader
	}
}

func (c *Chat) update(mutate func(*State)) {
	c.dispatch(func() {
		c.mu.Lock()
		mutate(&c.state)
		c.mu.Unlock()
	})
}

func (c *Chat) SetStatus(status string, progress float64) {
	c.update(func(s *State) {
		s.Status = status
		s.Progress = progress
	})
}

func (c *Chat) SetProgress(progress float64) {
	c.update(func(s *State) { s.Progress = progress })
}

// SetLink records what the transport reports about the path.
func (c *Chat) SetLink(link LinkState, rtt, silence time.Duration) {
	c.update(func(s *State) {
		s.Link = link
		s.RTT = rtt
		s.Silence = silence
	})
}

func (c *Chat) SetInvite(invite string) {
	c.update(func(s *State) { s.Invite = invite })
}

func (c *Chat) Connected(me, peer string) {
	c.update(func(s *State) {
		s.Phase = PhaseChat
		s.Me, s.Peer = me, peer
		s.Messages = append(s.Messages, Message{Speaker: "--", Body: "channel open", System: true})
	})
}

func (c *Chat) Closed(reason string) {
	c.update(func(s *State) {
		s.Phase = PhaseClosed
		s.ClosedError = reason
	})
}

// Message and Typing satisfy the session observer.
func (c *Chat) Message(payload string) {
	c.update(func(s *State) {
		s.PeerTyping = false
		s.Messages = append(s.Messages, Message{Speaker: s.Peer, Body: payload})
		keepPosition(s)
	})
}

func (c *Chat) Typing(active bool) {
	c.update(func(s *State) { s.PeerTyping = active })
}

func (c *Chat) System(body string) {
	c.update(func(s *State) {
		s.Messages = append(s.Messages, Message{Speaker: "--", Body: body, System: true})
		keepPosition(s)
	})
}

// Run draws until the context is cancelled or the user quits.
func (c *Chat) Run(ctx context.Context) error {
	keys := make(chan Key, 32)
	go readKeys(c.terminal.In(), keys)

	ticker := time.NewTicker(frameInterval)
	defer ticker.Stop()

	var lastKeystroke time.Time
	typingSent := false

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-c.quit:
			return nil

		case mutate := <-c.events:
			mutate()

		case <-c.terminal.Resized():
			width, height := c.terminal.Size()
			c.screen.Resize(width, height)

		case key := <-keys:
			if key.Kind == KeyInterrupt {
				return nil
			}
			if key.Kind == KeyEnter {
				if line := c.editor.Take(); line != "" {
					if c.command != nil && c.command(line) {
						break
					}
					c.appendMine(line)
					if c.sendLine != nil {
						c.sendLine(line)
					}
				}
				if typingSent {
					c.notifyTyping(false)
					typingSent = false
				}
				break
			}
			if key.Kind == KeyPageUp || key.Kind == KeyPageDown {
				c.scroll(key.Kind)
				break
			}
			c.editor.Apply(key)
			lastKeystroke = time.Now()
			if !typingSent && !c.editor.Empty() {
				c.notifyTyping(true)
				typingSent = true
			}

		case <-ticker.C:
			if typingSent && (c.editor.Empty() || time.Since(lastKeystroke) > typingIdle) {
				c.notifyTyping(false)
				typingSent = false
			}
			c.mu.Lock()
			c.state.Frame++
			c.state.Input = c.editor.String()
			c.state.Cursor = c.editor.Cursor()
			snapshot := c.state
			c.mu.Unlock()

			Draw(c.screen, snapshot)
			if err := c.screen.Flush(c.terminal.Out()); err != nil {
				return err
			}
		}
	}
}

// scroll moves the history window by half a screen, which keeps a couple of
// lines of overlap so the eye can follow where it landed.
func (c *Chat) scroll(kind KeyKind) {
	width, height := c.screen.Size()

	c.mu.Lock()
	defer c.mu.Unlock()

	step := (height - headerHeight - footerHeight) / 2
	if step < 1 {
		step = 1
	}
	if kind == KeyPageUp {
		c.state.Scroll += step
	} else {
		c.state.Scroll -= step
	}

	if maximum := MaxScroll(c.state, width, height); c.state.Scroll > maximum {
		c.state.Scroll = maximum
	}
	if c.state.Scroll < 0 {
		c.state.Scroll = 0
	}
}

// keepPosition holds the view still when a line arrives while the user is
// reading history. The estimate is one line per message, so a long wrapped
// message shifts the view slightly; scrolling back to the bottom resets it.
func keepPosition(s *State) {
	if s.Scroll > 0 {
		s.Scroll++
	}
}

func (c *Chat) appendMine(line string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state.Messages = append(c.state.Messages, Message{Speaker: c.state.Me, Body: line, Mine: true})
	// Sending is an act of joining the conversation, so it always returns the
	// view to the newest line.
	c.state.Scroll = 0
}

func (c *Chat) notifyTyping(active bool) {
	if c.setTyping != nil {
		c.setTyping(active)
	}
}

func readKeys(in *os.File, out chan<- Key) {
	buffer := make([]byte, 0, 64)
	chunk := make([]byte, 32)

	for {
		n, err := in.Read(chunk)
		if n > 0 {
			buffer = append(buffer, chunk[:n]...)
			for len(buffer) > 0 {
				key, consumed := DecodeKey(buffer)
				if consumed == 0 {
					break // a partial sequence: wait for the rest
				}
				buffer = buffer[consumed:]
				if key.Kind != KeyUnknown {
					out <- key
				}
			}
		}
		if err != nil {
			return
		}
	}
}
