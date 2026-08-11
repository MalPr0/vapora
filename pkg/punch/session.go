package punch

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/MalPr0/vapora/pkg/text"
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
//
// It never reads: a mux owns the socket and hands it datagrams through Deliver.
type Session struct {
	wire   Wire
	codec  Codec
	output io.Writer

	mu       sync.RWMutex
	peer     *net.UDPAddr
	open     bool
	opened   chan struct{}
	pending  []string
	observer Observer

	lastHeard    time.Time
	rtt          time.Duration
	pingSeq      uint64
	pingSentAt   time.Time
	recoverDelay time.Duration
	moves        int
	peerSender   Sender
	knownSender  bool
	departed     bool
	probes       probeCount
	extra        func(Opened) bool
}

func NewSession(wire Wire, codec Codec, output io.Writer) *Session {
	return &Session{
		wire:     wire,
		codec:    codec,
		output:   output,
		observer: writerObserver{output},
		opened:   make(chan struct{}),
	}
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

// Open punches towards the peer until one answers. A peer set later, by pasting
// an invite, is picked up by the running punch loop.
func (s *Session) Open(ctx context.Context, timeout time.Duration) error {
	openCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	go s.punchLoop(openCtx)

	select {
	case <-s.opened:
		return nil
	case <-openCtx.Done():
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return ErrPunchTimeout
	}
}

// Established waits for the path to open without starting another punch loop,
// which is what a caller wants when something else already started one.
func (s *Session) Established(ctx context.Context) error {
	select {
	case <-s.opened:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Run keeps the path warm until the context is cancelled. Datagrams arrive
// through Deliver, so there is nothing to read here.
func (s *Session) Run(ctx context.Context) error {
	s.heard()
	go s.pingLoop(ctx)

	<-ctx.Done()
	return nil
}

// Deliver handles one datagram and reports whether it belonged to this session.
func (s *Session) Deliver(payload []byte, from *net.UDPAddr) bool {
	frame, err := s.codec.Open(payload)
	if err != nil {
		// A frame that does not authenticate is someone else's packet: the
		// secret is what keeps a stranger from becoming the peer. Nothing is
		// sent back, so a scanner learns nothing from probing here, but the
		// attempt is counted: an address that only ever appeared on one invite
		// should not be hearing from anybody else.
		s.countProbe(from)
		return false
	}

	// A punch or its ack establishes the path. Anything else falls through:
	// the peer's first message can outrun the ack, and dropping it because the
	// handshake has not finished loses a line that authenticated.
	if !s.established() && s.handshake(frame, from) {
		return true
	}
	if !s.accept(from, frame.Sender) {
		return false
	}

	// Every frame that authenticates is proof of life, whatever it carries.
	s.heard()
	s.handle(frame)
	return true
}

// handshake is the part of Deliver that runs before a path exists: only a punch
// or its ack can establish one.
func (s *Session) handshake(frame Opened, from *net.UDPAddr) bool {
	if frame.Kind != kindPunch && frame.Kind != kindAck {
		return false
	}

	peer := s.Peer()
	if peer != nil && !sameEndpoint(peer, from) {
		return false
	}
	if peer == nil {
		// The invite went one way only and their packet still got in.
		// Announcing it is the caller's business: a session that writes to a
		// stream of its own lands in the middle of whatever a UI drew.
		s.SetPeer(from)
	}

	// The handshake is where the peer stops being an address and becomes a
	// codec instance, which is what a later move gets checked against.
	s.rememberSender(frame.Sender)
	s.heard()

	// A punch proves the peer reaches us; the ack closes the other way.
	if frame.Kind == kindPunch {
		s.send(kindAck, pad())
	}
	s.flushPending()
	return true
}

func (s *Session) handle(frame Opened) {
	switch frame.Kind {
	case kindMessage:
		// Only text crosses this channel. A frame carrying anything else is
		// not this program on the other end, so it is dropped rather than
		// cleaned up and shown.
		if text.Valid(frame.Payload) {
			s.events().Message(frame.Payload)
		}
	case kindBye:
		s.depart()
	case kindTyping:
		s.events().Typing(len(frame.Payload) > 0 && frame.Payload[0] == '1')
	case kindPing:
		// The pong echoes the sequence but pads independently, so a reply is
		// not recognisable by matching the size of what prompted it.
		if seq, ok := readSequence(frame.Payload); ok {
			s.send(kindPong, sequencePayload(seq))
		}
	case kindPong:
		s.receivePong(frame.Payload)
	case kindPunch:
		// The peer is still handshaking because our ack was lost.
		s.send(kindAck, pad())
	default:
		s.handleExtra(frame)
	}
}

// Extra handles frames whose kind this session does not know, which is how a
// room carries its own protocol over a pair channel without decrypting twice.
func (s *Session) Extra(handler func(Opened) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.extra = handler
}

func (s *Session) handleExtra(frame Opened) {
	s.mu.RLock()
	handler := s.extra
	s.mu.RUnlock()

	if handler != nil {
		handler(frame)
	}
}

func (s *Session) established() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.open
}

// flushPending delivers whatever was typed before the path was open, so an
// eager first message is not silently lost, and releases Open.
func (s *Session) flushPending() {
	s.mu.Lock()
	if s.open {
		s.mu.Unlock()
		return
	}
	pending := s.pending
	s.pending = nil
	s.open = true
	close(s.opened)
	s.mu.Unlock()

	for _, message := range pending {
		s.send(kindMessage, message)
	}
}

// SendMessage queues the line while the path is still being negotiated and
// sends it straight away once it is open. The line is sanitised first, so this
// channel carries text and nothing else, whatever the caller handed over.
func (s *Session) SendMessage(line string) {
	line = text.Safe(line)

	s.mu.Lock()
	if !s.open {
		s.pending = append(s.pending, line)
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	s.send(kindMessage, line)
}

// Goodbye tells the peer this side is leaving, so it can say so at once instead
// of waiting out the silence.
func (s *Session) Goodbye() {
	s.send(kindBye, pad())
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
	_ = s.wire.Send(s.codec.Seal(kind, payload), peer)
}

func sameEndpoint(a, b *net.UDPAddr) bool {
	return a.IP.Equal(b.IP) && a.Port == b.Port
}

// accept decides whether an authenticated frame from this address belongs to
// the session.
//
// Holding the secret is not enough to be the peer. Everyone handed the same
// invite seals under the same key, so a third party's frames authenticate just
// as well, and following one would hand the session to whoever showed up last
// while evicting the peer in silence. The sender is what tells them apart: it
// is drawn per codec, so it names the process rather than the invite.
//
// A move is therefore only followed when the sender matches the one already
// established, and only once the path has gone quiet, which keeps a healthy
// conversation from flapping between addresses. A peer that restarted has a new
// sender and is deliberately not followed: a restarted process is a new
// session, and treating it as a move is the same mistake seen from the inside.
func (s *Session) accept(from *net.UDPAddr, sender Sender) bool {
	peer := s.Peer()
	if peer == nil {
		return false
	}
	if sameEndpoint(peer, from) {
		s.rememberSender(sender)
		return true
	}

	// An authenticated frame from an address that is not the peer's is the
	// strongest evidence there is that the invite is in more hands than one:
	// junk from a scanner never gets this far. Whether or not it is followed,
	// it is worth counting.
	defer s.countImpostor(from)

	if s.Health().Link == LinkAlive {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.knownSender && s.peerSender != sender {
		return false
	}
	s.peer = from
	s.moves++
	s.peerSender = sender
	s.knownSender = true
	return true
}

// rememberSender pins the peer to the codec instance that first authenticated,
// so a later move can be checked against it.
func (s *Session) rememberSender(sender Sender) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.knownSender {
		s.peerSender = sender
		s.knownSender = true
	}
}

// Moves counts how many times the peer has been followed to a new address.
// The caller polls it, the way it polls Health: nothing arrives to announce a
// migration that has already been accepted.
func (s *Session) Moves() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.moves
}
