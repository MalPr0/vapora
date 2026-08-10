package punch

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
)

// Role tells the two ends apart so each direction gets its own key and the
// same nonce can never be used twice under one key.
type Role string

const (
	RoleInviter Role = "inviter"
	RoleJoiner  Role = "joiner"
)

func (r Role) other() Role {
	if r == RoleInviter {
		return RoleJoiner
	}
	return RoleInviter
}

const (
	keyBytes    = 32
	prefixBytes = 4
	nonceBytes  = 12
	keyInfo     = "vapora punch v1 "
)

var (
	ErrUnauthenticated = errors.New("punch: frame does not authenticate, wrong secret or foreign packet")
	ErrReplayed        = errors.New("punch: frame replays a nonce already seen")
)

// Codec turns frames into wire bytes and back. Session depends on this instead
// of a concrete cipher so an unauthenticated session stays a drop in.
type Codec interface {
	Seal(kind byte, payload string) []byte
	Open(wire []byte) (byte, string, error)
}

// SecretCodec encrypts and authenticates every frame with AES-256-GCM, keyed by
// the invite secret. A packet from anyone without the invite fails to open, so
// authentication and confidentiality come from the same primitive.
type SecretCodec struct {
	sealAEAD cipher.AEAD
	openAEAD cipher.AEAD

	mu      sync.Mutex
	prefix  [prefixBytes]byte
	counter uint64
	seen    map[[prefixBytes]byte]*replayWindow
}

func NewSecretCodec(secret Secret, role Role) (*SecretCodec, error) {
	sealAEAD, err := aeadFor(secret, role)
	if err != nil {
		return nil, err
	}
	openAEAD, err := aeadFor(secret, role.other())
	if err != nil {
		return nil, err
	}

	codec := &SecretCodec{
		sealAEAD: sealAEAD,
		openAEAD: openAEAD,
		seen:     map[[prefixBytes]byte]*replayWindow{},
	}
	if _, err := rand.Read(codec.prefix[:]); err != nil {
		return nil, fmt.Errorf("punch: cannot generate a nonce prefix: %w", err)
	}
	return codec, nil
}

func aeadFor(secret Secret, role Role) (cipher.AEAD, error) {
	// The secret is full entropy from crypto/rand, so a KDF is all that is
	// needed here; password stretching would be solving a problem we do not
	// have.
	key, err := hkdf.Key(sha256.New, secret, nil, keyInfo+string(role), keyBytes)
	if err != nil {
		return nil, fmt.Errorf("punch: cannot derive the %s key: %w", role, err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("punch: cannot build the cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("punch: cannot build the AEAD: %w", err)
	}
	return aead, nil
}

func (c *SecretCodec) Seal(kind byte, payload string) []byte {
	c.mu.Lock()
	c.counter++
	nonce := c.nonceFor(c.prefix, c.counter)
	c.mu.Unlock()

	wire := make([]byte, 0, nonceBytes+len(payload)+1+c.sealAEAD.Overhead())
	wire = append(wire, nonce...)
	return c.sealAEAD.Seal(wire, nonce, encode(kind, payload), nil)
}

func (c *SecretCodec) Open(wire []byte) (byte, string, error) {
	if len(wire) < nonceBytes+c.openAEAD.Overhead() {
		return 0, "", ErrUnauthenticated
	}

	nonce := wire[:nonceBytes]
	plain, err := c.openAEAD.Open(nil, nonce, wire[nonceBytes:], nil)
	if err != nil {
		return 0, "", ErrUnauthenticated
	}

	var prefix [prefixBytes]byte
	copy(prefix[:], nonce[:prefixBytes])
	counter := binary.BigEndian.Uint64(nonce[prefixBytes:])
	if !c.accept(prefix, counter) {
		return 0, "", ErrReplayed
	}

	return decode(plain)
}

func (c *SecretCodec) nonceFor(prefix [prefixBytes]byte, counter uint64) []byte {
	nonce := make([]byte, nonceBytes)
	copy(nonce, prefix[:])
	binary.BigEndian.PutUint64(nonce[prefixBytes:], counter)
	return nonce
}

func (c *SecretCodec) accept(prefix [prefixBytes]byte, counter uint64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	window, ok := c.seen[prefix]
	if !ok {
		window = &replayWindow{}
		c.seen[prefix] = window
	}
	return window.accept(counter)
}

// plainCodec is the unauthenticated wire format. It is unexported on purpose:
// a session reachable from the internet has no business running without the
// AEAD, and this exists so tests can exercise the framing on its own.
type plainCodec struct{}

func (plainCodec) Seal(kind byte, payload string) []byte {
	return encode(kind, payload)
}

func (plainCodec) Open(wire []byte) (byte, string, error) {
	return decode(wire)
}
