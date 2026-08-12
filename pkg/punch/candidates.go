package punch

import (
	"context"
	"net"
	"sync"
	"time"
)

// candidates is every address a member might answer on.
//
// One address cannot describe somebody behind the same router as you: their
// public address needs the router to send a packet out and route it straight
// back in, which most home routers will not do for UDP, and their local address
// means nothing from anywhere else. Neither is right for everyone, so both are
// tried and the one that answers wins.
type candidates struct {
	mu      sync.Mutex
	addrs   []*net.UDPAddr
	current int
}

func newCandidates(first *net.UDPAddr) *candidates {
	set := &candidates{}
	set.consider(first)
	return set
}

// consider adds an address if it is new, and reports whether it was.
func (c *candidates) consider(addr *net.UDPAddr) bool {
	if addr == nil {
		return false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, known := range c.addrs {
		if sameEndpoint(known, addr) {
			return false
		}
	}
	// More than a handful is somebody padding a roster rather than a machine
	// with several addresses.
	if len(c.addrs) >= maxCandidates {
		return false
	}
	c.addrs = append(c.addrs, addr)
	return true
}

// next moves to the following candidate and returns it, or nil when there is
// only one and rotating would achieve nothing.
func (c *candidates) next() *net.UDPAddr {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.addrs) < 2 {
		return nil
	}
	c.current = (c.current + 1) % len(c.addrs)
	return c.addrs[c.current]
}

func (c *candidates) at() *net.UDPAddr {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.addrs) == 0 {
		return nil
	}
	return c.addrs[c.current]
}

// localOf reports a candidate other than the given one, so a roster passes on
// the second address as well as the one in use. Which of the two is "local" is
// not this side's business — it just knows there was more than one.
func (c *candidates) localOf(inUse *net.UDPAddr) *net.UDPAddr {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, addr := range c.addrs {
		if !sameEndpoint(addr, inUse) {
			return addr
		}
	}
	return nil
}

func (c *candidates) all() []*net.UDPAddr {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]*net.UDPAddr(nil), c.addrs...)
}

const (
	maxCandidates = 4
	// rotateEvery is how long one candidate gets before the next is tried. Long
	// enough for a round trip across the world and back with room to spare,
	// short enough that a room with a local pair opens while people are still
	// looking at the screen.
	rotateEvery = 2 * time.Second
)

// sayHello announces this side at an address, under the key the whole room
// shares. It carries the local address too, so a member on the same network
// learns the one that does not need a router to turn a packet around.
func (r *Room) sayHello(to *net.UDPAddr) {
	hello := padded(appendCandidate(r.identity.Public().Bytes(), r.local))
	_ = r.mux.Send(r.roomCode.Seal(kindHello, hello), to)
}

// rotate walks a member's candidates until one of them answers.
//
// It stops at the first sign of life, and life means a frame that opened under
// the pair key — an address from a roster is a suggestion from somebody else,
// and only the peer's own cryptography settles which one is real.
func (r *Room) rotate(ctx context.Context, member *roomMember) {
	ticker := time.NewTicker(rotateEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		// Health is not the question here: a session that has never heard
		// anything still reports itself alive, because silence before the first
		// word is not evidence of trouble. What matters is whether a path
		// exists at all.
		if member.session.established() {
			return
		}

		addr := member.candidates.next()
		if addr == nil {
			continue
		}

		// The mux routes by address, so the new one needs a claim of its own or
		// the reply arrives unrouted. Losing that race is not a problem: adopt
		// catches what no route claimed.
		_ = r.mux.Route(addr, member.session)
		member.session.SetPeer(addr)

		// Say hello down the new one as well. A punch is sealed for a pair the
		// other side may not know exists yet — if they have never heard of us,
		// they have no session to open it with and it dies unread. A hello is
		// sealed with the room key, which everyone in the room can open, and it
		// is what makes them create the session that answers.
		r.sayHello(addr)
	}
}
