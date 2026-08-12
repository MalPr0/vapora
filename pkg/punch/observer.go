package punch

import (
	"encoding/hex"
	"fmt"
	"io"
)

// Observer receives what happens on an open session.
//
// It carries bytes, not lines: this package moves whatever it is handed and has
// no opinion about what the bytes mean. A chat implements this to turn them
// back into a conversation; anything else implements it to turn them back into
// whatever it is moving.
type Observer interface {
	// Data is a payload the peer sent, exactly as they sent it. Nothing has
	// been validated or sanitised: what is safe depends entirely on what the
	// consumer is going to do with it.
	Data(payload []byte)
}

// ObserverFunc adapts a function, which is all most callers need.
type ObserverFunc func(payload []byte)

// Data calls the function.
func (f ObserverFunc) Data(payload []byte) { f(payload) }

// writerObserver is the default: it prints to the session's writer, which keeps
// a session usable with no caller attached at all.
//
// It prints hex rather than the bytes themselves, because a payload from the
// network is not text and must never be handed to a terminal unexamined.
type writerObserver struct {
	output io.Writer
}

func (o writerObserver) Data(payload []byte) {
	if o.output == nil {
		return
	}
	fmt.Fprintf(o.output, "<peer> %d bytes: %s\n", len(payload), hex.EncodeToString(payload))
}
