package punch

import (
	"context"
	"encoding/binary"
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
	// recoverInterval is the cadence once the path is degraded. Recovery is
	// worth the extra packets; an idle path is not.
	recoverInterval = time.Second
	// maxRecoverInterval caps the backoff. A peer that never returns would
	// otherwise be punched at once a second for as long as the window stays
	// open, which is a lot of packets thrown at nobody.
	maxRecoverInterval = 30 * time.Second
)

// Health is a snapshot of the path.
type Health struct {
	Link Link
	// Departed means the peer said it was leaving, rather than the path having
	// simply gone quiet. The two look identical on the wire otherwise.
	Departed bool
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
	health := Health{RTT: s.rtt, Silence: silence, Departed: s.departed}
	if s.departed {
		health.Link = LinkLost
		return health
	}
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
	for {
		s.sendPing()

		// A quiet path is worth working at harder. Both NATs may simply have
		// dropped their state while nothing was flowing, and a fresh punch
		// rebuilds it without anyone restarting anything; probing at the idle
		// cadence would make every recovery wait out a full interval.
		delay := pingInterval
		if s.Health().Link != LinkAlive {
			s.send(kindPunch, pad())
			delay = s.backoff()
		} else {
			s.resetBackoff()
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}
}

func (s *Session) sendPing() {
	s.mu.Lock()
	s.pingSeq++
	seq := s.pingSeq
	s.pingSentAt = time.Now()
	s.mu.Unlock()

	s.send(kindPing, sequencePayload(seq))
}

// sequencePayload puts the sequence in a fixed prefix and lets the padding vary
// after it, so the number is readable without the frame having a tell-tale
// length.
func sequencePayload(seq uint64) string {
	prefix := make([]byte, 8)
	binary.BigEndian.PutUint64(prefix, seq)
	return padded(prefix)
}

func readSequence(payload string) (uint64, bool) {
	if len(payload) < 8 {
		return 0, false
	}
	return binary.BigEndian.Uint64([]byte(payload[:8])), true
}

func (s *Session) receivePong(payload string) {
	seq, ok := readSequence(payload)
	if !ok {
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

// backoff widens the recovery cadence the longer a path stays down. The first
// seconds after a path goes quiet are when it is most likely to be a pinhole
// that expired, so those are probed hard; an hour later it is almost certainly
// gone and hammering it helps nobody.
func (s *Session) backoff() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.recoverDelay <= 0 {
		s.recoverDelay = recoverInterval
		return s.recoverDelay
	}
	if s.recoverDelay < maxRecoverInterval {
		s.recoverDelay *= 2
		if s.recoverDelay > maxRecoverInterval {
			s.recoverDelay = maxRecoverInterval
		}
	}
	return s.recoverDelay
}

func (s *Session) resetBackoff() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recoverDelay = 0
}
