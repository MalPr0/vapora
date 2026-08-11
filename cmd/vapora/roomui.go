package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/MalPr0/vapora/internal/tui"
	"github.com/MalPr0/vapora/pkg/punch"
)

// runRoomUI is the full screen front end for a room. It is the same view the
// two way chat uses: a room is a roster of several rather than a different
// screen, and one participant is just the small case of that.
func runRoomUI(ctx context.Context, open *room) error {
	terminal, err := tui.OpenTerminal(os.Stdin, os.Stdout)
	if err != nil {
		return fmt.Errorf("%w: %w", errNoTerminal, err)
	}

	defer func() {
		terminal.Restore()
		open.report()
	}()

	chat := tui.NewChat(terminal, open.room.Me().Name)
	chat.OnSend(open.room.Broadcast)
	chat.OnTyping(open.room.SetTyping)
	chat.OnCommand(func(line string) bool { return open.command(line, chat) })
	open.room.Observe(roomObserver{chat: chat})

	uiCtx, stopUI := context.WithCancel(ctx)
	defer stopUI()

	done := make(chan error, 1)
	go func() { done <- chat.Run(uiCtx) }()

	go func() {
		if err := open.connect(uiCtx, chat); err != nil {
			if errors.Is(err, context.Canceled) {
				return // the user quit while waiting; report() says so already
			}
			open.note(err.Error())
			chat.Closed(err.Error())
			return
		}
		chat.Connected(open.room.Me().Name)
		go open.pushRoster(uiCtx, chat)
	}()

	return <-done
}

// pushRoster keeps the header current. Everything about a path is polled, and
// so is who is present: nothing arrives to announce that somebody stopped
// arriving.
func (r *room) pushRoster(ctx context.Context, chat *tui.Chat) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	known := map[punch.PublicKey]string{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			members := r.room.Members()

			participants := make([]tui.Participant, 0, len(members))
			present := map[punch.PublicKey]string{}
			for _, member := range members {
				present[member.Key] = member.Name
				if _, seen := known[member.Key]; !seen {
					chat.System(member.Name + " joined")
				}
				participants = append(participants, tui.Participant{
					Name:    member.Name,
					Link:    linkState(member.Health.Link),
					RTT:     member.Health.RTT,
					Silence: member.Health.Silence,
				})
			}
			for key, name := range known {
				if _, still := present[key]; !still {
					chat.System(name + " left")
				}
			}
			known = present
			chat.SetMembers(participants)

			if !r.quorum(stillHere(members)) {
				chat.System("everybody left, so the room is closing")
				chat.Quit()
				return
			}
		}
	}
}

// command handles the lines that are instructions rather than conversation.
func (r *room) command(line string, chat *tui.Chat) bool {
	switch {
	case isExit(line):
		r.room.Goodbye()
		chat.Quit()
		return true
	case trimmed(line) == "!who":
		chat.System(r.describeMembers())
		return true
	case r.pasteWhileWaiting(line, chat):
		return true
	case trimmed(line) == "!invite":
		r.mu.Lock()
		invite := r.shared
		r.mu.Unlock()

		if invite == "" {
			chat.System("no address to put on an invite yet")
			return true
		}
		chat.System(invite)
		return true
	}
	return false
}

// pasteWhileWaiting takes an address the other side sent back. It only applies
// before anybody is here: once the room has members, a line that looks like an
// address is far more likely to be somebody talking about one.
func (r *room) pasteWhileWaiting(line string, chat *tui.Chat) bool {
	if strings.HasPrefix(strings.TrimSpace(line), "!") || len(r.room.Members()) > 0 {
		return false
	}

	where, ok := r.reach(context.Background(), line)
	if !ok {
		return false
	}
	chat.System("punching towards " + where + ", they have to be running too")
	return true
}

func (r *room) describeMembers() string {
	members := r.room.Members()
	if len(members) == 0 {
		return "nobody else is here yet"
	}

	description := fmt.Sprintf("%d present:", len(members))
	for _, member := range members {
		description += fmt.Sprintf("  %s (%s)", member.Name, member.Health.Link)
	}
	return description
}

// roomObserver hands what the room reports to the view, which already names
// every speaker.
type roomObserver struct {
	chat *tui.Chat
}

func (o roomObserver) Message(from punch.Member, payload string) {
	o.chat.Message(from.Name, payload)
}

func (o roomObserver) Typing(from punch.Member, active bool) {
	o.chat.Typing(from.Name, active)
}
