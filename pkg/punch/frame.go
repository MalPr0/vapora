package punch

import "errors"

// Frame kinds exchanged over the punched UDP path. A one byte tag keeps the
// handshake distinguishable from chat traffic without needing a parser.
const (
	kindPunch byte = 0x01
	kindAck   byte = 0x02
	// 0x03 was a one way keepalive, retired in favour of the ping and pong
	// below. A frame from an older peer still counts as proof of life, since
	// anything that authenticates does.
	kindMessage byte = 0x04
	// kindTyping carries "1" while the local user has an unsent line in
	// progress and "" when they stop, so the peer can show it.
	kindTyping byte = 0x05
	// kindPing and kindPong are the silent probe: neither reaches a UI, and
	// their only job is to make the absence of the peer measurable.
	kindPing byte = 0x06
	kindPong byte = 0x07
)

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
