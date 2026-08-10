package punch

import (
	"context"
	"strconv"
	"time"
)

// Link is what can be known about a UDP path, which is only ever inferred from
// silence: there is no connection to close and no close to be told about.
type Link int

const (
	// LinkAlive means a frame arrived recently enough.
	LinkAlive Link = iota
	// LinkStale means several pings went unanswered. The path may still heal
	// on its own, so this is a warning rather than a verdict.
	LinkStale
	// LinkLost means the silence outlasted any plausible hiccup.
	LinkLost
)

func (l Link) String() string {
	switch l {
	case LinkStale:
		return "stale"
	case LinkLost:
		return "lost"
	default:
		return "alive"
	}
}

const (
	// pingInterval doubles as the NAT keepalive, so it sits below the shortest
	// binding timeouts seen in the wild rather than being tuned for detection
	// speed alone.
	pingInterval = 5 * time.Second
	// staleAfter is a little over two missed pings, which is enough to not
	// call a single lost datagram an outage.
	staleAfter = 12 * time.Second
	// lostAfter is long enough that a laptop lid or a roaming handover has had
	// its chance to come back.
	lostAfter = 45 * time.Second
)

// Health is a snapshot of the path.
type Health struct {
	Link Link
	// RTT is the round trip of the last answered ping, zero until one is.
	RTT time.Duration
	// Silence is how long since anything at all arrived from the peer.
	Silence time.Duration
}

// Health reports the state of the path. It is a poll rather than an event
// because the interesting value is a duration that grows on its own: nothing
// arrives to announce that nothing is arriving.
func (s *Session) Health() Health {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.peer == nil || s.lastHeard.IsZero() {
		return Health{Link: LinkAlive}
	}

	silence := time.Since(s.lastHeard)
	health := Health{RTT: s.rtt, Silence: silence}
	switch {
	case silence >= lostAfter:
		health.Link = LinkLost
	case silence >= staleAfter:
		health.Link = LinkStale
	default:
		health.Link = LinkAlive
	}
	return health
}

func (s *Session) heard() {
	s.mu.Lock()
	s.lastHeard = time.Now()
	s.mu.Unlock()
}

// pingLoop sends the probe that makes silence measurable. The peer answers with
// a pong that never reaches the UI, so a live path stays invisible and only its
// absence is worth showing.
func (s *Session) pingLoop(ctx context.Context) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		s.sendPing()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Session) sendPing() {
	s.mu.Lock()
	s.pingSeq++
	seq := s.pingSeq
	s.pingSentAt = time.Now()
	s.mu.Unlock()

	s.send(kindPing, strconv.FormatUint(seq, 10))
}

func (s *Session) receivePong(payload string) {
	seq, err := strconv.ParseUint(payload, 10, 64)
	if err != nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	// Only the ping still outstanding measures anything; an older pong arriving
	// late would report a round trip that already ended.
	if seq == s.pingSeq {
		s.rtt = time.Since(s.pingSentAt)
	}
}
