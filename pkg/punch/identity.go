package punch

import (
	"crypto/ecdh"
	"crypto/rand"
	"errors"
	"fmt"
)

// PublicKeySize is the width of an X25519 public key.
const PublicKeySize = 32

// ErrBadPublicKey covers a key that is not one: wrong length, not base32, or
// all zeroes, which X25519 would otherwise accept into a useless shared value.
var ErrBadPublicKey = errors.New("punch: not a usable public key")

// PublicKey identifies a participant. It is the identity everything else hangs
// off: pair keys are derived with it and a roster entry is keyed by it, so
// nothing about who somebody is has to be negotiated or trusted. Turning one
// into a name a person can say is pkg/names, and is nothing to do with here.
type PublicKey [PublicKeySize]byte

// String is the base32 form, which is what invites carry.
func (k PublicKey) String() string {
	return inviteEncoding.EncodeToString(k[:])
}

// ParsePublicKey reads a key back from its text form.
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

// Identity is this participant's keypair. The private half never leaves, which
// is what keeps a member from reading or forging traffic between two others.
type Identity struct {
	private *ecdh.PrivateKey
	public  PublicKey
}

// NewIdentity generates a keypair for this process.
//
// It is not persisted anywhere on purpose: a fresh identity every run means
// there is nothing to steal from disk and nothing that links two sessions to
// the same person.
func NewIdentity() (*Identity, error) {
	private, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("punch: cannot generate an identity: %w", err)
	}

	identity := &Identity{private: private}
	copy(identity.public[:], private.PublicKey().Bytes())
	return identity, nil
}

// Public is the half that is safe to share, and is what a member is known by.
func (i *Identity) Public() PublicKey { return i.public }

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
