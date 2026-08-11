package punch

import (
	"errors"
	"strings"
	"testing"
)

func identity(t *testing.T) *Identity {
	t.Helper()

	value, err := NewIdentity()
	if err != nil {
		t.Fatalf("cannot generate an identity: %v", err)
	}
	return value
}

func TestPublicKeyRoundTrips(t *testing.T) {
	key := identity(t).Public()

	parsed, err := ParsePublicKey(key.String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed != key {
		t.Fatal("a key did not survive the round trip")
	}

	for _, broken := range []string{"", "not base32!", "AAAA", strings.Repeat("A", 100)} {
		if _, err := ParsePublicKey(broken); !errors.Is(err, ErrBadPublicKey) {
			t.Fatalf("%q gave %v", broken, err)
		}
	}
}

// Both ends of a pair have to derive the same two keys, crossed, with nothing
// exchanged to decide which is which.
func TestPairKeysAreSymmetricAndCrossed(t *testing.T) {
	alice, bob := identity(t), identity(t)
	room, err := NewSecret()
	if err != nil {
		t.Fatalf("cannot generate a secret: %v", err)
	}

	aliceCodec, err := alice.PairCodec(bob.Public(), room)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bobCodec, err := bob.PairCodec(alice.Public(), room)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sealed := aliceCodec.Seal(kindMessage, "hola")
	frame, err := bobCodec.Open(sealed)
	if err != nil || frame.Payload != "hola" {
		t.Fatalf("got %+v err %v", frame, err)
	}

	back := bobCodec.Seal(kindMessage, "chau")
	if frame, err := aliceCodec.Open(back); err != nil || frame.Payload != "chau" {
		t.Fatalf("got %+v err %v", frame, err)
	}

	if alice.Direction(bob.Public()) == bob.Direction(alice.Public()) {
		t.Fatal("both sides claimed the same direction")
	}
}

// Direction comes from comparing keys, so it cannot depend on who asked first.
func TestDirectionIsDecidedByTheKeys(t *testing.T) {
	for attempt := 0; attempt < 50; attempt++ {
		alice, bob := identity(t), identity(t)

		low, high := Between(alice.Public(), bob.Public())
		if low2, high2 := Between(bob.Public(), alice.Public()); low2 != low || high2 != high {
			t.Fatal("canonical order depended on argument order")
		}

		if alice.Public() == low && alice.Direction(bob.Public()) != DirectionLow {
			t.Fatal("the lower key did not seal as low")
		}
	}
}

// A third member of the room holds the room secret but neither private half, so
// it can neither read nor forge what a pair says.
func TestAThirdMemberCannotOpenAPairChannel(t *testing.T) {
	alice, bob, carol := identity(t), identity(t), identity(t)
	room, err := NewSecret()
	if err != nil {
		t.Fatalf("cannot generate a secret: %v", err)
	}

	aliceCodec, err := alice.PairCodec(bob.Public(), room)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sealed := aliceCodec.Seal(kindMessage, "solo para bob")

	// Carol tries every channel it can build with what it has.
	for _, peer := range []PublicKey{alice.Public(), bob.Public()} {
		codec, err := carol.PairCodec(peer, room)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err := codec.Open(sealed); !errors.Is(err, ErrUnauthenticated) {
			t.Fatalf("a third member opened a pair channel: %v", err)
		}
	}

	// Nor can it forge into it.
	forged, err := carol.PairCodec(bob.Public(), room)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bobCodec, err := bob.PairCodec(alice.Public(), room)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := bobCodec.Open(forged.Seal(kindMessage, "soy alice")); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("a third member forged into a pair channel: %v", err)
	}
}

// The room secret is the salt, so the same two people in two rooms do not share
// a channel.
func TestPairKeysDifferBetweenRooms(t *testing.T) {
	alice, bob := identity(t), identity(t)
	first, _ := NewSecret()
	second, _ := NewSecret()

	inFirst, err := alice.PairCodec(bob.Public(), first)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	inSecond, err := bob.PairCodec(alice.Public(), second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := inSecond.Open(inFirst.Seal(kindMessage, "hola")); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("a channel from one room opened in another: %v", err)
	}
}

func TestPairCodecRejectsNonsense(t *testing.T) {
	alice := identity(t)
	room, _ := NewSecret()

	if _, err := alice.PairCodec(PublicKey{}, room); !errors.Is(err, ErrBadPublicKey) {
		t.Fatalf("an all zero key gave %v", err)
	}
	if _, err := alice.PairCodec(alice.Public(), room); !errors.Is(err, ErrBadPublicKey) {
		t.Fatalf("pairing with itself gave %v", err)
	}
}

// Every member computes the same name for every member, and a room of eight
// should not have two rows saying the same thing.
func TestNicknamesFromKeys(t *testing.T) {
	key := identity(t).Public()
	if key.Nickname() != key.Nickname() {
		t.Fatal("a key gave two different names")
	}

	name, suffix, found := strings.Cut(key.Nickname(), "-")
	if !found || len(suffix) != nicknameSuffix {
		t.Fatalf("got %q", key.Nickname())
	}
	pool := map[string]bool{}
	for _, animal := range animals {
		pool[animal] = true
	}
	if !pool[name] {
		t.Fatalf("%q is not from the pool", name)
	}

	// The number that matters is a room, not a crowd: sixty four animals times
	// a thousand suffixes is sixty five thousand names, so any large enough
	// sample collides by the birthday bound and says nothing. Eight people
	// sharing a screen is the case the suffix exists for.
	const rooms, size = 2000, MaxMembers
	clashes := 0
	for room := 0; room < rooms; room++ {
		names := map[string]bool{}
		for member := 0; member < size; member++ {
			name := identity(t).Public().Nickname()
			if names[name] {
				clashes++
				break
			}
			names[name] = true
		}
	}
	if clashes*100 > rooms {
		t.Fatalf("%d of %d rooms of %d had two members sharing a name", clashes, rooms, size)
	}
}
