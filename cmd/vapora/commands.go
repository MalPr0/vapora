package main

import (
	"strings"

	"github.com/MalPr0/vapora/pkg/punch"
)

// ExitCommand is the line that ends a session from the keyboard. It is checked
// before a line becomes a message, so it never reaches the peer as text.
const ExitCommand = "!exit"

// isExit reports whether a typed line is the quit command. The comparison is
// exact after trimming: a message that merely mentions !exit is a message.
func isExit(line string) bool {
	return strings.TrimSpace(line) == ExitCommand
}

// leave says goodbye and returns. Announcing it is what keeps the other side
// from waiting out the whole silence budget to discover something that was
// already decided.
func leave(session *punch.Session) {
	session.Goodbye()
}
