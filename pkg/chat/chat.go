// Package chat is a conversation built on top of a punched channel.
//
// It exists so that the transport does not have to know what a conversation is.
// Everything here — that a payload is a line of text, that somebody is typing,
// that a participant has a name you can say out loud — is this package's
// opinion, and none of it reaches the wire below.
//
// The shape to copy if you are building something else: take a
// *punch.Session or *punch.Room, define your own tags inside the payload, and
// leave the transport alone.
package chat

import (
	"github.com/MalPr0/vapora/pkg/text"
)

// Tags are this package's own numbering, carried inside the payload the
// transport moves. They are independent of the transport's frame kinds, so the
// two can never collide and neither constrains the other.
const (
	tagLine   byte = 0x01
	tagTyping byte = 0x02
)

// MaxLine is the longest line this will send or accept. A datagram that has to
// survive a home router is not the place for an essay, and a length nobody
// bounds is a length somebody else chooses.
const MaxLine = 2048

func encode(tag byte, payload string) []byte {
	frame := make([]byte, 0, len(payload)+1)
	frame = append(frame, tag)
	return append(frame, payload...)
}

// decode splits a payload, and refuses anything that is not one of ours. What
// arrives is from the network: it is checked, not trusted.
func decode(payload []byte) (tag byte, body string, ok bool) {
	if len(payload) == 0 || len(payload) > MaxLine+1 {
		return 0, "", false
	}
	return payload[0], string(payload[1:]), true
}

// line reads a chat line, and reports whether it is one that may be shown.
//
// Only text crosses a conversation. A payload carrying anything else is not
// this program on the other end, so it is dropped rather than cleaned up and
// displayed — cleaning it up would mean showing something nobody sent.
func line(body string) (string, bool) {
	if !text.Valid(body) {
		return "", false
	}
	return body, true
}

func typing(body string) bool {
	return len(body) > 0 && body[0] == '1'
}

func typingPayload(active bool) []byte {
	flag := byte('0')
	if active {
		flag = '1'
	}
	return encode(tagTyping, string([]byte{flag}))
}

// safeLine prepares a line for sending. Sanitising here rather than on receipt
// is deliberate: it means this program never emits a sequence a terminal would
// act on, whatever it was handed.
func safeLine(line string) string {
	line = text.Safe(line)
	if len(line) > MaxLine {
		line = line[:MaxLine]
	}
	return line
}
