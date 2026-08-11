package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// only is the two way case: one other person, which is what most of the view
// tests are about.
func only(name string) []Participant {
	return []Participant{{Name: name, Link: LinkAlive}}
}

func typingNow(name string) []Participant {
	return []Participant{{Name: name, Link: LinkAlive, Typing: true}}
}

func lostAt(silence time.Duration) []Participant {
	return []Participant{{Name: "BADGER", Link: LinkLost, Silence: silence}}
}

func crowd(count int) []Participant {
	members := make([]Participant, count)
	for i := range members {
		members[i] = Participant{Name: fmt.Sprintf("MEMBER-%02d", i), Link: LinkAlive}
	}
	return members
}

// The header is a constant for two people and grows with the room. Everything
// that measured the conversation against the old constant has to ask, and this
// is the test that catches the one that did not.
func TestHeaderGrowsWithTheRoom(t *testing.T) {
	two := headerRows(State{Members: only("BADGER")}, 24)
	if two != baseHeaderRows {
		t.Fatalf("two people took %d header rows, want %d", two, baseHeaderRows)
	}

	last := two
	for count := 2; count <= 7; count++ {
		rows := headerRows(State{Members: crowd(count)}, 24)
		if rows != last+1 {
			t.Fatalf("%d members took %d rows, want %d", count, rows, last+1)
		}
		last = rows
	}
}

// The conversation is the point. A full room on a short terminal gives up
// roster lines rather than the last thing anyone said.
func TestConversationKeepsItsFloor(t *testing.T) {
	// Below minHeight the view refuses to draw at all, so the floor is a claim
	// about the sizes it does accept.
	for _, height := range []int{minHeight, 20, 24, 40} {
		state := State{Phase: PhaseChat, Me: "OTTER", Members: crowd(8)}
		rows := headerRows(state, height)

		if left := height - footerHeight - rows; left < minConversation {
			t.Fatalf("at %d rows tall the conversation got %d lines, want at least %d",
				height, left, minConversation)
		}
		if rows < baseHeaderRows {
			t.Fatalf("at %d rows tall the header shrank to %d, below the wordmark", height, rows)
		}
	}
}

// A roster that does not fit says how much it is hiding. Silently dropping the
// tail reads as a smaller room than the one you are in.
func TestRosterReportsWhatItCannotShow(t *testing.T) {
	state := State{Phase: PhaseChat, Me: "OTTER", Members: crowd(8)}
	frame := plain(drawState(state))

	if !strings.Contains(frame, "MEMBER-00") {
		t.Fatalf("the roster did not list anyone:\n%s", frame)
	}
	if strings.Contains(frame, "MEMBER-07") {
		return // it all fit, so there is nothing to report
	}
	if !strings.Contains(frame, "more") {
		t.Fatalf("the roster hid members without saying so:\n%s", frame)
	}
}

// One name reads better than a count, and two still do. Past that a count is
// the only thing that fits, and it must not claim a single person is "people".
func TestTypingLineNamesWhoItCan(t *testing.T) {
	members := crowd(4)

	if line, _ := typingLine(State{Members: members}); line != "" {
		t.Fatalf("a quiet room claimed %q", line)
	}

	members[0].Typing = true
	line, _ := typingLine(State{Members: members})
	if line != "MEMBER-00 is typing" {
		t.Fatalf("one typist read as %q", line)
	}

	members[2].Typing = true
	line, _ = typingLine(State{Members: members})
	if !strings.Contains(line, "MEMBER-00") || !strings.Contains(line, "MEMBER-02") ||
		!strings.Contains(line, "are typing") {
		t.Fatalf("two typists read as %q", line)
	}

	members[3].Typing = true
	line, _ = typingLine(State{Members: members})
	if line != "3 people are typing" {
		t.Fatalf("three typists read as %q", line)
	}
}

// The gutter follows the longest name present. A fixed width either truncates
// the names a crowded room produces or wastes a third of a quiet screen.
func TestGutterFollowsTheLongestName(t *testing.T) {
	narrow := gutterFor(State{Me: "OTTER", Members: only("BADGER")}, 80)
	if narrow != minGutter {
		t.Fatalf("short names took a %d column gutter, want %d", narrow, minGutter)
	}

	wide := gutterFor(State{Me: "OTTER", Members: []Participant{
		{Name: "SWIFT CRIMSON OTTER"},
	}}, 80)
	if wide != maxGutter {
		t.Fatalf("a long name took a %d column gutter, want the %d cap", wide, maxGutter)
	}

	// My own name sits in the same column as everyone else's.
	mine := gutterFor(State{Me: "SWIFT CRIMSON OTTER", Members: only("A")}, 80)
	if mine != maxGutter {
		t.Fatalf("my own name did not widen the gutter: %d", mine)
	}
}

// Health is per participant: one dead path must not make the whole room look
// dead, and it must not hide behind the ones that are fine.
func TestOneLostPathDoesNotCondemnTheRoom(t *testing.T) {
	frame := plain(drawState(State{Phase: PhaseChat, Me: "OTTER", Members: []Participant{
		{Name: "BADGER", Link: LinkAlive, RTT: 21 * time.Millisecond},
		{Name: "HERON", Link: LinkLost},
		{Name: "MARTEN", Link: LinkStale, Silence: 9 * time.Second},
	}}))

	for _, want := range []string{"BADGER", "21ms", "HERON", "LINK LOST", "MARTEN", "no reply 9s"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("the roster is missing %q:\n%s", want, frame)
		}
	}
}

// The gutter is a promise, not a suggestion: the body of every line starts in
// the same column whoever is speaking. A three word nickname is thirty
// characters, wider than the gutter, and padding with %-*s widens instead of
// cutting — which shifts that speaker's text right and pushes its tail off the
// edge, where Screen.Text quietly drops it.
func TestLongNamesDoNotBreakTheColumn(t *testing.T) {
	const long = "RESTLESS PERIWINKLE SALAMANDER"
	const mark = "XMARKX"

	state := State{
		Phase: PhaseChat,
		Me:    "OTTER",
		Members: []Participant{
			{Name: long, Link: LinkAlive},
			{Name: "JADE HERON", Link: LinkAlive},
		},
		Messages: []Message{
			{Speaker: "JADE HERON", Body: mark},
			{Speaker: long, Body: mark},
		},
	}

	for _, width := range []int{minWidth, 80, 120} {
		screen := NewScreen(width, 30)
		Draw(screen, state)

		var columns []int
		for _, line := range strings.Split(plain(screen), "\n") {
			// In runes, not bytes: a truncated name ends in a three byte
			// ellipsis, and measuring in bytes is the same mistake that made
			// the first wordmark unreadable.
			if at := strings.Index(line, mark); at >= 0 {
				columns = append(columns, len([]rune(line[:at])))
			}
		}

		if len(columns) != 2 {
			t.Fatalf("at %d columns %d of 2 messages survived:\n%s", width, len(columns), plain(screen))
		}
		if columns[0] != columns[1] {
			t.Fatalf("at %d columns the bodies start at %d and %d, so a long name broke the column",
				width, columns[0], columns[1])
		}

		gutter := gutterFor(state, width)
		if gutter > width/3 {
			t.Fatalf("at %d columns the gutter took %d, more than a third", width, gutter)
		}
	}
}

// Nothing is lost off the right edge either: a long name must not eat into the
// width the wrapping already committed to.
func TestALongNameLosesNoText(t *testing.T) {
	body := strings.TrimSpace(strings.Repeat("palabra ", 40))
	state := State{
		Phase:    PhaseChat,
		Me:       "OTTER",
		Members:  []Participant{{Name: "RESTLESS PERIWINKLE SALAMANDER", Link: LinkAlive}},
		Messages: []Message{{Speaker: "RESTLESS PERIWINKLE SALAMANDER", Body: body}},
	}

	screen := NewScreen(80, 30)
	Draw(screen, state)

	if got := strings.Count(plain(screen), "palabra"); got != 40 {
		t.Fatalf("%d of 40 words reached the screen:\n%s", got, plain(screen))
	}
}

// A truncated name has to look truncated. Cutting "RESTLESS PERIWINKLE
// SALAMANDER" down to a clean "RESTLESS PERIWINKLE" would read as a different
// participant, and names are the one thing in a room that cannot be ambiguous.
func TestATruncatedNameSaysSo(t *testing.T) {
	got := fitName("RESTLESS PERIWINKLE SALAMANDER", 12)
	if TextWidth(got) != 12 {
		t.Fatalf("%q is %d wide, want 12", got, TextWidth(got))
	}
	if !strings.HasSuffix(strings.TrimRight(got, " "), "…") {
		t.Fatalf("%q does not show that it was cut", got)
	}
	// A cut name still keeps clear of the text, or the two run together.
	if !strings.HasSuffix(got, " ") {
		t.Fatalf("%q leaves nothing between the name and the body", got)
	}

	// A name that fits is left exactly as it is, padded to the column.
	if got := fitName("OTTER", 12); got != "OTTER       " {
		t.Fatalf("a short name came back as %q", got)
	}
}
