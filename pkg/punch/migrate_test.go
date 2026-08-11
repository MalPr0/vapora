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

	observer := newRecorder(4)
	session := wired(t, home, plainCodec{}, &syncBuffer{})
	session.Observe(observer)
	session.SetPeer(localAddr(t, oldAddr))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go session.Run(ctx)

	staleOut(t, session)

	// The peer reappears from somewhere else entirely.
	_, _ = newAddr.WriteToUDP(encode(kindData, "me mude"), localAddr(t, home))

	select {
	case payload := <-observer.payloads:
		got := string(payload)
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

	observer := newRecorder(4)
	session := wired(t, home, plainCodec{}, &syncBuffer{})
	session.Observe(observer)
	session.SetPeer(localAddr(t, peer))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go session.Run(ctx)

	_, _ = stranger.WriteToUDP(encode(kindData, "desde otro lado"), localAddr(t, home))

	select {
	case payload := <-observer.payloads:
		got := string(payload)
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

	session := wired(t, home, codec, &syncBuffer{})
	session.SetPeer(localAddr(t, peer))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go session.Run(ctx)

	staleOut(t, session)

	for i := 0; i < 20; i++ {
		_, _ = attacker.WriteToUDP(encode(kindData, "dejame entrar"), localAddr(t, home))
		time.Sleep(10 * time.Millisecond)
	}

	if session.Moves() != 0 {
		t.Fatal("an unauthenticated frame moved the session")
	}
	if peer := session.Peer(); peer.Port == localAddr(t, attacker).Port {
		t.Fatal("the session is talking to the attacker")
	}
}

// A sink registered ahead of the session claims its own datagrams, which is how
// the STUN watcher shares the one reader a UDP socket allows.
func TestAnEarlierSinkClaimsItsOwnDatagrams(t *testing.T) {
	home, other := listen(t), listen(t)
	defer home.Close()
	defer other.Close()

	claimed := make(chan []byte, 4)
	watcher := SinkFunc(func(payload []byte, _ *net.UDPAddr) bool {
		if len(payload) > 0 && payload[0] == 0xEE {
			copied := make([]byte, len(payload))
			copy(copied, payload)
			claimed <- copied
			return true
		}
		return false
	})

	session := wired(t, home, plainCodec{}, &syncBuffer{}, watcher)
	session.SetPeer(localAddr(t, other))

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
		t.Fatal("the earlier sink never saw the datagram")
	}
}

// A quiet path gets punched at again, because both NATs may simply have dropped
// their state while nothing was flowing.
func TestAQuietPathIsPunchedAgain(t *testing.T) {
	home, peer := listen(t), listen(t)
	defer home.Close()
	defer peer.Close()

	session := wired(t, home, plainCodec{}, &syncBuffer{})
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

// Holding the secret does not make you the peer. Everyone handed the same
// invite seals under the same key, so a third party's frames authenticate just
// as well as the peer's: without the sender check, following one hands the
// session to whoever showed up last and evicts the peer in silence.
func TestAThirdHolderOfTheSecretCannotStealTheSession(t *testing.T) {
	home, peer, third := listen(t), listen(t), listen(t)
	defer home.Close()
	defer peer.Close()
	defer third.Close()

	secret, err := NewSecret()
	if err != nil {
		t.Fatalf("cannot generate a secret: %v", err)
	}
	homeCodec, err := NewSecretCodec(secret, RoleInviter)
	if err != nil {
		t.Fatalf("cannot build the codec: %v", err)
	}
	// A separate codec is exactly what a third process holding the same invite
	// has: same keys, its own sender.
	thirdCodec, err := NewSecretCodec(secret, RoleJoiner)
	if err != nil {
		t.Fatalf("cannot build the codec: %v", err)
	}
	peerCodec, err := NewSecretCodec(secret, RoleJoiner)
	if err != nil {
		t.Fatalf("cannot build the codec: %v", err)
	}

	observer := newRecorder(4)
	session := wired(t, home, homeCodec, &syncBuffer{})
	session.Observe(observer)
	session.SetPeer(localAddr(t, peer))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go session.Run(ctx)

	// The peer speaks first, which is what pins it to the session.
	_, _ = peer.WriteToUDP(peerCodec.Seal(kindData, "soy el par"), localAddr(t, home))
	select {
	case payload := <-observer.payloads:
		got := string(payload)
		if got != "soy el par" {
			t.Fatalf("got %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the peer never got through")
	}

	staleOut(t, session)

	for i := 0; i < 10; i++ {
		_, _ = third.WriteToUDP(thirdCodec.Seal(kindData, "soy yo ahora"), localAddr(t, home))
		time.Sleep(10 * time.Millisecond)
	}

	select {
	case payload := <-observer.payloads:
		got := string(payload)
		t.Fatalf("a third holder of the invite was heard: %q", got)
	case <-time.After(300 * time.Millisecond):
	}

	if session.Moves() != 0 {
		t.Fatalf("the session followed a third holder, %d moves", session.Moves())
	}
	if got := session.Peer(); got.Port != localAddr(t, peer).Port {
		t.Fatalf("the session is talking to %v instead of its peer", got)
	}
	// Being told is the point: silence would leave the user unaware that the
	// invite is in more hands than they thought.
	// It has to be reported as what it is: a holder of the invite, not a
	// scanner. The two call for different answers.
	probes := session.Probes()
	if probes.Impostors == 0 {
		t.Fatalf("a third holder of the invite was not reported as one: %+v", probes)
	}
}

// The peer itself still has to be followable when it moves.
func TestTheRealPeerIsStillFollowed(t *testing.T) {
	home, oldAddr, newAddr := listen(t), listen(t), listen(t)
	defer home.Close()
	defer oldAddr.Close()
	defer newAddr.Close()

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

	observer := newRecorder(4)
	session := wired(t, home, homeCodec, &syncBuffer{})
	session.Observe(observer)
	session.SetPeer(localAddr(t, oldAddr))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go session.Run(ctx)

	_, _ = oldAddr.WriteToUDP(peerCodec.Seal(kindData, "hola"), localAddr(t, home))
	<-observer.payloads

	staleOut(t, session)

	// Same codec, new address: the peer moved.
	_, _ = newAddr.WriteToUDP(peerCodec.Seal(kindData, "me mude"), localAddr(t, home))
	select {
	case payload := <-observer.payloads:
		got := string(payload)
		if got != "me mude" {
			t.Fatalf("got %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the peer was not followed to its new address")
	}
	if session.Moves() != 1 {
		t.Fatalf("got %d moves", session.Moves())
	}
}
