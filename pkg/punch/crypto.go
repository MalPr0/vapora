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
	// RoleInviter is the side that minted the secret and is waiting.
	//
	// The role decides which derived key seals and which opens, so two sides
	// built from the same secret end up mirrored.
	RoleInviter Role = "inviter"
	// RoleJoiner is the side that received the invite.
	RoleJoiner Role = "joiner"
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
	// ErrUnauthenticated means a frame did not open: the wrong secret, or simply
	// somebody else's packet arriving on a shared port. It is the ordinary case on
	// a public address and is never answered — silence is what makes a scan
	// indistinguishable from a closed port.
	ErrUnauthenticated = errors.New("punch: frame does not authenticate, wrong secret or foreign packet")
	// ErrReplayed means a frame authenticated but has been seen before. The
	// window is per sender, so one peer's traffic cannot invalidate another's.
	ErrReplayed = errors.New("punch: frame replays a nonce already seen")
)

// Codec turns frames into wire bytes and back. Session depends on this instead
// of a concrete cipher so an unauthenticated session stays a drop in.
type Codec interface {
	Seal(kind byte, payload string) []byte
	Open(wire []byte) (Opened, error)
}

// Sender identifies the codec instance that sealed a frame. Two processes
// holding the same secret still seal under different senders, because the nonce
// prefix is drawn per codec: it is what tells a peer that moved apart from
// somebody else who simply has the invite.
type Sender [prefixBytes]byte

// Opened is an authenticated frame.
type Opened struct {
	Kind    byte
	Payload string
	Sender  Sender
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

// NewSecretCodec derives the two directional keys from a shared secret.
//
// The role decides which key seals and which opens, so both sides build this
// from the same secret and end up mirrored. A key per direction means a frame
// cannot be reflected back at its sender and still authenticate.
func NewSecretCodec(secret Secret, role Role) (*SecretCodec, error) {
	sealKey, err := deriveKey(secret, nil, keyInfo+string(role))
	if err != nil {
		return nil, err
	}
	openKey, err := deriveKey(secret, nil, keyInfo+string(role.other()))
	if err != nil {
		return nil, err
	}
	return newCodec(sealKey, openKey)
}

// newCodec is the shared construction: two keys in, one channel out. What the
// keys were derived from is the caller's business, which is what lets a room
// secret and an X25519 agreement produce the same kind of channel.
func newCodec(sealKey, openKey []byte) (*SecretCodec, error) {
	sealAEAD, err := newAEAD(sealKey)
	if err != nil {
		return nil, err
	}
	openAEAD, err := newAEAD(openKey)
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

// deriveKey turns key material into one channel key. The material is always
// full entropy, from crypto/rand or from X25519, so a KDF is all that is needed
// here; password stretching would be solving a problem we do not have.
func deriveKey(material, salt []byte, info string) ([]byte, error) {
	key, err := hkdf.Key(sha256.New, material, salt, info, keyBytes)
	if err != nil {
		return nil, fmt.Errorf("punch: cannot derive the %q key: %w", info, err)
	}
	return key, nil
}

func newAEAD(key []byte) (cipher.AEAD, error) {
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

// Seal encrypts a frame under this side's sending key. Every frame carries a
// nonce whose first bytes identify this codec instance, which is what tells a
// peer that moved apart from a stranger holding the same invite.
func (c *SecretCodec) Seal(kind byte, payload string) []byte {
	c.mu.Lock()
	c.counter++
	nonce := c.nonceFor(c.prefix, c.counter)
	c.mu.Unlock()

	wire := make([]byte, 0, nonceBytes+len(payload)+1+c.sealAEAD.Overhead())
	wire = append(wire, nonce...)
	return c.sealAEAD.Seal(wire, nonce, encode(kind, payload), nil)
}

// Open decrypts a frame, and refuses one that has been seen before.
//
// A frame that does not open is never answered: silence is what makes probing
// this address indistinguishable from probing a closed one.
func (c *SecretCodec) Open(wire []byte) (Opened, error) {
	if len(wire) < nonceBytes+c.openAEAD.Overhead() {
		return Opened{}, ErrUnauthenticated
	}

	nonce := wire[:nonceBytes]
	plain, err := c.openAEAD.Open(nil, nonce, wire[nonceBytes:], nil)
	if err != nil {
		return Opened{}, ErrUnauthenticated
	}

	var prefix [prefixBytes]byte
	copy(prefix[:], nonce[:prefixBytes])
	counter := binary.BigEndian.Uint64(nonce[prefixBytes:])
	if !c.accept(prefix, counter) {
		return Opened{}, ErrReplayed
	}

	kind, payload, err := decode(plain)
	if err != nil {
		return Opened{}, err
	}
	return Opened{Kind: kind, Payload: payload, Sender: Sender(prefix)}, nil
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

// Open reports a zero sender: without authentication there is nothing to
// attribute a frame to.
func (plainCodec) Open(wire []byte) (Opened, error) {
	kind, payload, err := decode(wire)
	if err != nil {
		return Opened{}, err
	}
	return Opened{Kind: kind, Payload: payload}, nil
}
