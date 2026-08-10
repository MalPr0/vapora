package punch

import (
	"context"
	"strings"
	"testing"
	"time"
)

// Both ends derive the pair from the shared secret, so neither has to send a
// name and neither can choose how it is labelled on the other's screen.
func TestNicknamesAgreeAcrossPeers(t *testing.T) {
	secret, err := NewSecret()
	if err != nil {
		t.Fatalf("cannot generate a secret: %v", err)
	}

	inviterView := secret.Nicknames()
	joinerView := secret.Nicknames()
	if inviterView != joinerView {
		t.Fatalf("the two sides derived %+v and %+v", inviterView, joinerView)
	}
	if inviterView.For(RoleInviter) != joinerView.For(RoleInviter) {
		t.Fatal("the sides disagree on who is who")
	}
	if inviterView.Other(RoleInviter) != inviterView.For(RoleJoiner) {
		t.Fatal("Other did not return the opposite role")
	}
}

func TestNicknamesAreDistinctAndFromThePool(t *testing.T) {
	pool := map[string]bool{}
	for _, animal := range animals {
		pool[animal] = true
	}

	for attempt := 0; attempt < 200; attempt++ {
		secret, err := NewSecret()
		if err != nil {
			t.Fatalf("cannot generate a secret: %v", err)
		}

		names := secret.Nicknames()
		if names.Inviter == names.Joiner {
			t.Fatalf("both sides got %q", names.Inviter)
		}
		if !pool[names.Inviter] || !pool[names.Joiner] {
			t.Fatalf("got %+v, which is not from the pool", names)
		}
		if strings.ToUpper(names.Inviter) != names.Inviter {
			t.Fatalf("%q is not the expected shape", names.Inviter)
		}
	}
}

func TestNicknamesDifferBetweenSecrets(t *testing.T) {
	seen := map[Nicknames]bool{}
	for attempt := 0; attempt < 50; attempt++ {
		secret, err := NewSecret()
		if err != nil {
			t.Fatalf("cannot generate a secret: %v", err)
		}
		seen[secret.Nicknames()] = true
	}
	if len(seen) < 25 {
		t.Fatalf("50 secrets only produced %d distinct pairs", len(seen))
	}
}

// An unauthenticated session still has to show something.
func TestNicknamesWithoutASecret(t *testing.T) {
	names := Secret(nil).Nicknames()
	if names.Inviter == "" || names.Joiner == "" || names.Inviter == names.Joiner {
		t.Fatalf("got %+v", names)
	}
}

// The typing state has to survive the wire, encrypted like everything else.
func TestTypingReachesThePeer(t *testing.T) {
	left, right := listen(t), listen(t)
	defer left.Close()
	defer right.Close()

	observer := &recordingObserver{typing: make(chan bool, 4), messages: make(chan string, 4)}
	leftSession := NewSession(left, PlainCodec{}, &syncBuffer{})
	rightSession := NewSession(right, PlainCodec{}, &syncBuffer{})
	rightSession.Observe(observer)

	leftSession.SetPeer(localAddr(t, right))
	rightSession.SetPeer(localAddr(t, left))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	openBoth(t, ctx, leftSession, rightSession)

	go rightSession.Run(ctx)

	leftSession.SetTyping(true)
	if active := receiveTyping(t, observer); !active {
		t.Fatal("the peer was not told that typing started")
	}

	leftSession.SetTyping(false)
	if active := receiveTyping(t, observer); active {
		t.Fatal("the peer was not told that typing stopped")
	}

	// A delivered message clears the indicator without needing its own frame.
	leftSession.SendMessage("hola")
	select {
	case got := <-observer.messages:
		if got != "hola" {
			t.Fatalf("got %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the message never arrived")
	}
}

func receiveTyping(t *testing.T, observer *recordingObserver) bool {
	t.Helper()
	select {
	case active := <-observer.typing:
		return active
	case <-time.After(2 * time.Second):
		t.Fatal("no typing update arrived")
		return false
	}
}

type recordingObserver struct {
	typing   chan bool
	messages chan string
}

func (o *recordingObserver) Message(payload string) { o.messages <- payload }
func (o *recordingObserver) Typing(active bool)     { o.typing <- active }
