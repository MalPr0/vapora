package tui

import (
	"fmt"
	"strings"

	"github.com/MalPr0/vapora/pkg/text"
)

// Phase is what the screen is showing.
type Phase int

const (
	PhaseLoading Phase = iota
	PhaseChat
	PhaseClosed
)

type Message struct {
	Speaker string
	Body    string
	Mine    bool
	System  bool
}

// State is everything the view draws. Keeping it a plain value is what lets the
// whole UI be asserted on without a terminal.
type State struct {
	Phase       Phase
	Status      string
	Invite      string
	Progress    float64
	Me, Peer    string
	Messages    []Message
	Input       string
	Cursor      int
	PeerTyping  bool
	Frame       int
	ClosedError string
}

const (
	headerHeight = 7
	// footerHeight covers the rule, the three rows the courier sprite needs,
	// the input line and the hint. A sprite taller than its band would paint
	// over the hint, which is exactly what it did before this was sized.
	footerHeight = 6
	minWidth     = 44
	minHeight    = headerHeight + footerHeight + 3
)

// Draw paints the whole state onto the screen.
func Draw(screen *Screen, state State) {
	width, height := screen.Size()
	screen.Clear()
	screen.Fill(0, 0, width, height, Navy)

	if width < minWidth || height < minHeight {
		screen.Text(0, 0, "terminal too small", Red, Navy)
		return
	}

	drawHeader(screen, state, width)
	if state.Phase == PhaseLoading {
		drawLoading(screen, state, width, height)
		return
	}
	drawMessages(screen, state, width, height)
	drawFooter(screen, state, width, height)
}

func drawHeader(screen *Screen, state State, width int) {
	screen.Fill(0, 0, width, headerHeight, Black)
	drawBanner(screen, 2, 1, Gold)

	if state.Phase == PhaseChat {
		badge := fmt.Sprintf("● %s", state.Me)
		screen.Text(width-TextWidth(badge)-2, 2, badge, speakerColor(state.Me), Black)
		peer := fmt.Sprintf("↔ %s", state.Peer)
		screen.Text(width-TextWidth(peer)-2, 3, peer, speakerColor(state.Peer), Black)
	}
	rule(screen, headerHeight-1, width, Gold, Black)
}

func rule(screen *Screen, y, width, color, bg int) {
	for x := 0; x < width; x++ {
		screen.Set(x, y, '━', color, bg)
	}
}

func drawLoading(screen *Screen, state State, width, height int) {
	middle := height / 2

	screen.Text(2, middle-4, state.Status, White, Navy)

	// The runner chases the flag as the punch proceeds, so the wait has a shape
	// instead of a spinner that says nothing about progress.
	flagX := width - 2 - flagSprite.Width()
	flagSprite.Draw(screen, flagX, (middle-1)*2)

	track := flagX - 4 - runnerFrames[0].Width()
	runnerX := 4 + int(float64(track)*clamp(state.Progress))
	hop := 0
	if state.Frame%6 < 3 {
		hop = -1
	}
	runnerFrames[state.Frame%len(runnerFrames)].Draw(screen, runnerX, (middle-1)*2+hop)

	for x := 2; x < width-2; x++ {
		screen.Set(x, middle+4, '▁', Green, Navy)
	}

	bar := progressBar(width-8, state.Progress)
	screen.Text(4, middle+6, bar, Lime, Navy)

	if state.Invite != "" {
		screen.Text(2, middle+8, "SEND THIS TO YOUR FRIEND", Gold, Navy)
		screen.Text(2, middle+9, state.Invite, White, Navy)
	}
}

func drawMessages(screen *Screen, state State, width, height int) {
	top := headerHeight
	bottom := height - footerHeight
	area := bottom - top
	if area < 1 {
		return
	}

	lines := wrapMessages(state, width-4)
	if len(lines) > area {
		lines = lines[len(lines)-area:]
	}

	for i, line := range lines {
		x := 2
		if line.speaker != "" {
			tag := fmt.Sprintf("%-10s", line.speaker)
			x += screen.Text(x, top+i, tag, line.color, Navy)
		} else {
			x += 10
		}
		screen.Text(x, top+i, line.body, line.bodyColor, Navy)
	}
}

type renderedLine struct {
	speaker   string
	body      string
	color     int
	bodyColor int
}

func wrapMessages(state State, width int) []renderedLine {
	body := width - 10
	if body < 8 {
		body = 8
	}

	var lines []renderedLine
	for _, message := range state.Messages {
		color := speakerColor(message.Speaker)
		bodyColor := White
		if message.System {
			color, bodyColor = Gray, Gray
		}

		for i, chunk := range wrap(text.Safe(message.Body), body) {
			line := renderedLine{body: chunk, color: color, bodyColor: bodyColor}
			if i == 0 {
				line.speaker = message.Speaker
			}
			lines = append(lines, line)
		}
	}
	return lines
}

func wrap(value string, width int) []string {
	if value == "" {
		return []string{""}
	}

	var lines []string
	var current strings.Builder
	used := 0

	for _, word := range strings.Fields(value) {
		size := TextWidth(word)
		switch {
		case used == 0:
			current.WriteString(word)
			used = size
		case used+1+size <= width:
			current.WriteString(" " + word)
			used += 1 + size
		default:
			lines = append(lines, current.String())
			current.Reset()
			current.WriteString(word)
			used = size
		}
	}
	if current.Len() > 0 || len(lines) == 0 {
		lines = append(lines, current.String())
	}
	return lines
}

func drawFooter(screen *Screen, state State, width, height int) {
	typingRow := height - footerHeight
	screen.Fill(0, typingRow, width, footerHeight, Black)
	rule(screen, typingRow, width, Gold, Black)

	if state.PeerTyping {
		label := fmt.Sprintf("%s is typing", state.Peer)
		screen.Text(12, typingRow+2, label+dots(state.Frame), speakerColor(state.Peer), Black)
		// The courier runs on the spot while the peer writes, and vanishes the
		// moment the line lands.
		courierFrames[state.Frame%len(courierFrames)].Draw(screen, 3, (typingRow+1)*2)
	} else if state.Phase == PhaseClosed {
		screen.Text(2, typingRow+2, state.ClosedError, Red, Black)
	}

	prompt := "> "
	screen.Text(2, height-2, prompt, Lime, Black)
	screen.Text(2+TextWidth(prompt), height-2, state.Input, White, Black)

	runes := []rune(state.Input)
	cursor := min(state.Cursor, len(runes))
	cursorX := 2 + TextWidth(prompt) + TextWidth(string(runes[:cursor]))
	drawCursor(screen, cursorX, height-2)

	hint := "enter sends  ·  ctrl+c quits"
	screen.Text(width-TextWidth(hint)-2, height-1, hint, DarkGray, Black)
}

// drawCursor inverts the cell it sits on instead of overwriting it, so a cursor
// in the middle of a line does not eat the character underneath.
func drawCursor(screen *Screen, x, y int) {
	cell := screen.At(x, y)
	if cell.Rune == ' ' || cell.Rune == 0 {
		screen.Set(x, y, '▋', Gold, cell.Bg)
		return
	}
	screen.Set(x, y, cell.Rune, cell.Bg, Gold)
}

func dots(frame int) string {
	return strings.Repeat(".", frame%4)
}

func clamp(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
