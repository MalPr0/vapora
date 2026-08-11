package punch

import (
	"context"
	"net"
	"testing"
	"time"
)

// Quitting on purpose has to be distinguishable from a network that went quiet,
// or the other side waits out the whole silence budget to learn something that
// was already decided.
func TestGoodbyeIsImmediate(t *testing.T) {
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
	go rightSession.Run(ctx)

	if health := rightSession.Health(); health.Link != LinkAlive || health.Departed {
		t.Fatalf("a live path reported %+v", health)
	}

	leftSession.Goodbye()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if health := rightSession.Health(); health.Departed {
			if health.Link != LinkLost {
				t.Fatalf("a departed peer reported %s", health.Link)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the goodbye never arrived")
}

// Traffic that cannot authenticate is never answered, so a scanner learns
// nothing, but it is counted: an address that only ever appeared on one invite
// should not be hearing from anybody else.
func TestUnauthenticatedTrafficIsCountedAndNeverAnswered(t *testing.T) {
	home, stranger := listen(t), listen(t)
	defer home.Close()
	defer stranger.Close()

	secret, err := NewSecret()
	if err != nil {
		t.Fatalf("cannot generate a secret: %v", err)
	}
	codec, err := NewSecretCodec(secret, RoleInviter)
	if err != nil {
		t.Fatalf("cannot build the codec: %v", err)
	}

	session := wired(t, home, codec, &syncBuffer{})
	// The peer is somewhere else entirely: the stranger must be answered by
	// nothing at all, and pings to a real peer would muddy that.
	session.SetPeer(&net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: freePort(t)})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go session.Run(ctx)

	for i := 0; i < 5; i++ {
		_, _ = stranger.WriteToUDP([]byte("quien anda ahi"), localAddr(t, home))
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if session.Probes().Count >= 5 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	probes := session.Probes()
	if probes.Count < 5 {
		t.Fatalf("only %d probes were counted", probes.Count)
	}
	if probes.Sources != 1 || probes.Last == nil {
		t.Fatalf("got %+v", probes)
	}

	// Silence is the whole point: an answer would confirm something is here.
	_ = stranger.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	buffer := make([]byte, 512)
	if _, _, err := stranger.ReadFromUDP(buffer); err == nil {
		t.Fatal("the session answered a stranger")
	}
}

// Cancelling has to end the loop even while datagrams keep arriving, or a
// session only quits once its peer stops talking.
func TestRunStopsOnCancellationUnderTraffic(t *testing.T) {
	home, peer := listen(t), listen(t)
	defer home.Close()
	defer peer.Close()

	session := wired(t, home, plainCodec{}, &syncBuffer{})
	session.SetPeer(localAddr(t, peer))

	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan error, 1)
	go func() { stopped <- session.Run(ctx) }()

	flooding := make(chan struct{})
	defer close(flooding)
	go func() {
		for {
			select {
			case <-flooding:
				return
			default:
				_, _ = peer.WriteToUDP(encode(kindPing, sequencePayload(1)), localAddr(t, home))
				time.Sleep(5 * time.Millisecond)
			}
		}
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-stopped:
		if err != nil {
			t.Fatalf("got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run kept going after its context was cancelled")
	}
}
