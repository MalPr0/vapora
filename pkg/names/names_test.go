package names

import (
	"crypto/rand"
	"strings"
	"testing"
)

// identity is a random key, which is all this package needs to name somebody.
func identity(t *testing.T) Key {
	t.Helper()

	var key Key
	if _, err := rand.Read(key[:]); err != nil {
		t.Fatal(err)
	}
	return key
}

func TestNicknamesFromKeys(t *testing.T) {
	key := identity(t)
	if Full(key) != Full(key) {
		t.Fatal("a key gave two different names")
	}

	words := strings.Fields(Full(key))
	if len(words) != 3 {
		t.Fatalf("got %q", Full(key))
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
		t.Fatalf("%q is not built from the pools", Full(key))
	}
	if Colour(key) != words[1] {
		t.Fatalf("Colour said %q for %q", Colour(key), Full(key))
	}
}

// A name only has to be long enough to tell apart who is actually present, so
// a small room gets bare animals and the extra words appear where they earn
// their space.
func TestShortNamesGrowOnlyWhenTheyHaveTo(t *testing.T) {
	var keys []Key
	for i := 0; i < 8; i++ {
		keys = append(keys, identity(t))
	}

	names := Short(keys)
	if len(names) != len(keys) {
		t.Fatalf("named %d of %d", len(names), len(keys))
	}

	seen := map[string]Key{}
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
	var first, second Key
	for attempt := 0; attempt < 20000; attempt++ {
		candidate := identity(t)
		if first == (Key{}) {
			first = candidate
			continue
		}
		if Of(candidate, 1) == Of(first, 1) && candidate != first {
			second = candidate
			break
		}
	}
	if second == (Key{}) {
		t.Skip("no animal collision came up in twenty thousand keys")
	}

	names := Short([]Key{first, second})
	if names[first] == names[second] {
		t.Fatalf("both were named %q", names[first])
	}
	for _, key := range []Key{first, second} {
		if len(strings.Fields(names[key])) < 2 {
			t.Fatalf("a collision was left as the bare animal %q", names[key])
		}
	}
}

// A lone member does not need qualifying.
func TestShortNamesStayShortAlone(t *testing.T) {
	key := identity(t)
	if got := Short([]Key{key})[key]; strings.Contains(got, " ") {
		t.Fatalf("a room of one got %q", got)
	}
}
