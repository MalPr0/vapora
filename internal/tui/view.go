package tui

import (
	"fmt"
	"strings"
	"time"

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
	Scroll      int
	PeerTyping  bool
	Frame       int
	ClosedError string
	Link        LinkState
	RTT         time.Duration
	Silence     time.Duration
}

// LinkState mirrors what the session can infer about the path. The UI keeps its
// own copy of the vocabulary so it stays independent of the transport.
type LinkState int

const (
	LinkAlive LinkState = iota
	LinkStale
	LinkLost
)

const (
	// headerHeight fits the wordmark, which is seven pixels and therefore four
	// rows tall, plus a row of padding and the rule.
	headerHeight = 6
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
	drawBanner(screen, 2, 2, Gold)

	if state.Phase == PhaseChat {
		badge := fmt.Sprintf("● %s", state.Me)
		screen.Text(width-TextWidth(badge)-2, 2, badge, speakerColor(state.Me), Black)

		mark, label, color := linkBadge(state)
		peer := fmt.Sprintf("%s %s", mark, state.Peer)
		screen.Text(width-TextWidth(peer)-2, 3, peer, speakerColor(state.Peer), Black)
		screen.Text(width-TextWidth(label)-2, 4, label, color, Black)
	}
	rule(screen, headerHeight-1, width, Gold, Black)
}

// linkBadge turns silence into something readable. A healthy path says almost
// nothing, because the point of the probe is that it stays invisible until it
// stops being answered.
func linkBadge(state State) (mark, label string, color int) {
	switch state.Link {
	case LinkLost:
		// Blinking is the one thing a console can do that the eye catches
		// without moving, and a dead link has earned it.
		if state.Frame%8 < 4 {
			return "○", "LINK LOST", Red
		}
		return "○", "", Red
	case LinkStale:
		return "◐", fmt.Sprintf("no reply %ds", int(state.Silence.Seconds())), Gold
	default:
		if state.RTT > 0 {
			return "●", fmt.Sprintf("%dms", state.RTT.Milliseconds()), Gray
		}
		return "●", "", Gray
	}
}

func rule(screen *Screen, y, width, color, bg int) {
	for x := 0; x < width; x++ {
		screen.Set(x, y, '━', color, bg)
	}
}

func drawLoading(screen *Screen, state State, width, height int) {
	screen.Text(2, headerHeight+1, state.Status, White, Navy)

	// Everything stands on one ground line, which is what makes the scene read
	// as a level rather than as sprites floating at unrelated heights. The
	// ground sits as low as whatever has to go under it allows.
	ground := height - 3
	if state.Invite != "" {
		ground = height - 6
	}
	if ground < headerHeight+2 {
		ground = headerHeight + 2
	}

	for x := 2; x < width-2; x++ {
		screen.Set(x, ground, '▁', Green, Navy)
	}

	// Pixel rows are half a cell, so a sprite standing on the ground starts a
	// its own height above it.
	feet := ground * 2

	flag := flagOfHeight(clampInt((ground-headerHeight)*2, MinFlagHeight, MaxFlagHeight))
	flagX := width - 3 - flag.Width()
	flag.Draw(screen, flagX, feet-flag.Height())

	track := flagX - 4 - runnerFrames[0].Width()
	runnerX := 4 + int(float64(track)*clamp(state.Progress))
	hop := 0
	if state.Frame%6 < 3 {
		hop = -1
	}
	runnerFrames[state.Frame%len(runnerFrames)].Draw(screen, runnerX, feet-runnerFrames[0].Height()+hop)

	bar := progressBar(width-8, state.Progress)
	screen.Text(4, ground+2, bar, Lime, Navy)

	if state.Invite != "" {
		screen.Text(2, ground+4, "SEND THIS TO YOUR FRIEND", Gold, Navy)
		screen.Text(2, ground+5, state.Invite, White, Navy)
	}
}

func drawMessages(screen *Screen, state State, width, height int) {
	top := headerHeight
	area := height - footerHeight - top
	if area < 1 {
		return
	}

	lines := wrapMessages(state, width-4)
	visible, scroll, more := window(lines, area, state.Scroll)

	// A conversation reads upward from the input line, so the newest line sits
	// against the footer and an empty chat leaves the gap above, not below.
	offset := area - len(visible)
	for i, line := range visible {
		y := top + offset + i
		x := 2
		if line.speaker != "" {
			tag := fmt.Sprintf("%-10s", line.speaker)
			x += screen.Text(x, y, tag, line.color, Navy)
		} else {
			x += 10
		}
		screen.Text(x, y, line.body, line.bodyColor, Navy)
	}

	if scroll > 0 || more > 0 {
		marker := fmt.Sprintf("▲ %d more", more+scroll)
		if scroll > 0 {
			marker = fmt.Sprintf("▼ %d below  ▲ %d above", scroll, more)
		}
		screen.Text(width-TextWidth(marker)-2, top, marker, Gold, Navy)
	}
}

// window picks the visible slice and reports how many lines fell off each end,
// clamping a scroll offset that a resize may have left out of range.
func window(lines []renderedLine, area, scroll int) ([]renderedLine, int, int) {
	maxScroll := len(lines) - area
	if maxScroll < 0 {
		maxScroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if scroll < 0 {
		scroll = 0
	}

	end := len(lines) - scroll
	start := end - area
	if start < 0 {
		start = 0
	}
	return lines[start:end], scroll, start
}

// MaxScroll is how far back a state can be scrolled at a given size, which the
// input loop needs to stop the offset from running past the history.
func MaxScroll(state State, width, height int) int {
	area := height - footerHeight - headerHeight
	if area < 1 {
		return 0
	}
	if maximum := len(wrapMessages(state, width-4)) - area; maximum > 0 {
		return maximum
	}
	return 0
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

	hint := "enter sends  ·  pgup/pgdn scrolls  ·  !exit or ctrl+c quits"
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

func clampInt(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
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
