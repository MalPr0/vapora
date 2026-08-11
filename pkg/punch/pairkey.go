package punch

import (
	"bytes"
	"fmt"
)

// Direction names one of the two keys a pair derives.
//
// In a mesh two members have no hierarchy: neither invited the other in any
// sense that both would agree on, and a role handed out by whoever spoke first
// would have the two ends disagreeing about which key is which. Comparing the
// public keys settles it with nothing exchanged.
type Direction string

const (
	// DirectionLow is sealed by the smaller of the two public keys.
	DirectionLow  Direction = "low"
	DirectionHigh Direction = "high"
)

func (d Direction) other() Direction {
	if d == DirectionLow {
		return DirectionHigh
	}
	return DirectionLow
}

const pairInfo = "vapora pair v1"

// Between returns the two keys in canonical order, which is what lets both ends
// build the same domain string without agreeing on anything first.
func Between(a, b PublicKey) (low, high PublicKey) {
	if bytes.Compare(a[:], b[:]) < 0 {
		return a, b
	}
	return b, a
}

// Direction is which of the pair's two keys this identity seals with.
func (i *Identity) Direction(peer PublicKey) Direction {
	if bytes.Compare(i.public[:], peer[:]) < 0 {
		return DirectionLow
	}
	return DirectionHigh
}

// PairCodec is the channel between this identity and one peer. Only the two of
// them can build it: a third member of the room holds the room secret but not
// either private half, so it can neither read what they say nor forge it.
//
// The room secret is the salt rather than the key, so the same two people in two
// rooms do not share a channel; the two public keys go in the domain in
// canonical order, which binds the key to the pair and leaves a reflected
// handshake deriving something else.
func (i *Identity) PairCodec(peer PublicKey, room Secret) (*SecretCodec, error) {
	shared, err := i.shared(peer)
	if err != nil {
		return nil, err
	}
	if peer == i.public {
		return nil, fmt.Errorf("%w: a pair needs two", ErrBadPublicKey)
	}

	low, high := Between(i.public, peer)
	domain := fmt.Sprintf("%s %s %s ", pairInfo, low, high)

	mine := i.Direction(peer)
	sealKey, err := deriveKey(shared, room, domain+string(mine))
	if err != nil {
		return nil, err
	}
	openKey, err := deriveKey(shared, room, domain+string(mine.other()))
	if err != nil {
		return nil, err
	}
	return newCodec(sealKey, openKey)
}
