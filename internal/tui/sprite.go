package tui

import "strings"

// upperHalf paints two pixels in one cell: the foreground is the top pixel and
// the background the bottom one. It is what gives a terminal square pixels
// instead of the tall cells a plain character would produce.
const upperHalf = '▀'

// Sprite is pixel art. Each rune of a row is a palette key, and a space is
// transparent, so a sprite can be drawn over any background.
type Sprite struct {
	Rows    []string
	Palette map[rune]int
}

func (s Sprite) Width() int {
	width := 0
	for _, row := range s.Rows {
		if length := len([]rune(row)); length > width {
			width = length
		}
	}
	return width
}

func (s Sprite) Height() int { return len(s.Rows) }

func (s Sprite) pixel(x, y int) (int, bool) {
	if y < 0 || y >= len(s.Rows) {
		return 0, false
	}
	row := []rune(s.Rows[y])
	if x < 0 || x >= len(row) {
		return 0, false
	}
	color, ok := s.Palette[row[x]]
	return color, ok
}

// Draw paints the sprite with its top left pixel at (x, y) in pixel space.
// Vertical pixel coordinates are half a cell, so an odd y shifts the sprite
// half a character down, which is what makes a jump look smooth.
func (s Sprite) Draw(screen *Screen, x, y int) {
	for py := 0; py < s.Height(); py++ {
		for px := 0; px < s.Width(); px++ {
			color, ok := s.pixel(px, py)
			if !ok {
				continue
			}

			column := x + px
			row := (y + py) / 2
			cell := screen.At(column, row)

			top, bottom := cell.Fg, cell.Bg
			if cell.Rune != upperHalf {
				// The cell holds ordinary text, so both of its pixels take the
				// background it was drawn on.
				top, bottom = cell.Bg, cell.Bg
			}
			if (y+py)%2 == 0 {
				top = color
			} else {
				bottom = color
			}
			screen.Set(column, row, upperHalf, top, bottom)
		}
	}
}

var runnerPalette = map[rune]int{
	'R': Red,
	'S': Skin,
	'B': Brown,
	'W': White,
	'D': Maroon,
}

// runnerFrames is a three step walk cycle for the little courier that carries
// the "typing" state. It is drawn from scratch rather than borrowed: eight by
// ten pixels is enough to read as a running figure at this size.
var runnerFrames = []Sprite{
	{
		Palette: runnerPalette,
		Rows: []string{
			"  RRRR  ",
			" RRRRRR ",
			" SSDS   ",
			" SSSSS  ",
			"  RRR   ",
			" WRRRW  ",
			" WRRRW  ",
			"  R R   ",
			"  B B   ",
			" BB  BB ",
		},
	},
	{
		Palette: runnerPalette,
		Rows: []string{
			"  RRRR  ",
			" RRRRRR ",
			" SSDS   ",
			" SSSSS  ",
			"  RRR   ",
			" WRRRW  ",
			"  RRR W ",
			"  RR    ",
			"  BB    ",
			"  BBB   ",
		},
	},
	{
		Palette: runnerPalette,
		Rows: []string{
			"  RRRR  ",
			" RRRRRR ",
			" SSDS   ",
			" SSSSS  ",
			"  RRR   ",
			"WRRRRW  ",
			" WRRR   ",
			"  R R   ",
			" B   B  ",
			"BB    BB",
		},
	},
}

// courierFrames are the same walk cycle at six pixels tall, which is what fits
// in the typing band without stealing rows from the conversation.
var courierFrames = []Sprite{
	{
		Palette: runnerPalette,
		Rows: []string{
			" RRR ",
			" SSD ",
			"WRRRW",
			" RRR ",
			" B B ",
			"BB BB",
		},
	},
	{
		Palette: runnerPalette,
		Rows: []string{
			" RRR ",
			" SSD ",
			" RRRW",
			" RRR ",
			"  BB ",
			"  BBB",
		},
	},
	{
		Palette: runnerPalette,
		Rows: []string{
			" RRR ",
			" SSD ",
			"WRRR ",
			" RRR ",
			" B B ",
			"B   BB",
		},
	},
}

var flagSprite = Sprite{
	Palette: map[rune]int{'P': Silver, 'F': Green, 'K': Lime},
	Rows: []string{
		"PFFFF ",
		"PFFKK ",
		"PFFFF ",
		"PKK   ",
		"P     ",
		"P     ",
		"P     ",
		"P     ",
		"P     ",
		"PPP   ",
	},
}

// glyphs is a five by seven font. A cell is about twice as tall as it is wide,
// so a wordmark built from whole block characters comes out stretched and
// unreadable; drawing it as half block pixels like the sprites keeps the
// letterforms square.
var glyphs = map[rune][]string{
	'V': {
		"X   X",
		"X   X",
		"X   X",
		"X   X",
		"X   X",
		" X X ",
		"  X  ",
	},
	'A': {
		"  X  ",
		" X X ",
		"X   X",
		"X   X",
		"XXXXX",
		"X   X",
		"X   X",
	},
	'P': {
		"XXXX ",
		"X   X",
		"X   X",
		"XXXX ",
		"X    ",
		"X    ",
		"X    ",
	},
	'O': {
		" XXX ",
		"X   X",
		"X   X",
		"X   X",
		"X   X",
		"X   X",
		" XXX ",
	},
	'R': {
		"XXXX ",
		"X   X",
		"X   X",
		"XXXX ",
		"X  X ",
		"X   X",
		"X   X",
	},
}

const wordmark = "VAPORA"

// glyphHeight is in pixels; on screen it takes half as many rows.
const glyphHeight = 7

// BannerWidth is how many columns the wordmark needs, so a caller can decide
// whether it fits before drawing it.
func BannerWidth() int {
	return len(wordmark)*6 - 1
}

func drawBanner(screen *Screen, x, y int, color int) {
	cursor := x
	for _, letter := range wordmark {
		rows, ok := glyphs[letter]
		if !ok {
			cursor += 6
			continue
		}
		Sprite{Rows: rows, Palette: map[rune]int{'X': color}}.Draw(screen, cursor, y)
		cursor += 6
	}
}

// progressBar is the retro loading meter: filled blocks against a dim track.
func progressBar(width int, ratio float64) string {
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	filled := int(float64(width) * ratio)
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}
