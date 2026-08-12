// Package tui draws the chat with nothing but ANSI escapes, since the project
// carries no external dependencies. The renderer writes to an io.Writer and the
// terminal handling lives apart, so frames can be asserted on without a tty.
package tui

import "strings"

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

// inks map the colour word a participant is named after to what it is painted
// in, so the label and the ink say the same thing. Every entry is bright enough
// to read on the dark console; a name nobody can see is worse than a dull one.
var inks = map[string]int{
	"CRIMSON": 161, "SCARLET": 196, "CHERRY": 197, "RUBY": 160,
	"FLAME": 202, "EMBER": 203, "RUST": 166, "COPPER": 173,
	"BRONZE": 179, "AMBER": 214, "GOLD": 220, "SAFFRON": 208,
	"MARIGOLD": 215, "HONEY": 222, "BUTTER": 228, "LEMON": 227,
	"STRAW": 229, "CREAM": 230, "IVORY": 255, "LINEN": 254,
	"PEARL": 253, "SNOW": 231, "ASH": 249, "SILVER": 250,
	"STEEL": 111, "SLATE": 146, "KHAKI": 186, "SAND": 187,
	"TAN": 180, "CLAY": 174, "OCHRE": 178, "SIENNA": 172,
	"CORAL": 209, "SALMON": 210, "PEACH": 216, "APRICOT": 217,
	"ROSE": 211, "BLUSH": 218, "MAGENTA": 201, "ORCHID": 170,
	"LILAC": 183, "LAVENDER": 189, "PERIWINKLE": 147, "VIOLET": 141,
	"PLUM": 176, "INDIGO": 99, "COBALT": 75, "AZURE": 39,
	"SKY": 117, "FROST": 195, "CYAN": 51, "AQUA": 87,
	"TURQUOISE": 80, "SEAFOAM": 122, "TEAL": 44, "MINT": 121,
	"JADE": 43, "EMERALD": 42, "SAGE": 151, "MOSS": 108,
	"FERN": 114, "OLIVE": 149, "LIME": 118, "CHARTREUSE": 154,
}

// fallbackInks colour anything not named after a colour, such as the system
// rows or a peer from a build that names people some other way.
var fallbackInks = []int{Cyan, Lime, Gold, Magenta, SkyBlue, Orange}

// speakerColor reads the colour out of the name when there is one, so a member
// called CRIMSON OTTER is drawn crimson rather than in whatever a hash of the
// letters happened to land on.
func speakerColor(name string) int {
	for _, word := range strings.Fields(name) {
		if ink, named := inks[word]; named {
			return ink
		}
	}

	var sum int
	for _, current := range name {
		sum += int(current)
	}
	return fallbackInks[sum%len(fallbackInks)]
}
