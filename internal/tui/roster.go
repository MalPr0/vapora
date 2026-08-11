package tui

import (
	"fmt"
	"time"
)

// Participant is one row of the roster the header draws.
type Participant struct {
	Name    string
	Link    LinkState
	RTT     time.Duration
	Silence time.Duration
	Typing  bool
}

// gutterFor sizes the column the speaker names sit in. A fixed width either
// truncates the long names a crowded room produces or wastes a third of the
// screen in a quiet one.
func gutterFor(state State) int {
	widest := len(state.Me)
	for _, member := range state.Members {
		if width := TextWidth(member.Name); width > widest {
			widest = width
		}
	}

	return clampInt(widest+2, minGutter, maxGutter)
}

const (
	minGutter = 10
	maxGutter = 16
)

// headerRows is how tall the header has to be to list everyone. It stops being
// a constant the moment there is more than one other person, and every place
// that measured the conversation against the old constant has to ask here.
func headerRows(state State, height int) int {
	rows := baseHeaderRows
	if extra := len(state.Members) - 1; extra > 0 {
		rows += extra
	}

	// The conversation is the point; the roster gives way before it does.
	if most := height - footerHeight - minConversation; rows > most {
		rows = most
	}
	if rows < baseHeaderRows {
		rows = baseHeaderRows
	}
	return rows
}

const (
	// baseHeaderRows fits the wordmark, which is four rows, plus padding, one
	// roster line and the rule.
	baseHeaderRows  = 6
	minConversation = 4
)

// drawRoster lists who is present down the right hand side, with what is known
// about the path to each of them. A healthy path says almost nothing: the point
// of the strip is the one row that stops looking healthy.
func drawRoster(screen *Screen, state State, width, rows int) {
	lines := rows - 2
	if lines < 1 {
		return
	}

	shown := state.Members
	overflow := 0
	if len(shown) > lines {
		overflow = len(shown) - (lines - 1)
		shown = shown[:lines-1]
	}

	for i, member := range shown {
		mark, detail, colour := memberBadge(member, state.Frame)
		label := fmt.Sprintf("%s %s", mark, member.Name)

		// The row reads left to right like a sentence: who, then how they are.
		// The detail is what changes, so it is the part anchored to the edge.
		right := width - 2
		if detail != "" {
			screen.Text(right-TextWidth(detail), 2+i, detail, colour, Black)
			right -= TextWidth(detail) + 2
		}
		screen.Text(right-TextWidth(label), 2+i, label, speakerColor(member.Name), Black)
	}

	if overflow > 0 {
		more := fmt.Sprintf("+%d more", overflow)
		screen.Text(width-TextWidth(more)-2, 2+len(shown), more, Gray, Black)
	}
}

// memberBadge turns silence into something readable for one participant.
func memberBadge(member Participant, frame int) (mark, detail string, colour int) {
	switch member.Link {
	case LinkLost:
		// Blinking is the one thing a console can do that the eye catches
		// without moving, and a dead path has earned it.
		if frame%8 < 4 {
			return "○", "LINK LOST", Red
		}
		return "○", "", Red
	case LinkStale:
		return "◐", fmt.Sprintf("no reply %ds", int(member.Silence.Seconds())), Gold
	default:
		// A round trip that rounds to zero is a number nobody can act on, and
		// on the loopback that is every measurement.
		if member.RTT >= time.Millisecond {
			return "●", fmt.Sprintf("%dms", member.RTT.Milliseconds()), Gray
		}
		return "●", "", Gray
	}
}

// typingLine says who is writing. One name reads better than a count, two still
// do, and past that a count is the only thing that fits.
func typingLine(state State) (string, int) {
	var names []string
	for _, member := range state.Members {
		if member.Typing {
			names = append(names, member.Name)
		}
	}

	switch len(names) {
	case 0:
		return "", ColorDefault
	case 1:
		return names[0] + " is typing", speakerColor(names[0])
	case 2:
		return names[0] + " and " + names[1] + " are typing", speakerColor(names[0])
	default:
		return fmt.Sprintf("%d people are typing", len(names)), Gold
	}
}

// names is everyone this side answers to, which is what a mention is matched
// against.
func (s State) names() []string {
	if s.Me == "" {
		return nil
	}
	return []string{s.Me}
}
