package punch

import (
	"net"
	"time"
)

// probeSources bounds how many distinct addresses are remembered. The count is
// what matters; the set is only there to say whether it is one host retrying or
// many hosts sweeping.
const probeSources = 32

// Probes is traffic that reached this socket and could not authenticate. It is
// never answered, so nothing about this host is confirmed to whoever sent it,
// but it is worth counting: an address that only ever appeared on one invite
// should not be receiving anything from anybody else.
type Probes struct {
	// Count is how many datagrams were rejected.
	Count int
	// Sources is how many distinct addresses they came from.
	Sources int
	// Last is the most recent one.
	Last *net.UDPAddr
	// Since is when the first arrived.
	Since time.Time
}

type probeCount struct {
	count   int
	sources map[string]bool
	last    *net.UDPAddr
	since   time.Time
}

func (s *Session) countProbe(from *net.UDPAddr) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.probes.sources == nil {
		s.probes.sources = map[string]bool{}
		s.probes.since = time.Now()
	}
	s.probes.count++
	s.probes.last = from
	if len(s.probes.sources) < probeSources {
		s.probes.sources[from.String()] = true
	}
}

// Probes reports what has been rejected so far. Like everything else about a
// path, it is polled: a datagram nobody answered generates no event.
func (s *Session) Probes() Probes {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return Probes{
		Count:   s.probes.count,
		Sources: len(s.probes.sources),
		Last:    s.probes.last,
		Since:   s.probes.since,
	}
}

// depart records that the peer said it was leaving, which is the difference
// between a path that broke and one that was closed.
func (s *Session) depart() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.departed = true
}
