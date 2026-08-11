package main

import (
	"fmt"
	"strings"
)

// The wordmarks, in the same five-by-seven font the chat draws, packed two
// pixel rows to a line with half blocks so the letters come out square: a
// terminal cell is about twice as tall as it is wide.
var glyphs = map[rune][]string{
	'P': {"XXXX ", "X   X", "X   X", "XXXX ", "X    ", "X    ", "X    "},
	'O': {" XXX ", "X   X", "X   X", "X   X", "X   X", "X   X", " XXX "},
	'N': {"X   X", "XX  X", "XX  X", "X X X", "X  XX", "X  XX", "X   X"},
	'G': {" XXX ", "X   X", "X    ", "X  XX", "X   X", "X   X", " XXX "},
	'V': {"X   X", "X   X", "X   X", "X   X", "X   X", " X X ", "  X  "},
	'A': {"  X  ", " X X ", "X   X", "X   X", "XXXXX", "X   X", "X   X"},
	'R': {"XXXX ", "X   X", "X   X", "XXXX ", "X  X ", "X   X", "X   X"},
}

// wordmark renders a word as four lines of half-block pixels.
func wordmark(word string) []string {
	pixels := make([]string, 7)
	for _, letter := range word {
		rows, known := glyphs[letter]
		if !known {
			continue
		}
		for i, row := range rows {
			pixels[i] += row + " "
		}
	}

	var lines []string
	for row := 0; row < 7; row += 2 {
		var line strings.Builder
		for column := 0; column < len(pixels[row]); column++ {
			top := pixels[row][column] == 'X'
			bottom := row+1 < 7 && pixels[row+1][column] == 'X'
			switch {
			case top && bottom:
				line.WriteString("█")
			case top:
				line.WriteString("▀")
			case bottom:
				line.WriteString("▄")
			default:
				line.WriteString(" ")
			}
		}
		lines = append(lines, line.String())
	}
	return lines
}

// splash is what you see before the path opens, which on a bad night is the
// screen you look at for three minutes.
func splash() string {
	var out strings.Builder

	out.WriteString("\n")
	for _, line := range wordmark("PONG") {
		out.WriteString("      \x1b[38;5;82m" + line + "\x1b[0m\n")
	}

	out.WriteString("\n      \x1b[38;5;240mpowered by\x1b[0m\n\n")
	for _, line := range wordmark("VAPORA") {
		out.WriteString("      \x1b[38;5;214m" + line + "\x1b[0m\n")
	}

	out.WriteString("\n      \x1b[38;5;240mdirect, encrypted, no server in the middle\x1b[0m\n")
	return out.String()
}

// credit is the one line that stays on screen while the game is running. Small
// and grey on purpose: it belongs in the corner, not in the way.
func credit() string {
	return fmt.Sprintf("\x1b[38;5;240m%s\x1b[0m", "powered by vapora")
}
