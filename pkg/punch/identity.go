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

const nicknameSuffix = 2

// MaxMembers caps a room. Seven pairs at one ping every five seconds is under
// three packets a second, a roster of eight fits a datagram comfortably, and
// twenty eight punched paths is where restrictive NATs start failing in earnest.
const MaxMembers = 8

// Nickname is the name shown for this participant. It comes from the key, so
// everyone computes the same name for everyone with nothing sent and nothing to
// trust, and nobody can choose how they appear on somebody else's screen.
//
// The suffix is not decoration: with sixty four animals and eight people in a
// room, two sharing a name is likelier than not, and a room where two rows say
// OTTER is worse than one where they say OTTER-K3 and OTTER-9F.
func (k PublicKey) Nickname() string {
	material, err := hkdf.Key(sha256.New, k[:], nil, nicknameInfo, 8)
	if err != nil {
		return animals[0]
	}

	animal := animals[int(binary.BigEndian.Uint32(material[0:4]))%len(animals)]
	return animal + "-" + inviteEncoding.EncodeToString(material[4:8])[:nicknameSuffix]
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
