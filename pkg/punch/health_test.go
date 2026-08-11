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
	leftSession := wired(t, left, plainCodec{}, &syncBuffer{})
	rightSession := wired(t, right, plainCodec{}, &syncBuffer{})
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

	leftSession := wired(t, left, plainCodec{}, &syncBuffer{})
	rightSession := wired(t, right, plainCodec{}, &syncBuffer{})
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

	if health := wired(t, conn, plainCodec{}, &syncBuffer{}).Health(); health.Link != LinkAlive {
		t.Fatalf("an unopened session reported %s", health.Link)
	}
}

// Any authenticated frame is proof of life, not just a pong.
func TestAnyFrameRefreshesLiveness(t *testing.T) {
	left, right := listen(t), listen(t)
	defer left.Close()
	defer right.Close()

	leftSession := wired(t, left, plainCodec{}, &syncBuffer{})
	rightSession := wired(t, right, plainCodec{}, &syncBuffer{})
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

	session := wired(t, conn, plainCodec{}, &syncBuffer{})
	// Health reports nothing without a peer, so the session has to look open
	// for the measurement to be observable at all.
	session.SetPeer(localAddr(t, conn))
	session.mu.Lock()
	session.lastHeard = time.Now()
	session.pingSeq = 5
	session.pingSentAt = time.Now()
	session.mu.Unlock()

	session.receivePong(sequencePayload(3))
	if rtt := session.Health().RTT; rtt != 0 {
		t.Fatalf("a stale pong measured %s", rtt)
	}

	session.receivePong("short")
	if rtt := session.Health().RTT; rtt != 0 {
		t.Fatalf("a truncated pong measured %s", rtt)
	}

	session.receivePong(sequencePayload(5))
	if rtt := session.Health().RTT; rtt <= 0 {
		t.Fatal("the outstanding ping measured nothing")
	}
}

// AEAD hides what a frame says but not how long it is, so a control frame with
// nothing to carry would arrive at a size nobody else produces, on a cadence
// that names it a heartbeat.
func TestControlFramesVaryInLength(t *testing.T) {
	sizes := map[int]bool{}
	for i := 0; i < 200; i++ {
		sizes[len(pad())] = true
	}
	if len(sizes) < 50 {
		t.Fatalf("200 control frames only took %d distinct lengths", len(sizes))
	}

	// The sequence still has to survive being padded.
	for _, seq := range []uint64{0, 1, 7, 1 << 40} {
		got, ok := readSequence(sequencePayload(seq))
		if !ok || got != seq {
			t.Fatalf("sequence %d came back as %d (ok=%v)", seq, got, ok)
		}
	}
}

// Two frames carrying the same thing must not look the same on the wire.
func TestPaddingIsNotReused(t *testing.T) {
	first := sequencePayload(1)
	for i := 0; i < 20; i++ {
		if sequencePayload(1) != first {
			return
		}
	}
	t.Fatal("the same sequence produced an identical frame every time")
}

// Padding only helps if every control frame actually gets it. This walks a real
// handshake and measures what goes on the wire, because a frame that forgot to
// pad still leaves a fixed size on a fixed cadence for anyone counting bytes.
func TestEveryControlFrameOnTheWireVaries(t *testing.T) {
	sizes := map[byte]map[int]bool{}

	for attempt := 0; attempt < 12; attempt++ {
		home, peer := listen(t), listen(t)

		secret, err := NewSecret()
		if err != nil {
			t.Fatalf("cannot generate a secret: %v", err)
		}
		homeCodec, err := NewSecretCodec(secret, RoleInviter)
		if err != nil {
			t.Fatalf("cannot build the codec: %v", err)
		}
		peerCodec, err := NewSecretCodec(secret, RoleJoiner)
		if err != nil {
			t.Fatalf("cannot build the codec: %v", err)
		}

		session := wired(t, home, homeCodec, &syncBuffer{})
		session.SetPeer(localAddr(t, peer))

		ctx, cancel := context.WithCancel(context.Background())
		go session.Open(ctx, 2*time.Second)

		// Prompt an ack out of the handshake, then a pong out of the session.
		_, _ = peer.WriteToUDP(peerCodec.Seal(kindPunch, pad()), localAddr(t, home))
		time.Sleep(60 * time.Millisecond)
		go session.Run(ctx)
		_, _ = peer.WriteToUDP(peerCodec.Seal(kindPing, sequencePayload(1)), localAddr(t, home))

		_ = peer.SetReadDeadline(time.Now().Add(time.Second))
		buffer := make([]byte, 2048)
		for reads := 0; reads < 6; reads++ {
			n, _, err := peer.ReadFromUDP(buffer)
			if err != nil {
				break
			}
			frame, err := peerCodec.Open(buffer[:n])
			if err != nil {
				continue
			}
			if sizes[frame.Kind] == nil {
				sizes[frame.Kind] = map[int]bool{}
			}
			sizes[frame.Kind][n] = true
		}

		cancel()
		home.Close()
		peer.Close()
	}

	for _, kind := range []byte{kindPunch, kindAck, kindPing, kindPong} {
		seen := sizes[kind]
		if len(seen) == 0 {
			t.Fatalf("kind 0x%02x never reached the wire, the test did not exercise it", kind)
		}
		if len(seen) == 1 {
			t.Fatalf("kind 0x%02x always had the same size on the wire: it is not padded", kind)
		}
	}
}
