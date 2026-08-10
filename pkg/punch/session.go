package punch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

const (
	punchInterval  = 300 * time.Millisecond
	readBufferSize = 2048
)

// ErrPunchTimeout means no packet ever arrived from the peer, which usually
// says the other side was not punching at the same time.
var ErrPunchTimeout = errors.New("punch: the peer never answered")

// Session is a direct UDP path to a peer. The peer may be known upfront, from
// an invite, or learned from the first packet that makes it through the NAT.
type Session struct {
	conn   *net.UDPConn
	codec  Codec
	output io.Writer

	mu       sync.RWMutex
	peer     *net.UDPAddr
	open     bool
	pending  []string
	observer Observer

	lastHeard  time.Time
	rtt        time.Duration
	pingSeq    uint64
	pingSentAt time.Time
}

func NewSession(conn *net.UDPConn, codec Codec, output io.Writer) *Session {
	return &Session{conn: conn, codec: codec, output: output, observer: writerObserver{output}}
}

// Observe redirects what arrives to a consumer that renders it itself.
func (s *Session) Observe(observer Observer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.observer = observer
}

func (s *Session) events() Observer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.observer
}

// SetTyping tells the peer whether a line is in progress here.
func (s *Session) SetTyping(active bool) {
	payload := ""
	if active {
		payload = "1"
	}
	s.send(kindTyping, payload)
}

func (s *Session) SetPeer(peer *net.UDPAddr) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.peer = peer
}

func (s *Session) Peer() *net.UDPAddr {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.peer
}

// Open punches towards the peer while accepting packets from anyone. A peer set
// later, by pasting an invite, is picked up by the running punch loop.
func (s *Session) Open(ctx context.Context, timeout time.Duration) error {
	openCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	go s.punchLoop(openCtx)

	buffer := make([]byte, readBufferSize)
	for {
		if err := s.conn.SetReadDeadline(time.Now().Add(punchInterval)); err != nil {
			return fmt.Errorf("punch: cannot set read deadline: %w", err)
		}

		n, from, err := s.conn.ReadFromUDP(buffer)
		if err != nil {
			if openCtx.Err() == nil {
				continue
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return ErrPunchTimeout
		}

		// A frame that does not authenticate is someone else's packet: the
		// secret is what keeps a stranger from becoming the peer.
		kind, _, err := s.codec.Open(buffer[:n])
		if err != nil || (kind != kindPunch && kind != kindAck) {
			continue
		}

		peer := s.Peer()
		if peer != nil && !sameEndpoint(peer, from) {
			continue
		}
		if peer == nil {
			// The invite went one way only and their packet still got in.
			// Announcing it is the caller's business: a session that writes to
			// a stream of its own lands in the middle of whatever a UI drew.
			s.SetPeer(from)
		}

		// A punch proves the peer reaches us; the ack closes the other way.
		if kind == kindPunch {
			s.send(kindAck, "")
		}
		s.flushPending()
		return nil
	}
}

// flushPending delivers whatever was typed before the path was open, so an
// eager first message is not silently lost.
func (s *Session) flushPending() {
	s.mu.Lock()
	pending := s.pending
	s.pending = nil
	s.open = true
	s.mu.Unlock()

	for _, message := range pending {
		s.send(kindMessage, message)
	}
}

// Run pumps incoming messages to the output until the context is cancelled.
func (s *Session) Run(ctx context.Context) error {
	if err := s.conn.SetReadDeadline(time.Time{}); err != nil {
		return fmt.Errorf("punch: cannot clear read deadline: %w", err)
	}

	s.heard()
	go s.pingLoop(ctx)

	buffer := make([]byte, readBufferSize)
	for {
		n, from, err := s.conn.ReadFromUDP(buffer)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("punch: read failed: %w", err)
		}
		if peer := s.Peer(); peer == nil || !sameEndpoint(peer, from) {
			continue
		}

		kind, payload, err := s.codec.Open(buffer[:n])
		if err != nil {
			continue
		}
		// Every frame that authenticates is proof of life, whatever it carries.
		s.heard()

		switch kind {
		case kindMessage:
			s.events().Message(payload)
		case kindTyping:
			s.events().Typing(payload != "")
		case kindPing:
			s.send(kindPong, payload)
		case kindPong:
			s.receivePong(payload)
		case kindPunch:
			// The peer is still handshaking because our ack was lost.
			s.send(kindAck, "")
		}
	}
}

// SendMessage queues the text while the path is still being negotiated and
// sends it straight away once it is open.
func (s *Session) SendMessage(text string) {
	s.mu.Lock()
	if !s.open {
		s.pending = append(s.pending, text)
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	s.send(kindMessage, text)
}

func (s *Session) punchLoop(ctx context.Context) {
	ticker := time.NewTicker(punchInterval)
	defer ticker.Stop()

	for {
		s.send(kindPunch, "")
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Session) send(kind byte, payload string) {
	peer := s.Peer()
	if peer == nil {
		return
	}
	_, _ = s.conn.WriteToUDP(s.codec.Seal(kind, payload), peer)
}

func sameEndpoint(a, b *net.UDPAddr) bool {
	return a.IP.Equal(b.IP) && a.Port == b.Port
}
