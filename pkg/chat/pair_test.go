package chat

import (
	"strings"
	"testing"

	"github.com/MalPr0/vapora/pkg/names"
	"github.com/MalPr0/vapora/pkg/punch"
)

// Both ends derive the pair from the shared secret, so neither has to send a
// name and neither can choose how it is labelled on the other's screen.
func TestNicknamesAgreeAcrossPeers(t *testing.T) {
	secret, err := punch.NewSecret()
	if err != nil {
		t.Fatalf("cannot generate a secret: %v", err)
	}

	inviterView := NamePair(secret)
	joinerView := NamePair(secret)
	if inviterView != joinerView {
		t.Fatalf("the two sides derived %+v and %+v", inviterView, joinerView)
	}
	if inviterView.For(punch.RoleInviter) != joinerView.For(punch.RoleInviter) {
		t.Fatal("the sides disagree on who is who")
	}
	if inviterView.Other(punch.RoleInviter) != inviterView.For(punch.RoleJoiner) {
		t.Fatal("Other did not return the opposite role")
	}
}

func TestNicknamesAreDistinctAndFromThePool(t *testing.T) {
	pool := map[string]bool{}
	for _, animal := range names.Animals() {
		pool[animal] = true
	}

	for attempt := 0; attempt < 200; attempt++ {
		secret, err := punch.NewSecret()
		if err != nil {
			t.Fatalf("cannot generate a secret: %v", err)
		}

		names := NamePair(secret)
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
	seen := map[Pair]bool{}
	for attempt := 0; attempt < 50; attempt++ {
		secret, err := punch.NewSecret()
		if err != nil {
			t.Fatalf("cannot generate a secret: %v", err)
		}
		seen[NamePair(secret)] = true
	}
	if len(seen) < 25 {
		t.Fatalf("50 secrets only produced %d distinct pairs", len(seen))
	}
}

// An unauthenticated session still has to show something.
func TestNicknamesWithoutASecret(t *testing.T) {
	names := NamePair(nil)
	if names.Inviter == "" || names.Joiner == "" || names.Inviter == names.Joiner {
		t.Fatalf("got %+v", names)
	}
}

// The typing state has to survive the wire, encrypted like everything else.
