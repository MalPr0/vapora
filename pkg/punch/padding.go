package punch

import (
	"crypto/rand"
	"encoding/binary"
)

// maxPadding bounds the filler on a control frame. The range is what matters,
// not the size: a wide spread is what stops a length from naming a frame.
const maxPadding = 255

// AEAD hides what a frame says but not how long it is, and a control frame with
// nothing to carry has a length nobody else produces. A ping would be a fixed
// size arriving on a fixed cadence, which is a heartbeat visible to anyone
// counting bytes on the wire, and knowing which packets are heartbeats is
// enough to fingerprint the protocol and to tell an idle session from a
// conversation. Random filler puts control frames inside the same size spread
// as the messages they travel among.
func padded(prefix []byte) string {
	var length [2]byte
	if _, err := rand.Read(length[:]); err != nil {
		return string(prefix)
	}

	filler := make([]byte, int(binary.BigEndian.Uint16(length[:]))%(maxPadding+1))
	if _, err := rand.Read(filler); err != nil {
		return string(prefix)
	}
	return string(append(prefix, filler...))
}

// pad is the filler-only form, for frames whose whole content is their kind.
func pad() string {
	return padded(nil)
}
