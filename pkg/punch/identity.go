package punch

import (
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
)

// PublicKeySize is the width of an X25519 public key.
const PublicKeySize = 32

var ErrBadPublicKey = errors.New("punch: not a usable public key")

// PublicKey identifies a participant. It is the identity everything else hangs
// off: names are derived from it, pair keys are derived with it, and a roster
// entry is keyed by it, so nothing about who somebody is has to be negotiated
// or trusted.
type PublicKey [PublicKeySize]byte

func (k PublicKey) String() string {
	return inviteEncoding.EncodeToString(k[:])
}

func ParsePublicKey(value string) (PublicKey, error) {
	var key PublicKey

	raw, err := inviteEncoding.DecodeString(value)
	if err != nil {
		return key, fmt.Errorf("%w: %q is not base32: %w", ErrBadPublicKey, value, err)
	}
	if len(raw) != PublicKeySize {
		return key, fmt.Errorf("%w: %d bytes, want %d", ErrBadPublicKey, len(raw), PublicKeySize)
	}
	copy(key[:], raw)
	return key, nil
}

func (k PublicKey) isZero() bool {
	return k == PublicKey{}
}

// MaxMembers caps a room. Seven pairs at one ping every five seconds is under
// three packets a second, a roster of eight fits a datagram comfortably, and
// twenty eight punched paths is where restrictive NATs start failing in earnest.
const MaxMembers = 8

// Nickname is the full name of a participant: an adjective, a colour and an
// animal, all derived from the key. Everyone computes the same one for everyone
// with nothing sent and nothing to trust, and nobody chooses how they appear on
// somebody else's screen.
//
// Most of the time only the last word or two is shown, which is what ShortNames
// is for. The colour also says what to paint the name in, so the label and the
// ink agree instead of the colour being an unrelated hash of the text.
func (k PublicKey) Nickname() string {
	adjective, colour, animal := k.nameParts()
	return adjective + " " + colour + " " + animal
}

// Colour is the word a renderer should paint this participant in.
func (k PublicKey) Colour() string {
	_, colour, _ := k.nameParts()
	return colour
}

// name returns the three words from longest suffix to shortest: an animal, then
// a colour before it, then an adjective before that.
func (k PublicKey) name(words int) string {
	adjective, colour, animal := k.nameParts()
	switch {
	case words <= 1:
		return animal
	case words == 2:
		return colour + " " + animal
	default:
		return adjective + " " + colour + " " + animal
	}
}

func (k PublicKey) nameParts() (adjective, colour, animal string) {
	material, err := hkdf.Key(sha256.New, k[:], nil, nicknameInfo, 12)
	if err != nil {
		return adjectives[0], colours[0], animals[0]
	}
	return adjectives[int(binary.BigEndian.Uint32(material[0:4]))%len(adjectives)],
		colours[int(binary.BigEndian.Uint32(material[4:8]))%len(colours)],
		animals[int(binary.BigEndian.Uint32(material[8:12]))%len(animals)]
}

// ShortNames picks the shortest name that tells these participants apart: a
// bare animal while that is unique, a colour in front of it when two share one,
// and an adjective in front of that in the rare room where two still match.
//
// It is a function of the whole set rather than of a key on its own, because
// "short enough" only means anything against who else is present. Every member
// computes it from the same roster, so the names agree without being sent.
func ShortNames(keys []PublicKey) map[PublicKey]string {
	names := make(map[PublicKey]string, len(keys))

	for words := 1; words <= 3; words++ {
		counts := map[string]int{}
		for _, key := range keys {
			counts[key.name(words)]++
		}

		remaining := keys[:0:0]
		for _, key := range keys {
			if counts[key.name(words)] == 1 || words == 3 {
				names[key] = key.name(words)
				continue
			}
			remaining = append(remaining, key)
		}
		if len(remaining) == 0 {
			break
		}
		keys = remaining
	}
	return names
}

// Identity is this participant's keypair. The private half never leaves, which
// is what keeps a member from reading or forging traffic between two others.
type Identity struct {
	private *ecdh.PrivateKey
	public  PublicKey
}

func NewIdentity() (*Identity, error) {
	private, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("punch: cannot generate an identity: %w", err)
	}

	identity := &Identity{private: private}
	copy(identity.public[:], private.PublicKey().Bytes())
	return identity, nil
}

func (i *Identity) Public() PublicKey { return i.public }

func (i *Identity) Nickname() string { return i.public.Nickname() }

// shared is the X25519 agreement with a peer. It is never used as a key on its
// own: everything runs it through a KDF with a domain that names the pair.
func (i *Identity) shared(peer PublicKey) ([]byte, error) {
	if peer.isZero() {
		return nil, fmt.Errorf("%w: all zeroes", ErrBadPublicKey)
	}

	public, err := ecdh.X25519().NewPublicKey(peer[:])
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadPublicKey, err)
	}

	secret, err := i.private.ECDH(public)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadPublicKey, err)
	}
	return secret, nil
}
