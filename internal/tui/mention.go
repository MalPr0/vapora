package tui

import "strings"

// MentionMark is what a line addressed to you carries in the gutter. A colour
// alone would not survive a colourblind reader or a screenshot in grayscale.
const MentionMark = "▸"

// mentions reports whether a line addresses any of the given names.
//
// Names are derived from keys, so every member computes the same one for
// everyone: an @ needs nothing negotiated and cannot be claimed by somebody
// else. Matching runs longest name first, because a short name can be the tail
// of a longer one and the longer reading is the one the writer meant.
func mentions(body string, names []string) bool {
	if len(names) == 0 {
		return false
	}

	upper := strings.ToUpper(body)
	var wanted []string
	for _, name := range longestFirst(names) {
		if name == "" {
			continue
		}
		wanted = append(wanted, strings.ToUpper(name))
		// A multi word name is also addressable by its last word, which is the
		// animal, since that is what people will actually type.
		if fields := strings.Fields(name); len(fields) > 1 {
			wanted = append(wanted, strings.ToUpper(fields[len(fields)-1]))
		}
	}

	for _, at := range starts(upper) {
		for _, name := range wanted {
			if strings.HasPrefix(upper[at+1:], name) {
				return true
			}
		}
	}
	return false
}

// starts finds the @ signs that begin a mention. An @ with a letter in front of
// it is part of something else, most often an address, and highlighting every
// email in a conversation would make the mark worthless.
func starts(upper string) []int {
	var found []int
	for index, current := range upper {
		if current != '@' {
			continue
		}
		if index > 0 && isNameRune(rune(upper[index-1])) {
			continue
		}
		found = append(found, index)
	}
	return found
}

func isNameRune(value rune) bool {
	return value >= 'A' && value <= 'Z' || value >= '0' && value <= '9'
}

func longestFirst(names []string) []string {
	sorted := append([]string(nil), names...)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && len(sorted[j]) > len(sorted[j-1]); j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	return sorted
}
