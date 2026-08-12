package punch

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base32"
	"errors"
	"fmt"
)

// secretBytes is the entropy behind an invite. 128 bits is beyond brute force
// for a secret that lives as long as a chat session.
const secretBytes = 16

// inviteEncoding avoids padding so the invite stays a single clean token.
var inviteEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// ErrSecretMismatch means two secrets that were supposed to be the same are
// not. Compared in constant time, so failing does not leak where.
var ErrSecretMismatch = errors.New("punch: secret does not match")

// Secret is the shared key carried by an invite. It authenticates the peer:
// only whoever received the invite can produce frames this session accepts.
type Secret []byte

// NewSecret mints one. This is the key to the whole conversation, not a name
// for it: whoever holds it can join, and everything is encrypted with it.
func NewSecret() (Secret, error) {
	secret := make(Secret, secretBytes)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("punch: cannot generate a secret: %w", err)
	}
	return secret, nil
}

// ParseSecret reads one back from the text on an invite.
func ParseSecret(encoded string) (Secret, error) {
	secret, err := inviteEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("punch: %q is not a valid secret: %w", encoded, err)
	}
	if len(secret) != secretBytes {
		return nil, fmt.Errorf("punch: a secret is %d bytes, got %d", secretBytes, len(secret))
	}
	return secret, nil
}

// String is the base32 form that appears on an invite.
func (s Secret) String() string {
	return inviteEncoding.EncodeToString(s)
}

// Equal compares in constant time, so a mismatch does not leak where it
// happened.
func (s Secret) Equal(other Secret) bool {
	return subtle.ConstantTimeCompare(s, other) == 1
}
