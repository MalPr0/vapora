package punch

import (
	"crypto/hkdf"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
)

const nicknameInfo = "vapora nickname v1"

// animals is the pool both sides pick from. Keeping it a power of two makes the
// pick uniform without the modulo bias a ragged list would introduce.
var animals = []string{
	"OTTER", "BADGER", "FALCON", "MARTEN", "HERON", "LYNX", "RAVEN", "BISON",
	"WOMBAT", "PANGOLIN", "CARACAL", "OKAPI", "TAPIR", "IBEX", "MARMOT", "OSPREY",
	"NARWHAL", "AXOLOTL", "QUOKKA", "SERVAL", "KESTREL", "MANATEE", "JERBOA", "FOSSA",
	"COYOTE", "PUFFIN", "GECKO", "MAGPIE", "WALRUS", "SHRIKE", "CIVET", "DINGO",
	"ORYX", "SALAMANDER", "TOUCAN", "URCHIN", "VIPER", "WEASEL", "YAK", "ZEBU",
	"ALBATROSS", "BEAVER", "CHAMOIS", "DUGONG", "EIDER", "FERRET", "GIBBON", "HOOPOE",
	"IGUANA", "JACKAL", "KRILL", "LEMUR", "MOOSE", "NUTHATCH", "OCELOT", "PLOVER",
	"QUAIL", "RACCOON", "STOAT", "TERN", "UAKARI", "VICUNA", "WAPITI", "XERUS",
}

// Nicknames are the two names a session shows. Both peers derive the same pair
// from the shared secret, so nothing has to be negotiated and neither side can
// choose how it is labelled on the other's screen.
type Nicknames struct {
	Inviter string
	Joiner  string
}

func (n Nicknames) For(role Role) string {
	if role == RoleJoiner {
		return n.Joiner
	}
	return n.Inviter
}

func (n Nicknames) Other(role Role) string {
	return n.For(role.other())
}

// Nicknames derives the pair.
func (s Secret) Nicknames() Nicknames {
	first := s.pick(0)
	second := s.pick(1)
	if second == first {
		// One rotation is enough: the pool is larger than two, so the pair can
		// always be made distinct without another derivation.
		second = animals[(indexOf(first)+1)%len(animals)]
	}
	return Nicknames{Inviter: first, Joiner: second}
}

// pick derives one name. The slot goes in the info string rather than indexing
// into a fixed buffer, which is what stops a third name from reading past the
// end of it.
func (s Secret) pick(slot int) string {
	key, err := hkdf.Key(sha256.New, s, nil, fmt.Sprintf("%s %d", nicknameInfo, slot), 4)
	if err != nil {
		return animals[slot%len(animals)]
	}
	return animals[int(binary.BigEndian.Uint32(key))%len(animals)]
}

func indexOf(name string) int {
	for i, animal := range animals {
		if animal == name {
			return i
		}
	}
	return 0
}
