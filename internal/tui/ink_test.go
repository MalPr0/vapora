package tui

import (
	"strings"
	"testing"

	"github.com/MalPr0/vapora/pkg/punch"
)

// The colour in a name is also the ink it is drawn in, so the two lists have to
// agree. They live in different packages on purpose: names belong to the
// protocol and pixels belong to the terminal.
func TestEveryNamedColourHasAnInk(t *testing.T) {
	seen := map[string]bool{}
	for attempt := 0; attempt < 20000; attempt++ {
		identity, err := punch.NewIdentity()
		if err != nil {
			t.Fatalf("cannot generate an identity: %v", err)
		}
		seen[identity.Public().Colour()] = true
	}

	if len(seen) < 60 {
		t.Fatalf("only %d colour words came up, the sample is too small to check", len(seen))
	}
	for colour := range seen {
		if _, named := inks[colour]; !named {
			t.Fatalf("%q names a participant but has no ink", colour)
		}
	}
}

// A name drawn in the colour of the background is a name nobody can read.
func TestNoInkVanishesIntoTheBackground(t *testing.T) {
	for colour, ink := range inks {
		if ink == Navy || ink == Black || ink == DarkGray {
			t.Fatalf("%q is drawn in %d, which is the background", colour, ink)
		}
		if ink < 17 {
			t.Fatalf("%q is drawn in %d, which is in the dim end of the palette", colour, ink)
		}
	}
}

// A line addressed to you is the one thing worth pulling out of a conversation
// that scrolls past, and it has to be findable without relying on colour alone.
func TestMentionsAreMarkedAndTinted(t *testing.T) {
	plainState := State{
		Phase: PhaseChat, Me: "CRIMSON OTTER", Peer: "JADE BADGER",
		Messages: []Message{{Speaker: "JADE BADGER", Body: "hola a todos"}},
	}
	if strings.Contains(plain(drawState(plainState)), MentionMark) {
		t.Fatal("an ordinary line was marked as a mention")
	}

	for _, body := range []string{
		"@CRIMSON OTTER mira esto",
		"@OTTER mira esto",
		"che @otter, mira",
	} {
		state := plainState
		state.Messages = []Message{{Speaker: "JADE BADGER", Body: body}}
		if !strings.Contains(plain(drawState(state)), MentionMark) {
			t.Fatalf("%q did not register as a mention", body)
		}
	}

	// Your own lines are not news to you, and neither is a mention of somebody
	// else.
	own := plainState
	own.Messages = []Message{{Speaker: "CRIMSON OTTER", Body: "@OTTER soy yo", Mine: true}}
	if strings.Contains(plain(drawState(own)), MentionMark) {
		t.Fatal("your own line was marked as a mention of you")
	}

	other := plainState
	other.Messages = []Message{{Speaker: "JADE BADGER", Body: "@BADGER hola"}}
	if strings.Contains(plain(drawState(other)), MentionMark) {
		t.Fatal("a mention of somebody else was marked as yours")
	}
}

// A short name can be the tail of a longer one, and the longer reading is what
// the writer meant.
func TestMentionsPreferTheLongerName(t *testing.T) {
	if !mentions("@CRIMSON OTTER hola", []string{"CRIMSON OTTER", "OTTER"}) {
		t.Fatal("the full name did not match")
	}
	if mentions("hola sin arrobas", []string{"OTTER"}) {
		t.Fatal("a line with no @ matched")
	}
	if mentions("correo@ejemplo.com", []string{"EJEMPLO"}) {
		t.Fatal("an address matched as a mention")
	}
}
