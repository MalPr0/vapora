package punch

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

// ErrAddressTaken means another sink already owns that address. Overwriting the
// route would silently steal a peer's traffic, so claiming is refused instead.
var ErrAddressTaken = errors.New("punch: address already routed")

// Sink receives datagrams the mux routed to it. Returning false leaves the
// datagram to the next sink in the chain.
//
// The payload is a window into the read buffer and is reused as soon as Deliver
// returns: a sink that needs to keep it has to copy it. Deliver must not block
// either, because every sink shares one reader.
type Sink interface {
	Deliver(payload []byte, from *net.UDPAddr) bool
}

// SinkFunc adapts a plain function, which is what lets stun.Watcher.Handle be
// registered without knowing anything about this package.
type SinkFunc func(payload []byte, from *net.UDPAddr) bool

func (f SinkFunc) Deliver(payload []byte, from *net.UDPAddr) bool { return f(payload, from) }

// Wire is the socket as a sender needs it. Nothing that sends also reads: one
// goroutine owns the reader and hands out what it finds.
type Wire interface {
	Send(payload []byte, to *net.UDPAddr) error
}

// Mux is the one reader of a UDP socket. Two readers on the same socket is a
// lottery over which one receives each datagram, so everything that needs to
// see traffic is dispatched from here instead of reading alongside.
type Mux struct {
	conn *net.UDPConn

	mu       sync.RWMutex
	routes   map[string]Sink
	fallback []Sink
}

func NewMux(conn *net.UDPConn) *Mux {
	return &Mux{conn: conn, routes: map[string]Sink{}}
}

// Route claims an address for a sink. Traffic from anywhere else still reaches
// the fallback chain.
func (m *Mux) Route(addr *net.UDPAddr, sink Sink) error {
	if addr == nil || sink == nil {
		return errors.New("punch: route needs an address and a sink")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	key := addr.String()
	if _, taken := m.routes[key]; taken {
		return fmt.Errorf("%w: %s", ErrAddressTaken, key)
	}
	m.routes[key] = sink
	return nil
}

func (m *Mux) Unroute(addr *net.UDPAddr) {
	if addr == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.routes, addr.String())
}

// Fallback appends a sink for datagrams no route claimed. Order matters: the
// most selective claimant goes first, because the first to return true ends the
// chain.
func (m *Mux) Fallback(sink Sink) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fallback = append(m.fallback, sink)
}

func (m *Mux) Send(payload []byte, to *net.UDPAddr) error {
	if to == nil {
		return errors.New("punch: nowhere to send")
	}
	if _, err := m.conn.WriteToUDP(payload, to); err != nil {
		return fmt.Errorf("punch: cannot send to %s: %w", to, err)
	}
	return nil
}

func (m *Mux) Local() *net.UDPAddr {
	address, _ := m.conn.LocalAddr().(*net.UDPAddr)
	return address
}

// Run reads until the context ends. It is the only place this socket is read.
func (m *Mux) Run(ctx context.Context) error {
	if err := m.conn.SetReadDeadline(time.Time{}); err != nil {
		return fmt.Errorf("punch: cannot clear read deadline: %w", err)
	}

	// Cancelling a context does not interrupt a blocked read, and steady
	// traffic keeps this loop busy enough that it would never look. A deadline
	// in the past is what turns the cancellation into a wakeup.
	go func() {
		<-ctx.Done()
		_ = m.conn.SetReadDeadline(time.Now())
	}()

	buffer := make([]byte, readBufferSize)
	for {
		if ctx.Err() != nil {
			return nil
		}

		n, from, err := m.conn.ReadFromUDP(buffer)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("punch: read failed: %w", err)
		}
		m.dispatch(buffer[:n], from)
	}
}

func (m *Mux) dispatch(payload []byte, from *net.UDPAddr) {
	// The lock is released before anything is delivered: a sink is expected to
	// call Route or Unroute from inside Deliver, and holding it here would
	// deadlock on the first one that does.
	m.mu.RLock()
	sink := m.routes[from.String()]
	fallback := m.fallback
	m.mu.RUnlock()

	if sink != nil {
		sink.Deliver(payload, from)
		return
	}
	for _, candidate := range fallback {
		if candidate.Deliver(payload, from) {
			return
		}
	}
}
