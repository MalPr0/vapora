package punch

import (
	"context"
	"testing"
	"time"
)

// The probe has to stay invisible: a chat that announced every heartbeat would
// be unreadable, and the point is that only its absence is worth showing.
func TestPingAndPongNeverReachTheObserver(t *testing.T) {
	left, right := listen(t), listen(t)
	defer left.Close()
	defer right.Close()

	observer := &recordingObserver{typing: make(chan bool, 8), messages: make(chan string, 8)}
	leftSession := NewSession(left, PlainCodec{}, &syncBuffer{})
	rightSession := NewSession(right, PlainCodec{}, &syncBuffer{})
	rightSession.Observe(observer)

	leftSession.SetPeer(localAddr(t, right))
	rightSession.SetPeer(localAddr(t, left))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	openBoth(t, ctx, leftSession, rightSession)

	go leftSession.Run(ctx)
	go rightSession.Run(ctx)

	// Both sides ping on entering Run, so an answered round trip has happened
	// well before this returns.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if leftSession.Health().RTT > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if rtt := leftSession.Health().RTT; rtt <= 0 {
		t.Fatal("no round trip was ever measured")
	}
	select {
	case payload := <-observer.messages:
		t.Fatalf("a probe surfaced as a message: %q", payload)
	default:
	}
}

func TestHealthReportsSilence(t *testing.T) {
	left, right := listen(t), listen(t)
	defer left.Close()
	defer right.Close()

	leftSession := NewSession(left, PlainCodec{}, &syncBuffer{})
	rightSession := NewSession(right, PlainCodec{}, &syncBuffer{})
	leftSession.SetPeer(localAddr(t, right))
	rightSession.SetPeer(localAddr(t, left))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	openBoth(t, ctx, leftSession, rightSession)

	go leftSession.Run(ctx)
	go rightSession.Run(ctx)

	time.Sleep(300 * time.Millisecond)
	if health := leftSession.Health(); health.Link != LinkAlive {
		t.Fatalf("a talking peer reported %s", health.Link)
	}

	// Closing one socket is what a peer walking away actually looks like from
	// here: no notice, just nothing further.
	right.Close()

	leftSession.mu.Lock()
	leftSession.lastHeard = time.Now().Add(-staleAfter - time.Second)
	leftSession.mu.Unlock()

	if health := leftSession.Health(); health.Link != LinkStale {
		t.Fatalf("after %s of silence the link reported %s", staleAfter, health.Link)
	}

	leftSession.mu.Lock()
	leftSession.lastHeard = time.Now().Add(-lostAfter - time.Second)
	leftSession.mu.Unlock()

	health := leftSession.Health()
	if health.Link != LinkLost {
		t.Fatalf("after %s of silence the link reported %s", lostAfter, health.Link)
	}
	if health.Silence < lostAfter {
		t.Fatalf("silence reported as %s", health.Silence)
	}
}

// A path with no peer yet is not a broken path.
func TestHealthBeforeAPeerExists(t *testing.T) {
	conn := listen(t)
	defer conn.Close()

	if health := NewSession(conn, PlainCodec{}, &syncBuffer{}).Health(); health.Link != LinkAlive {
		t.Fatalf("an unopened session reported %s", health.Link)
	}
}

// Any authenticated frame is proof of life, not just a pong.
func TestAnyFrameRefreshesLiveness(t *testing.T) {
	left, right := listen(t), listen(t)
	defer left.Close()
	defer right.Close()

	leftSession := NewSession(left, PlainCodec{}, &syncBuffer{})
	rightSession := NewSession(right, PlainCodec{}, &syncBuffer{})
	leftSession.SetPeer(localAddr(t, right))
	rightSession.SetPeer(localAddr(t, left))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	openBoth(t, ctx, leftSession, rightSession)

	go leftSession.Run(ctx)

	leftSession.mu.Lock()
	leftSession.lastHeard = time.Now().Add(-lostAfter - time.Second)
	leftSession.mu.Unlock()
	if leftSession.Health().Link != LinkLost {
		t.Fatal("the setup did not take")
	}

	rightSession.SendMessage("hola")

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if leftSession.Health().Link == LinkAlive {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("a message did not bring the link back")
}

// A pong for a ping that is no longer outstanding would report a round trip
// that already ended.
func TestStalePongIsIgnored(t *testing.T) {
	conn := listen(t)
	defer conn.Close()

	session := NewSession(conn, PlainCodec{}, &syncBuffer{})
	// Health reports nothing without a peer, so the session has to look open
	// for the measurement to be observable at all.
	session.SetPeer(localAddr(t, conn))
	session.mu.Lock()
	session.lastHeard = time.Now()
	session.pingSeq = 5
	session.pingSentAt = time.Now()
	session.mu.Unlock()

	session.receivePong("3")
	if rtt := session.Health().RTT; rtt != 0 {
		t.Fatalf("a stale pong measured %s", rtt)
	}

	session.receivePong("not a number")
	if rtt := session.Health().RTT; rtt != 0 {
		t.Fatalf("a malformed pong measured %s", rtt)
	}

	session.receivePong("5")
	if rtt := session.Health().RTT; rtt <= 0 {
		t.Fatal("the outstanding ping measured nothing")
	}
}
