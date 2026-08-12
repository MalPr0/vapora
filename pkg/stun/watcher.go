package stun

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

const (
	// pendingTTL bounds how long an unanswered request is remembered. Dropping
	// the whole table instead would throw away requests still legitimately in
	// flight.
	pendingTTL = 30 * time.Second
	// startupInterval is the cadence before the first answer. Probing at the
	// keepalive rate instead would make that answer hostage to one server and
	// one datagram, and UDP loses datagrams; nothing can be shared until it
	// arrives, so this rotates through every server several times a second.
	startupInterval = 400 * time.Millisecond
)

// Watcher tracks the public endpoint of a socket it does not own. It writes on
// its own, because writes to a UDP socket are safe from any goroutine, and is
// handed the answers by whoever owns the single reader.
//
// The endpoint moving is what silently breaks an invite already shared: the
// address on it stops existing and the peer keeps talking to nobody.
type Watcher struct {
	servers []string
	every   time.Duration

	mu       sync.Mutex
	pending  map[[12]byte]time.Time
	current  *net.UDPAddr
	onChange func(previous, current *net.UDPAddr)
	observed chan struct{}
}

// NewWatcher builds one. It keeps asking rather than asking once, because an
// address is not a fact but a lease: switch network, or sit idle long enough,
// and the invite you shared points at a door that no longer exists.
func NewWatcher(servers []string, every time.Duration) *Watcher {
	if len(servers) == 0 {
		servers = DefaultServers
	}
	if every <= 0 {
		every = DefaultKeepalive
	}
	return &Watcher{
		servers:  servers,
		every:    every,
		pending:  map[[12]byte]time.Time{},
		observed: make(chan struct{}),
	}
}

// OnChange fires when the observed endpoint differs from the last one seen. The
// first observation is not a change.
func (w *Watcher) OnChange(handler func(previous, current *net.UDPAddr)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onChange = handler
}

// Endpoint is the last address observed, or nil before the first answer. Use
// Wait when there is nothing useful to do without one.
func (w *Watcher) Endpoint() *net.UDPAddr {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.current
}

// Run sends binding requests until the context ends. It never reads: the socket
// has one owner and a second reader would steal its datagrams.
func (w *Watcher) Run(ctx context.Context, conn *net.UDPConn) error {
	if conn == nil {
		return errors.New("stun: watcher needs a socket")
	}

	for attempt := 0; ; attempt++ {
		w.probe(conn, w.servers[attempt%len(w.servers)])

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(w.interval()):
		}
	}
}

// interval probes hard until there is something to report, then settles into
// the cadence that only has to keep a NAT binding warm. Startup is never the
// slower of the two: the point is to get an answer sooner, not to override a
// caller that wants a tighter cadence than this.
func (w *Watcher) interval() time.Duration {
	if w.Endpoint() != nil || w.every < startupInterval {
		return w.every
	}
	return startupInterval
}

func (w *Watcher) probe(conn *net.UDPConn, server string) {
	target, err := net.ResolveUDPAddr("udp4", server)
	if err != nil {
		return
	}

	transactionID, request, err := buildBindingRequest(queryOptions{})
	if err != nil {
		return
	}

	now := time.Now()
	w.mu.Lock()
	for id, sent := range w.pending {
		if now.Sub(sent) > pendingTTL {
			delete(w.pending, id)
		}
	}
	w.pending[transactionID] = now
	w.mu.Unlock()

	_, _ = conn.WriteToUDP(request, target)
}

// Handle claims a datagram if it answers one of this watcher's requests, which
// is how it shares a socket with another protocol.
func (w *Watcher) Handle(payload []byte, _ *net.UDPAddr) bool {
	transactionID, ok := responseTransaction(payload)
	if !ok {
		return false
	}

	w.mu.Lock()
	_, mine := w.pending[transactionID]
	if !mine {
		w.mu.Unlock()
		return false
	}
	delete(w.pending, transactionID)
	w.mu.Unlock()

	response, err := parseBindingResponse(payload, transactionID)
	if err != nil {
		return true // it was ours, it was just not usable
	}
	w.observe(response.Mapped)
	return true
}

func (w *Watcher) observe(mapped *net.UDPAddr) {
	w.mu.Lock()
	previous := w.current
	w.current = mapped
	handler := w.onChange
	first := previous == nil
	w.mu.Unlock()

	if first {
		close(w.observed)
		return
	}
	if handler == nil || sameEndpoint(previous, mapped) {
		return
	}
	handler(previous, mapped)
}

// Wait blocks until the first answer arrives. A caller that shares its socket
// cannot query STUN directly, because the reader belongs to somebody else, so
// this is how it learns the address to put on an invite.
func (w *Watcher) Wait(ctx context.Context, timeout time.Duration) (*net.UDPAddr, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-w.observed:
		return w.Endpoint(), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		return nil, fmt.Errorf("stun: no server answered within %s", timeout)
	}
}

// responseTransaction reads the transaction id off a binding response without
// committing to parsing the rest, so a datagram that is not ours is rejected
// cheaply.
func responseTransaction(payload []byte) ([12]byte, bool) {
	var transactionID [12]byte
	if len(payload) < headerSize {
		return transactionID, false
	}
	if binary.BigEndian.Uint16(payload[0:2]) != bindingResponse {
		return transactionID, false
	}
	if binary.BigEndian.Uint32(payload[4:8]) != magicCookie {
		return transactionID, false
	}
	copy(transactionID[:], payload[8:20])
	return transactionID, true
}

func sameEndpoint(a, b *net.UDPAddr) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.IP.Equal(b.IP) && a.Port == b.Port
}
