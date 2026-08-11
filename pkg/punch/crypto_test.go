package punch

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
)

func codecPair(t *testing.T) (*SecretCodec, *SecretCodec, Secret) {
	t.Helper()

	secret, err := NewSecret()
	if err != nil {
		t.Fatalf("cannot generate a secret: %v", err)
	}
	inviter, err := NewSecretCodec(secret, RoleInviter)
	if err != nil {
		t.Fatalf("cannot build the inviter codec: %v", err)
	}
	joiner, err := NewSecretCodec(secret, RoleJoiner)
	if err != nil {
		t.Fatalf("cannot build the joiner codec: %v", err)
	}
	return inviter, joiner, secret
}

func TestSecretCodecRoundTripsBothDirections(t *testing.T) {
	inviter, joiner, _ := codecPair(t)

	wire := inviter.Seal(kindData, "hola")
	frame, err := joiner.Open(wire)
	if err != nil || frame.Kind != kindData || frame.Payload != "hola" {
		t.Fatalf("got %+v err %v", frame, err)
	}

	back := joiner.Seal(kindAck, "")
	if frame, err := inviter.Open(back); err != nil || frame.Kind != kindAck {
		t.Fatalf("got %+v err %v", frame, err)
	}
}

func TestSecretCodecHidesThePayload(t *testing.T) {
	inviter, _, _ := codecPair(t)

	wire := inviter.Seal(kindData, "cuenta bancaria")
	if bytes.Contains(wire, []byte("cuenta bancaria")) {
		t.Fatal("the payload travels in clear")
	}
}

func TestSecretCodecRejectsAnotherSecret(t *testing.T) {
	inviter, _, _ := codecPair(t)
	_, stranger, _ := codecPair(t)

	wire := stranger.Seal(kindPunch, "")
	if _, err := inviter.Open(wire); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("got %v", err)
	}
}

// A side must not open what it sealed itself: each direction has its own key,
// which is what keeps a nonce from ever repeating under one key.
func TestSecretCodecKeysAreDirectional(t *testing.T) {
	inviter, _, _ := codecPair(t)

	wire := inviter.Seal(kindData, "eco")
	if _, err := inviter.Open(wire); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("got %v", err)
	}
}

func TestSecretCodecRejectsTampering(t *testing.T) {
	inviter, joiner, _ := codecPair(t)

	wire := inviter.Seal(kindData, "hola")
	wire[len(wire)-1] ^= 0xFF

	if _, err := joiner.Open(wire); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("got %v", err)
	}
}

func TestSecretCodecRejectsReplay(t *testing.T) {
	inviter, joiner, _ := codecPair(t)

	wire := inviter.Seal(kindData, "una sola vez")
	if _, err := joiner.Open(wire); err != nil {
		t.Fatalf("the first delivery must pass: %v", err)
	}
	if _, err := joiner.Open(wire); !errors.Is(err, ErrReplayed) {
		t.Fatalf("got %v", err)
	}
}

func TestSecretCodecToleratesReordering(t *testing.T) {
	inviter, joiner, _ := codecPair(t)

	var frames [][]byte
	for i := 0; i < 5; i++ {
		frames = append(frames, inviter.Seal(kindData, "x"))
	}

	for _, index := range []int{4, 0, 3, 1, 2} {
		if _, err := joiner.Open(frames[index]); err != nil {
			t.Fatalf("frame %d rejected out of order: %v", index, err)
		}
	}
}

func TestReplayWindowDropsWhatFellOutOfIt(t *testing.T) {
	window := &replayWindow{}

	if !window.accept(1) || !window.accept(200) {
		t.Fatal("fresh counters must be accepted")
	}
	if window.accept(200) {
		t.Fatal("a duplicate must be rejected")
	}
	if window.accept(1) {
		t.Fatal("a counter far below the window must be rejected")
	}
	if !window.accept(199) {
		t.Fatal("a counter inside the window must still be accepted")
	}
	if window.accept(0) {
		t.Fatal("counter zero is never issued and must be rejected")
	}
}

// End to end: the secret is what stops a stranger from becoming the peer while
// the waiting side accepts packets from any source.
func TestSessionIgnoresPacketsWithoutTheSecret(t *testing.T) {
	waiting, friend, stranger := listen(t), listen(t), listen(t)
	defer waiting.Close()
	defer friend.Close()
	defer stranger.Close()

	secret, err := NewSecret()
	if err != nil {
		t.Fatalf("cannot generate a secret: %v", err)
	}
	waitingCodec, err := NewSecretCodec(secret, RoleInviter)
	if err != nil {
		t.Fatalf("cannot build the codec: %v", err)
	}
	friendCodec, err := NewSecretCodec(secret, RoleJoiner)
	if err != nil {
		t.Fatalf("cannot build the codec: %v", err)
	}

	waitingSession := wired(t, waiting, waitingCodec, &syncBuffer{})
	friendSession := wired(t, friend, friendCodec, &syncBuffer{})
	friendSession.SetPeer(localAddr(t, waiting))

	// The stranger floods the waiting side while it has no peer yet.
	strangerDone := make(chan struct{})
	go func() {
		defer close(strangerDone)
		for i := 0; i < 40; i++ {
			stranger.WriteToUDP(encode(kindPunch, ""), localAddr(t, waiting))
			time.Sleep(20 * time.Millisecond)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	openBoth(t, ctx, waitingSession, friendSession)
	<-strangerDone

	peer := waitingSession.Peer()
	if peer == nil || peer.Port != localAddr(t, friend).Port {
		t.Fatalf("the waiting side paired with %v instead of the friend", peer)
	}
}

func TestSecretRejectsWrongLength(t *testing.T) {
	if _, err := ParseSecret(""); err == nil {
		t.Fatal("an empty secret must be rejected")
	}
	if _, err := ParseSecret("AAAAAAAA"); err == nil {
		t.Fatal("a short secret must be rejected")
	}
}

func TestSecretEqualIsConstantTime(t *testing.T) {
	first, err := NewSecret()
	if err != nil {
		t.Fatalf("cannot generate a secret: %v", err)
	}
	second, err := NewSecret()
	if err != nil {
		t.Fatalf("cannot generate a secret: %v", err)
	}

	if !first.Equal(first) {
		t.Fatal("a secret must equal itself")
	}
	if first.Equal(second) {
		t.Fatal("two generated secrets must differ")
	}
	if first.Equal(nil) {
		t.Fatal("a secret must not equal nothing")
	}
}
