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

	lastHeard    time.Time
	rtt          time.Duration
	pingSeq      uint64
	pingSentAt   time.Time
	recoverDelay time.Duration
	moves        int
	sniff        func(payload []byte, from *net.UDPAddr) bool
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
	flag := byte('0')
	if active {
		flag = '1'
	}
	s.send(kindTyping, padded([]byte{flag}))
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

		// Whoever shares this socket gets its datagrams here too. Waiting is
		// exactly when an endpoint change matters most, because the invite
		// already handed out is the thing that stops working.
		if s.sniffed(buffer[:n], from) {
			continue
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
		if s.sniffed(buffer[:n], from) {
			continue
		}

		kind, payload, err := s.codec.Open(buffer[:n])
		if err != nil {
			continue
		}
		if !s.accept(from) {
			continue
		}
		// Every frame that authenticates is proof of life, whatever it carries.
		s.heard()

		switch kind {
		case kindMessage:
			s.events().Message(payload)
		case kindTyping:
			s.events().Typing(len(payload) > 0 && payload[0] == '1')
		case kindPing:
			// The pong echoes the sequence but pads independently, so a reply
			// is not recognisable by matching the size of what prompted it.
			if seq, ok := readSequence(payload); ok {
				s.send(kindPong, sequencePayload(seq))
			}
		case kindPong:
			s.receivePong(payload)
		case kindPunch:
			// The peer is still handshaking because our ack was lost.
			s.send(kindAck, pad())
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
		s.send(kindPunch, pad())
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

// Sniff hands datagrams that are not from the peer to another protocol sharing
// this socket. There can only be one reader on a UDP socket, so anything else
// that needs to see traffic has to be dispatched from here rather than reading
// alongside.
func (s *Session) Sniff(handler func(payload []byte, from *net.UDPAddr) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sniff = handler
}

func (s *Session) sniffed(payload []byte, from *net.UDPAddr) bool {
	s.mu.RLock()
	handler := s.sniff
	s.mu.RUnlock()

	return handler != nil && handler(payload, from)
}

// accept decides whether an authenticated frame from this address belongs to
// the session. A frame from somewhere new is the peer having moved: only the
// holder of the secret can produce one, so following it is as safe as trusting
// the secret in the first place. It is only followed once the path has gone
// quiet, which keeps a healthy conversation from flapping between addresses.
func (s *Session) accept(from *net.UDPAddr) bool {
	peer := s.Peer()
	if peer == nil {
		return false
	}
	if sameEndpoint(peer, from) {
		return true
	}
	if s.Health().Link == LinkAlive {
		return false
	}

	s.mu.Lock()
	s.peer = from
	s.moves++
	s.mu.Unlock()
	return true
}

// Moves counts how many times the peer has been followed to a new address.
// The caller polls it, the way it polls Health: nothing arrives to announce a
// migration that has already been accepted.
func (s *Session) Moves() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.moves
}
