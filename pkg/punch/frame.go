package punch

import "errors"

// Frame kinds exchanged over the punched UDP path. A one byte tag keeps the
// handshake distinguishable from chat traffic without needing a parser.
const (
	kindPunch     byte = 0x01
	kindAck       byte = 0x02
	kindKeepalive byte = 0x03
	kindMessage   byte = 0x04
	// kindTyping carries "1" while the local user has an unsent line in
	// progress and "" when they stop, so the peer can show it.
	kindTyping byte = 0x05
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
