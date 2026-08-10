package punch

import (
	"fmt"
	"io"

	"github.com/MalPr0/vapora/pkg/text"
)

// Observer receives what happens on an open session. A chat UI implements it to
// drive its own rendering instead of having the session write to a stream.
type Observer interface {
	// Message is a line the peer sent. It has not been sanitised yet: what a
	// terminal must not act on depends on how the consumer renders it.
	Message(payload string)
	// Typing reports whether the peer currently has a line in progress.
	Typing(active bool)
}

// writerObserver is the default: it prints to the session's writer, which keeps
// a session usable with no UI at all.
type writerObserver struct {
	output io.Writer
}

func (o writerObserver) Message(payload string) {
	fmt.Fprintf(o.output, "<peer> %s\n", text.Safe(payload))
}

func (o writerObserver) Typing(bool) {}
