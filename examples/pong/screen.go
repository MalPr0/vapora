package main

import (
	"fmt"
	"strings"
)

// The screen, drawn with half blocks so a pixel is square: a terminal cell is
// about twice as tall as it is wide, and a whole block would make the ball a
// rectangle and the court the wrong shape.
const (
	columns = 78
	rows    = 22 // in half-block pixels, so 11 terminal lines
)

type screen struct {
	pixels [rows][columns]bool
	frame  strings.Builder
}

func (s *screen) clear() {
	s.pixels = [rows][columns]bool{}
}

func (s *screen) set(x, y int) {
	if x < 0 || x >= columns || y < 0 || y >= rows {
		return
	}
	s.pixels[y][x] = true
}

func (s *screen) rect(x, y, width, height int) {
	for dy := 0; dy < height; dy++ {
		for dx := 0; dx < width; dx++ {
			s.set(x+dx, y+dy)
		}
	}
}

// render packs two pixel rows into each line of text.
func (s *screen) render(state State, mine, theirs string, status string) string {
	s.frame.Reset()

	// Home, then paint. Redrawing in place is what keeps it from scrolling.
	s.frame.WriteString("\x1b[H")
	s.frame.WriteString(fmt.Sprintf("\x1b[38;5;214m  %s %d   —   %d %s\x1b[0m\x1b[K\n",
		mine, state.LeftScore, state.RightScore, theirs))
	s.frame.WriteString("\x1b[38;5;240m  " + strings.Repeat("─", columns/2) + "\x1b[0m\x1b[K\n")

	for y := 0; y < rows; y += 2 {
		s.frame.WriteString("  \x1b[38;5;82m")
		for x := 0; x < columns; x++ {
			top, bottom := s.pixels[y][x], s.pixels[y+1][x]
			switch {
			case top && bottom:
				s.frame.WriteString("█")
			case top:
				s.frame.WriteString("▀")
			case bottom:
				s.frame.WriteString("▄")
			default:
				s.frame.WriteString(" ")
			}
		}
		s.frame.WriteString("\x1b[0m\x1b[K\n")
	}

	s.frame.WriteString("\x1b[38;5;240m  " + strings.Repeat("─", columns/2) + "\x1b[0m\x1b[K\n")

	// The status on the left, the credit pushed to the right edge of the court.
	gap := columns - len(status) - len("powered by vapora")
	if gap < 2 {
		gap = 2
	}
	s.frame.WriteString("  \x1b[38;5;250m" + status + "\x1b[0m" +
		strings.Repeat(" ", gap) + credit() + "\x1b[K\n")
	return s.frame.String()
}

// draw puts the world on the screen. Game units are a fixed field so both
// terminals agree about the game without agreeing about their own size.
func (s *screen) draw(state State) {
	s.clear()

	// The net.
	for y := 0; y < rows; y += 3 {
		s.set(columns/2, y)
	}

	s.paddle(scaleX(paddleInset), state.LeftY)
	s.paddle(scaleX(fieldWidth-paddleInset), state.RightY)

	// One column wide and two pixel rows tall, which is one terminal cell and
	// therefore square on screen.
	s.rect(scaleX(int(state.BallX)), scaleY(int(state.BallY)), 1, 2)
}

func (s *screen) paddle(x int, centre uint16) {
	height := paddleHeight * rows / fieldHeight
	top := scaleY(int(centre)) - height/2
	s.rect(x, top, 1, height)
}

func scaleX(value int) int { return value * (columns - 1) / fieldWidth }
func scaleY(value int) int { return value * (rows - 1) / fieldHeight }

const (
	enterAlternate = "\x1b[?1049h\x1b[?25l"
	leaveAlternate = "\x1b[?25h\x1b[?1049l"
)
