package punch

import (
	"context"
	"net"
	"testing"
	"time"
)

// staleOut backdates the session after Run has had its turn: Run refreshes
// liveness as it starts, and would overwrite this if it ran second.
func staleOut(t *testing.T, session *Session) {
	t.Helper()
	time.Sleep(100 * time.Millisecond)
	session.mu.Lock()
	session.lastHeard = time.Now().Add(-lostAfter - time.Second)
	session.mu.Unlock()
}

// A peer that roams gets a new address. Only the holder of the secret can
// produce a frame that authenticates, so following one is as safe as trusting
// the secret was.
func TestPeerIsFollowedToANewAddress(t *testing.T) {
	home, oldAddr, newAddr := listen(t), listen(t), listen(t)
	defer home.Close()
	defer oldAddr.Close()
	defer newAddr.Close()

	observer := &recordingObserver{typing: make(chan bool, 4), messages: make(chan string, 4)}
	session := NewSession(home, PlainCodec{}, &syncBuffer{})
	session.Observe(observer)
	session.SetPeer(localAddr(t, oldAddr))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go session.Run(ctx)

	staleOut(t, session)

	// The peer reappears from somewhere else entirely.
	_, _ = newAddr.WriteToUDP(encode(kindMessage, "me mude"), localAddr(t, home))

	select {
	case got := <-observer.messages:
		if got != "me mude" {
			t.Fatalf("got %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a frame from the peer's new address was dropped")
	}

	if peer := session.Peer(); peer.Port != localAddr(t, newAddr).Port {
		t.Fatalf("the session is still talking to %v", peer)
	}
	if session.Moves() != 1 {
		t.Fatalf("the migration was not counted, got %d", session.Moves())
	}
}

// A healthy conversation must not bounce between addresses, or a duplicated
// path would have the two ends chasing each other.
func TestAHealthyPathDoesNotFollow(t *testing.T) {
	home, peer, stranger := listen(t), listen(t), listen(t)
	defer home.Close()
	defer peer.Close()
	defer stranger.Close()

	observer := &recordingObserver{typing: make(chan bool, 4), messages: make(chan string, 4)}
	session := NewSession(home, PlainCodec{}, &syncBuffer{})
	session.Observe(observer)
	session.SetPeer(localAddr(t, peer))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go session.Run(ctx)

	_, _ = stranger.WriteToUDP(encode(kindMessage, "desde otro lado"), localAddr(t, home))

	select {
	case got := <-observer.messages:
		t.Fatalf("a live path followed a second address: %q", got)
	case <-time.After(500 * time.Millisecond):
	}
	if session.Moves() != 0 {
		t.Fatalf("a migration was counted on a healthy path")
	}
}

// Following requires the secret, not just an address.
func TestAnUnauthenticatedFrameNeverMoves(t *testing.T) {
	home, peer, attacker := listen(t), listen(t), listen(t)
	defer home.Close()
	defer peer.Close()
	defer attacker.Close()

	secret, err := NewSecret()
	if err != nil {
		t.Fatalf("cannot generate a secret: %v", err)
	}
	codec, err := NewSecretCodec(secret, RoleInviter)
	if err != nil {
		t.Fatalf("cannot build the codec: %v", err)
	}

	session := NewSession(home, codec, &syncBuffer{})
	session.SetPeer(localAddr(t, peer))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go session.Run(ctx)

	staleOut(t, session)

	for i := 0; i < 20; i++ {
		_, _ = attacker.WriteToUDP(encode(kindMessage, "dejame entrar"), localAddr(t, home))
		time.Sleep(10 * time.Millisecond)
	}

	if session.Moves() != 0 {
		t.Fatal("an unauthenticated frame moved the session")
	}
	if peer := session.Peer(); peer.Port == localAddr(t, attacker).Port {
		t.Fatal("the session is talking to the attacker")
	}
}

// Sniff is how another protocol shares the one reader a UDP socket allows.
func TestSniffClaimsForeignDatagrams(t *testing.T) {
	home, other := listen(t), listen(t)
	defer home.Close()
	defer other.Close()

	claimed := make(chan []byte, 4)
	session := NewSession(home, PlainCodec{}, &syncBuffer{})
	session.SetPeer(localAddr(t, other))
	session.Sniff(func(payload []byte, _ *net.UDPAddr) bool {
		if len(payload) > 0 && payload[0] == 0xEE {
			copied := make([]byte, len(payload))
			copy(copied, payload)
			claimed <- copied
			return true
		}
		return false
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go session.Run(ctx)

	_, _ = other.WriteToUDP([]byte{0xEE, 'h', 'i'}, localAddr(t, home))

	select {
	case payload := <-claimed:
		if string(payload[1:]) != "hi" {
			t.Fatalf("got %q", payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the sniffer never saw the datagram")
	}
}

// A quiet path gets punched at again, because both NATs may simply have dropped
// their state while nothing was flowing.
func TestAQuietPathIsPunchedAgain(t *testing.T) {
	home, peer := listen(t), listen(t)
	defer home.Close()
	defer peer.Close()

	session := NewSession(home, PlainCodec{}, &syncBuffer{})
	session.SetPeer(localAddr(t, peer))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go session.Run(ctx)

	staleOut(t, session)

	_ = peer.SetReadDeadline(time.Now().Add(5 * time.Second))
	buffer := make([]byte, 128)
	for {
		n, _, err := peer.ReadFromUDP(buffer)
		if err != nil {
			t.Fatal("a quiet path was never punched at again")
		}
		if kind, _, err := decode(buffer[:n]); err == nil && kind == kindPunch {
			return
		}
	}
}
