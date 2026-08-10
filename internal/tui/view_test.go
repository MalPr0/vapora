package tui

import (
	"bytes"
	"strings"
	"testing"
)

// plain renders the screen as text, which is how a frame is asserted on without
// a terminal to look at.
func plain(screen *Screen) string {
	width, height := screen.Size()

	var out strings.Builder
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			cell := screen.At(x, y)
			if cell.Rune == 0 {
				continue
			}
			out.WriteRune(cell.Rune)
		}
		out.WriteString("\n")
	}
	return out.String()
}

func drawState(state State) *Screen {
	screen := NewScreen(80, 24)
	Draw(screen, state)
	return screen
}

func TestChatShowsSpeakersAndMessages(t *testing.T) {
	frame := plain(drawState(State{
		Phase: PhaseChat,
		Me:    "OTTER",
		Peer:  "BADGER",
		Messages: []Message{
			{Speaker: "BADGER", Body: "hola"},
			{Speaker: "OTTER", Body: "todo bien", Mine: true},
		},
		Input: "escribiendo",
	}))

	for _, want := range []string{"BADGER", "hola", "OTTER", "todo bien", "escribiendo", "enter sends"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("frame is missing %q:\n%s", want, frame)
		}
	}
}

func TestTypingIndicatorAppearsOnlyWhileTyping(t *testing.T) {
	quiet := plain(drawState(State{Phase: PhaseChat, Me: "OTTER", Peer: "BADGER"}))
	if strings.Contains(quiet, "is typing") {
		t.Fatalf("a quiet peer must not show the indicator:\n%s", quiet)
	}

	typing := plain(drawState(State{Phase: PhaseChat, Me: "OTTER", Peer: "BADGER", PeerTyping: true}))
	if !strings.Contains(typing, "BADGER is typing") {
		t.Fatalf("frame is missing the indicator:\n%s", typing)
	}
	if !strings.Contains(typing, string(upperHalf)) {
		t.Fatal("the runner sprite is not drawn next to the indicator")
	}
}

// The walk cycle has to actually change between frames, or the animation is a
// still image nobody notices is broken.
func TestRunnerAnimates(t *testing.T) {
	seen := map[string]bool{}
	for frame := 0; frame < len(runnerFrames); frame++ {
		screen := drawState(State{Phase: PhaseChat, Peer: "BADGER", PeerTyping: true, Frame: frame})
		seen[plain(screen)] = true
	}
	if len(seen) < 2 {
		t.Fatalf("the runner drew %d distinct frames", len(seen))
	}
}

func TestLoadingShowsProgressAndInvite(t *testing.T) {
	frame := plain(drawState(State{
		Phase:    PhaseLoading,
		Status:   "punching",
		Invite:   "vapora punch 203.0.113.7:41001/ABC",
		Progress: 0.5,
	}))

	if !strings.Contains(frame, "punching") || !strings.Contains(frame, "203.0.113.7:41001/ABC") {
		t.Fatalf("frame is missing the status or the invite:\n%s", frame)
	}
	if !strings.Contains(frame, "█") || !strings.Contains(frame, "░") {
		t.Fatal("the progress bar shows neither filled nor empty blocks")
	}
}

func TestLoadingProgressMovesTheRunner(t *testing.T) {
	start := plain(drawState(State{Phase: PhaseLoading, Progress: 0}))
	end := plain(drawState(State{Phase: PhaseLoading, Progress: 1}))
	if start == end {
		t.Fatal("the runner did not move between 0% and 100%")
	}
}

// A peer controls the text of its own messages, so nothing it sends may reach
// the terminal as a control sequence.
func TestMessagesAreSanitisedBeforeDrawing(t *testing.T) {
	screen := drawState(State{
		Phase:    PhaseChat,
		Peer:     "BADGER",
		Messages: []Message{{Speaker: "BADGER", Body: "hola\x1b[2Jchau"}},
	})

	width, height := screen.Size()
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if screen.At(x, y).Rune == 0x1B {
				t.Fatal("an escape byte reached the screen")
			}
		}
	}
}

func TestLongMessagesWrapUnderTheSpeaker(t *testing.T) {
	long := strings.Repeat("palabra ", 40)
	frame := plain(drawState(State{
		Phase:    PhaseChat,
		Peer:     "BADGER",
		Messages: []Message{{Speaker: "BADGER", Body: long}},
	}))

	for _, line := range strings.Split(frame, "\n") {
		if TextWidth(line) > 80 {
			t.Fatalf("a line overflowed the screen: %q", line)
		}
	}
	if strings.Count(frame, "palabra") < 2 {
		t.Fatal("the message did not wrap")
	}
}

// The courier is drawn in pixel space, where it is easy to make it taller than
// the band it lives in and have it paint over the hint or the input line.
func TestTypingSpriteStaysInsideItsBand(t *testing.T) {
	screen := drawState(State{Phase: PhaseChat, Peer: "BADGER", PeerTyping: true, Input: "hola", Frame: 1})
	width, height := screen.Size()

	for x := 0; x < width; x++ {
		if screen.At(x, height-1).Rune == upperHalf {
			t.Fatalf("the sprite reached the hint row at column %d", x)
		}
		if screen.At(x, height-2).Rune == upperHalf {
			t.Fatalf("the sprite reached the input row at column %d", x)
		}
	}

	hint := plain(screen)
	if !strings.Contains(hint, "enter sends") || !strings.Contains(hint, "> hola") {
		t.Fatalf("the sprite covered the footer text:\n%s", hint)
	}
}

// Sprites must not run off the right edge either.
func TestLoadingArtStaysOnScreen(t *testing.T) {
	for _, width := range []int{44, 60, 80, 120} {
		screen := NewScreen(width, 24)
		Draw(screen, State{Phase: PhaseLoading, Progress: 1, Frame: 1})

		for _, line := range strings.Split(plain(screen), "\n") {
			if TextWidth(line) > width {
				t.Fatalf("at width %d a line spilled to %d columns", width, TextWidth(line))
			}
		}
	}
}

func TestTooSmallTerminalSaysSo(t *testing.T) {
	screen := NewScreen(20, 5)
	Draw(screen, State{Phase: PhaseChat})
	if !strings.Contains(plain(screen), "too small") {
		t.Fatal("a cramped terminal must say so instead of drawing garbage")
	}
}

func TestFlushOnlyEmitsWhatChanged(t *testing.T) {
	screen := NewScreen(20, 3)

	var first bytes.Buffer
	Draw(screen, State{Phase: PhaseChat})
	if err := screen.Flush(&first); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if first.Len() == 0 {
		t.Fatal("the first frame emitted nothing")
	}

	var second bytes.Buffer
	if err := screen.Flush(&second); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if second.Len() != 0 {
		t.Fatalf("an unchanged frame emitted %d bytes", second.Len())
	}
}

func TestSpeakerColorIsStable(t *testing.T) {
	if speakerColor("OTTER") != speakerColor("OTTER") {
		t.Fatal("a speaker changed colour between calls")
	}
}
