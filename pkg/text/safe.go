// Package text sanitises strings that arrived from the network before they
// reach a terminal.
package text

import (
	"strings"
	"unicode/utf8"
)

// MaxRendered caps how much of a remote line is printed, so one frame cannot
// flood the screen.
const MaxRendered = 1024

// Safe strips the sequences a terminal would act on instead of render, so a
// line that came from the network can never move the cursor, repaint the
// screen or reach the clipboard. Everything a human writes survives: accents,
// emoji and CJK pass through untouched.
func Safe(line string) string {
	return SafeLimit(line, MaxRendered)
}

func SafeLimit(line string, max int) string {
	var safe strings.Builder
	safe.Grow(len(line))

	// The marker has to fit inside the limit, not after it: anything this
	// produces has to satisfy Valid, or a line would be sanitised on the way
	// out and rejected on the way in.
	const marker = "..."
	body := max - len(marker)
	if body < 0 {
		body = 0
	}

	rendered := 0
	for _, current := range line {
		if rendered >= body {
			safe.WriteString(marker)
			break
		}
		replacement, keep := sanitise(current)
		if !keep {
			continue
		}
		safe.WriteRune(replacement)
		rendered++
	}
	return safe.String()
}

func sanitise(current rune) (rune, bool) {
	switch {
	case current == '\t':
		return ' ', true

	// C0 controls and DEL drive the terminal: escape sequences, carriage
	// returns that overwrite the line, bells.
	case current < 0x20 || current == 0x7F:
		return 0, false

	// C1 controls are a second escape alphabet that some terminals honour
	// even when the ESC form is filtered.
	case current >= 0x80 && current <= 0x9F:
		return 0, false

	// Bidi overrides reorder what the eye reads without changing the bytes,
	// which is enough to forge a convincing line.
	case current >= 0x202A && current <= 0x202E:
		return 0, false
	case current >= 0x2066 && current <= 0x2069:
		return 0, false

	case current == utf8.RuneError:
		return '�', true

	default:
		return current, true
	}
}

// Valid reports whether a string is the kind of text this protocol carries.
// The definition is exactly that sanitising it changes nothing: well formed
// UTF-8, no sequence a terminal would act on, and short enough to render.
//
// It exists so the wire can be checked rather than only the screen. Sanitising
// on the way out and validating on the way in means a frame carrying anything
// else is not a rendering problem to paper over, it is a peer that is not this
// program.
func Valid(value string) bool {
	return len([]rune(value)) <= MaxRendered && Safe(value) == value
}
