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

var ErrSecretMismatch = errors.New("punch: secret does not match")

// Secret is the shared key carried by an invite. It authenticates the peer:
// only whoever received the invite can produce frames this session accepts.
type Secret []byte

func NewSecret() (Secret, error) {
	secret := make(Secret, secretBytes)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("punch: cannot generate a secret: %w", err)
	}
	return secret, nil
}

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

func (s Secret) String() string {
	return inviteEncoding.EncodeToString(s)
}

func (s Secret) Equal(other Secret) bool {
	return subtle.ConstantTimeCompare(s, other) == 1
}
