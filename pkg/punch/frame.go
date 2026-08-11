package punch

import "errors"

// Frame kinds exchanged over the punched UDP path. A one byte tag keeps the
// transport's own traffic distinguishable from whatever is being carried,
// without needing a parser.
//
// Everything below 0x40 belongs to the transport. Everything from 0x40 up is
// the caller's: this package neither reads nor interprets it, which is what
// lets the same channel carry a conversation, a file, or a game.
const (
	kindPunch byte = 0x01
	kindAck   byte = 0x02
	// 0x03 was a one way keepalive, retired in favour of the ping and pong
	// below. A frame from an older peer still counts as proof of life, since
	// anything that authenticates does.
	//
	// 0x04 and 0x05 were a chat line and a typing indicator, back when this
	// package knew what a conversation was. They are now one kind of payload
	// among any others, carried opaquely above kindData.
	//
	// kindPing and kindPong are the silent probe: neither reaches a caller, and
	// their only job is to make the absence of the peer measurable.
	kindPing byte = 0x06
	kindPong byte = 0x07
	// kindBye says the peer is leaving on purpose. Without it, quitting is
	// indistinguishable from a network that went quiet, and the other side
	// waits out the whole silence budget to find out.
	kindBye byte = 0x08

	// kindData carries the caller's bytes and is the whole application surface
	// of this package. Exported as AppKind.
	kindData = AppKind
)

// AppKind is the frame kind carrying a caller's bytes, and the only one this
// package does not interpret. It is exported so the boundary is legible from
// outside: everything below it belongs to the transport.
//
// A caller that needs several kinds of its own puts its tag inside the payload
// rather than alongside this one, which keeps the two numbering spaces from
// ever colliding.
const AppKind byte = 0x40

var errEmptyFrame = errors.New("punch: empty frame")

func encode(kind byte, payload string) []byte {
	frame := make([]byte, 0, len(payload)+1)
	frame = append(frame, kind)
	return append(frame, payload...)
}

func decode(frame []byte) (byte, string, error) {
	if len(frame) == 0 {
		return 0, "", errEmptyFrame
	}
	return frame[0], string(frame[1:]), nil
}
