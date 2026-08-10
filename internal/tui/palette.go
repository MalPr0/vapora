// Package tui draws the chat with nothing but ANSI escapes, since the project
// carries no external dependencies. The renderer writes to an io.Writer and the
// terminal handling lives apart, so frames can be asserted on without a tty.
package tui

// Colours are 256 colour ANSI indices. The set is deliberately small and flat,
// the way a console with a fixed palette forced designs to be.
const (
	ColorDefault = -1

	Black    = 16
	Navy     = 18
	SkyBlue  = 39
	Cyan     = 51
	Green    = 40
	Lime     = 82
	Gold     = 214
	Orange   = 208
	Red      = 196
	Maroon   = 88
	Brown    = 130
	Skin     = 223
	White    = 231
	Silver   = 250
	Gray     = 244
	DarkGray = 238
	Magenta  = 201
)

// speakerColors are handed out per nickname so each side of the chat keeps one
// colour for the whole session.
var speakerColors = []int{Cyan, Lime, Gold, Magenta, SkyBlue, Orange}

func speakerColor(name string) int {
	var sum int
	for _, current := range name {
		sum += int(current)
	}
	return speakerColors[sum%len(speakerColors)]
}
