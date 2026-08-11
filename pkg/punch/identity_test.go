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

	words := strings.Fields(key.Nickname())
	if len(words) != 3 {
		t.Fatalf("got %q", key.Nickname())
	}
	inPool := func(pool []string, word string) bool {
		for _, candidate := range pool {
			if candidate == word {
				return true
			}
		}
		return false
	}
	if !inPool(adjectives, words[0]) || !inPool(colours, words[1]) || !inPool(animals, words[2]) {
		t.Fatalf("%q is not built from the pools", key.Nickname())
	}
	if key.Colour() != words[1] {
		t.Fatalf("Colour said %q for %q", key.Colour(), key.Nickname())
	}
}

// A name only has to be long enough to tell apart who is actually present, so
// a small room gets bare animals and the extra words appear where they earn
// their space.
func TestShortNamesGrowOnlyWhenTheyHaveTo(t *testing.T) {
	var keys []PublicKey
	for i := 0; i < MaxMembers; i++ {
		keys = append(keys, identity(t).Public())
	}

	names := ShortNames(keys)
	if len(names) != len(keys) {
		t.Fatalf("named %d of %d", len(names), len(keys))
	}

	seen := map[string]PublicKey{}
	for key, name := range names {
		if other, clash := seen[name]; clash && other != key {
			t.Fatalf("%q named two members", name)
		}
		seen[name] = key
		if words := len(strings.Fields(name)); words < 1 || words > 3 {
			t.Fatalf("%q has %d words", name, words)
		}
	}
}

// Two keys that share an animal have to be told apart, and the one word that
// does it is the one that gets added.
func TestShortNamesDisambiguateACollision(t *testing.T) {
	var first, second PublicKey
	for attempt := 0; attempt < 20000; attempt++ {
		candidate := identity(t).Public()
		if first.isZero() {
			first = candidate
			continue
		}
		if candidate.name(1) == first.name(1) && candidate != first {
			second = candidate
			break
		}
	}
	if second.isZero() {
		t.Skip("no animal collision came up in twenty thousand keys")
	}

	names := ShortNames([]PublicKey{first, second})
	if names[first] == names[second] {
		t.Fatalf("both were named %q", names[first])
	}
	for _, key := range []PublicKey{first, second} {
		if len(strings.Fields(names[key])) < 2 {
			t.Fatalf("a collision was left as the bare animal %q", names[key])
		}
	}
}

// A lone member does not need qualifying.
func TestShortNamesStayShortAlone(t *testing.T) {
	key := identity(t).Public()
	if got := ShortNames([]PublicKey{key})[key]; strings.Contains(got, " ") {
		t.Fatalf("a room of one got %q", got)
	}
}
