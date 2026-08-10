package tui

import (
	"bytes"
	"fmt"
	"io"
	"strings"
)

// Cell is one character position. Pixel art is drawn with half blocks, so a
// cell holding '▀' shows two pixels: the top one in Fg and the bottom in Bg.
type Cell struct {
	Rune rune
	Fg   int
	Bg   int
}

var blank = Cell{Rune: ' ', Fg: ColorDefault, Bg: ColorDefault}

// Screen is a double buffered grid. Flush emits only the cells that changed,
// which is what keeps an animation from flickering.
type Screen struct {
	width, height int
	current       []Cell
	previous      []Cell
	forceRedraw   bool
}

func NewScreen(width, height int) *Screen {
	screen := &Screen{}
	screen.Resize(width, height)
	return screen
}

func (s *Screen) Size() (int, int) { return s.width, s.height }

func (s *Screen) Resize(width, height int) {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	if width == s.width && height == s.height {
		return
	}

	s.width, s.height = width, height
	s.current = make([]Cell, width*height)
	s.previous = make([]Cell, width*height)
	s.forceRedraw = true
	s.Clear()
}

func (s *Screen) Clear() {
	for i := range s.current {
		s.current[i] = blank
	}
}

func (s *Screen) Fill(x, y, width, height int, bg int) {
	for row := y; row < y+height; row++ {
		for column := x; column < x+width; column++ {
			s.Set(column, row, ' ', ColorDefault, bg)
		}
	}
}

func (s *Screen) Set(x, y int, value rune, fg, bg int) {
	if x < 0 || y < 0 || x >= s.width || y >= s.height {
		return
	}
	s.current[y*s.width+x] = Cell{Rune: value, Fg: fg, Bg: bg}
}

func (s *Screen) At(x, y int) Cell {
	if x < 0 || y < 0 || x >= s.width || y >= s.height {
		return blank
	}
	return s.current[y*s.width+x]
}

// Text draws a string and returns how many columns it took, so callers can lay
// out the next piece without re-measuring.
func (s *Screen) Text(x, y int, value string, fg, bg int) int {
	column := x
	for _, current := range value {
		width := runeWidth(current)
		s.Set(column, y, current, fg, bg)
		// A wide rune owns the next cell too; leaving it blank stops the old
		// content from showing through its right half.
		for filler := 1; filler < width; filler++ {
			s.Set(column+filler, y, 0, fg, bg)
		}
		column += width
	}
	return column - x
}

// Flush writes the difference against the previous frame.
func (s *Screen) Flush(w io.Writer) error {
	var out bytes.Buffer
	fg, bg := ColorDefault-1, ColorDefault-1
	cursorX, cursorY := -1, -1

	for y := 0; y < s.height; y++ {
		for x := 0; x < s.width; x++ {
			index := y*s.width + x
			cell := s.current[index]
			if cell.Rune == 0 {
				continue // right half of a wide rune, already emitted
			}
			if !s.forceRedraw && s.previous[index] == cell {
				continue
			}

			if cursorY != y || cursorX != x {
				fmt.Fprintf(&out, "\x1b[%d;%dH", y+1, x+1)
				cursorX, cursorY = x, y
			}
			if cell.Fg != fg || cell.Bg != bg {
				out.WriteString(sgr(cell.Fg, cell.Bg))
				fg, bg = cell.Fg, cell.Bg
			}
			out.WriteRune(cell.Rune)
			cursorX += runeWidth(cell.Rune)
		}
	}

	if out.Len() > 0 {
		out.WriteString("\x1b[0m")
	}
	copy(s.previous, s.current)
	s.forceRedraw = false

	_, err := w.Write(out.Bytes())
	return err
}

func sgr(fg, bg int) string {
	var codes strings.Builder
	codes.WriteString("\x1b[0")
	if fg != ColorDefault {
		fmt.Fprintf(&codes, ";38;5;%d", fg)
	}
	if bg != ColorDefault {
		fmt.Fprintf(&codes, ";48;5;%d", bg)
	}
	codes.WriteString("m")
	return codes.String()
}

// runeWidth is the minimum needed to keep a chat line from drifting: the wide
// East Asian ranges and the emoji block take two columns, everything else one.
func runeWidth(value rune) int {
	switch {
	case value == 0:
		return 0
	case value < 0x1100:
		return 1
	case value >= 0x1100 && value <= 0x115F, // Hangul Jamo
		value >= 0x2E80 && value <= 0xA4CF, // CJK radicals through Yi
		value >= 0xAC00 && value <= 0xD7A3, // Hangul syllables
		value >= 0xF900 && value <= 0xFAFF, // CJK compatibility
		value >= 0xFE30 && value <= 0xFE6F, // CJK forms
		value >= 0xFF00 && value <= 0xFF60, // fullwidth forms
		value >= 0xFFE0 && value <= 0xFFE6,
		value >= 0x1F300 && value <= 0x1FAFF, // emoji
		value >= 0x20000 && value <= 0x3FFFD:
		return 2
	default:
		return 1
	}
}

// TextWidth is how many columns a string will occupy once drawn.
func TextWidth(value string) int {
	total := 0
	for _, current := range value {
		total += runeWidth(current)
	}
	return total
}
