package stun

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"sync"
	"time"
)

// pendingLimit bounds the outstanding transactions. Answers that never come
// would otherwise accumulate for as long as the session lives.
const pendingLimit = 16

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
}

func NewWatcher(servers []string, every time.Duration) *Watcher {
	if len(servers) == 0 {
		servers = DefaultServers
	}
	if every <= 0 {
		every = DefaultKeepalive
	}
	return &Watcher{servers: servers, every: every, pending: map[[12]byte]time.Time{}}
}

// OnChange fires when the observed endpoint differs from the last one seen. The
// first observation is not a change.
func (w *Watcher) OnChange(handler func(previous, current *net.UDPAddr)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onChange = handler
}

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

	ticker := time.NewTicker(w.every)
	defer ticker.Stop()

	for attempt := 0; ; attempt++ {
		w.probe(conn, w.servers[attempt%len(w.servers)])

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
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

	w.mu.Lock()
	if len(w.pending) >= pendingLimit {
		w.pending = map[[12]byte]time.Time{}
	}
	w.pending[transactionID] = time.Now()
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
	w.mu.Unlock()

	if previous == nil || handler == nil || sameEndpoint(previous, mapped) {
		return
	}
	handler(previous, mapped)
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
