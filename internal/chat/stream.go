// Package chat is the demo that proves a UPnP port mapping carries traffic.
// It is not the product, but it is reachable from the internet, so it is
// authenticated and encrypted with the same session key the punch channel uses.
package chat

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/MalPr0/vapora/pkg/punch"
)

// maxFrame bounds what a peer can make this side allocate from a length it
// chose. A stream is not a datagram: the length prefix is attacker controlled
// and believing it is how a chat becomes a memory exhaustion primitive.
const maxFrame = 64 * 1024

// frameMessage is the only frame kind this demo carries. The codec treats the
// tag as opaque, so the value only has to be stable between the two ends.
const frameMessage byte = 0x01

var errFrameTooLarge = errors.New("chat: frame larger than the limit")

// stream is a length prefixed sequence of sealed frames over TCP.
type stream struct {
	conn   net.Conn
	codec  punch.Codec
	reader *bufio.Reader
}

func newStream(conn net.Conn, codec punch.Codec) *stream {
	return &stream{conn: conn, codec: codec, reader: bufio.NewReader(conn)}
}

func (s *stream) WriteLine(text string) error {
	sealed := s.codec.Seal(frameMessage, text)
	if len(sealed) > maxFrame {
		return fmt.Errorf("%w: %d bytes", errFrameTooLarge, len(sealed))
	}

	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, uint32(len(sealed)))
	if _, err := s.conn.Write(append(header, sealed...)); err != nil {
		return fmt.Errorf("chat: cannot write frame: %w", err)
	}
	return nil
}

// ReadLine returns the next authenticated line. A frame that does not
// authenticate ends the stream rather than being skipped: over TCP there is no
// resynchronising after a bad frame, and a peer without the key has no business
// being here at all.
func (s *stream) ReadLine() (string, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(s.reader, header); err != nil {
		return "", err
	}

	size := binary.BigEndian.Uint32(header)
	if size == 0 || size > maxFrame {
		return "", fmt.Errorf("%w: %d bytes announced", errFrameTooLarge, size)
	}

	sealed := make([]byte, size)
	if _, err := io.ReadFull(s.reader, sealed); err != nil {
		return "", err
	}

	kind, payload, err := s.codec.Open(sealed)
	if err != nil {
		return "", fmt.Errorf("chat: frame does not authenticate: %w", err)
	}
	if kind != frameMessage {
		return "", fmt.Errorf("chat: unexpected frame kind 0x%02x", kind)
	}
	return payload, nil
}

func (s *stream) Close() error { return s.conn.Close() }
