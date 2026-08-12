package chat

import (
	"crypto/hkdf"
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"github.com/MalPr0/vapora/pkg/names"
	"github.com/MalPr0/vapora/pkg/punch"
)

// Pair names the two sides of a one to one conversation.
//
// A room derives names from public keys, which a two-way session does not have:
// it is built on a shared secret and a role. Same idea, different material —
// both sides compute the same pair of names with nothing sent.
type Pair struct {
	Inviter string
	Joiner  string
}

// For is the name belonging to a role, which is how each side finds its own.
func (p Pair) For(role punch.Role) string {
	if role == punch.RoleJoiner {
		return p.Joiner
	}
	return p.Inviter
}

// Other is the name of whoever is not this role, which is how each side finds
// the person it is talking to.
func (p Pair) Other(role punch.Role) string {
	if role == punch.RoleJoiner {
		return p.Inviter
	}
	return p.Joiner
}

const pairInfo = "vapora nickname v1"

// NamePair derives the two names from the secret they share.
func NamePair(secret punch.Secret) Pair {
	first := pick(secret, 0)
	second := pick(secret, 1)
	if second == first {
		// One rotation is enough: the pool is larger than two, so the pair can
		// always be made distinct without another derivation.
		second = names.Animals()[(names.IndexOfAnimal(first)+1)%len(names.Animals())]
	}
	return Pair{Inviter: first, Joiner: second}
}

// pick derives one name. The slot goes in the info string rather than indexing
// into a fixed buffer, which is what stops a third name from reading past the
// end of it.
func pick(secret punch.Secret, slot int) string {
	animals := names.Animals()

	key, err := hkdf.Key(sha256.New, secret, nil, fmt.Sprintf("%s %d", pairInfo, slot), 4)
	if err != nil {
		return animals[slot%len(animals)]
	}
	return animals[int(binary.BigEndian.Uint32(key))%len(animals)]
}
