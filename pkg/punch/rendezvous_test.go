package punch

import (
	"bytes"
	"testing"
)

// The key is derived, not the secret itself. Publishing the secret would hand
// the conversation to anyone watching the DHT, which is a public network.
func TestTheRendezvousKeyIsNotTheSecret(t *testing.T) {
	secret, err := NewSecret()
	if err != nil {
		t.Fatal(err)
	}

	key, err := RendezvousKey(secret)
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Contains(key[:], secret[:8]) {
		t.Fatal("the key carries the secret, so watching the DHT would be enough to read the room")
	}

	// The same secret has to reach the same place, or the two sides never meet.
	again, err := RendezvousKey(secret)
	if err != nil {
		t.Fatal(err)
	}
	if again != key {
		t.Fatal("the same secret produced two different meeting points")
	}
}

// Two conversations must not meet at the same place, or each would find the
// other's addresses and waste every packet on somebody who cannot answer.
func TestDifferentSecretsMeetElsewhere(t *testing.T) {
	seen := map[[20]byte]bool{}

	for i := 0; i < 200; i++ {
		secret, err := NewSecret()
		if err != nil {
			t.Fatal(err)
		}
		key, err := RendezvousKey(secret)
		if err != nil {
			t.Fatal(err)
		}
		if seen[key] {
			t.Fatal("two secrets landed on the same meeting point")
		}
		seen[key] = true
	}
}
