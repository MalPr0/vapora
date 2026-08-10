package tui

import "unicode/utf8"

// KeyKind separates the keys the editor acts on from ordinary text.
type KeyKind int

const (
	KeyRune KeyKind = iota
	KeyEnter
	KeyBackspace
	KeyLeft
	KeyRight
	KeyHome
	KeyEnd
	KeyInterrupt
	KeyUnknown
)

type Key struct {
	Kind  KeyKind
	Value rune
}

// DecodeKey reads one key from the front of buf and returns how many bytes it
// consumed. It returns 0 when buf holds only part of a sequence, so the caller
// can wait for the rest instead of misreading a split escape as Escape itself.
func DecodeKey(buf []byte) (Key, int) {
	if len(buf) == 0 {
		return Key{Kind: KeyUnknown}, 0
	}

	switch buf[0] {
	case '\r', '\n':
		return Key{Kind: KeyEnter}, 1
	case 0x7F, 0x08:
		return Key{Kind: KeyBackspace}, 1
	case 0x03:
		return Key{Kind: KeyInterrupt}, 1
	case 0x01:
		return Key{Kind: KeyHome}, 1
	case 0x05:
		return Key{Kind: KeyEnd}, 1
	case 0x1B:
		return decodeEscape(buf)
	}

	if buf[0] < 0x20 {
		return Key{Kind: KeyUnknown}, 1
	}

	value, size := utf8.DecodeRune(buf)
	if value == utf8.RuneError && size == 1 && !utf8.FullRune(buf) {
		return Key{Kind: KeyUnknown}, 0 // a multi byte rune split across reads
	}
	return Key{Kind: KeyRune, Value: value}, size
}

func decodeEscape(buf []byte) (Key, int) {
	if len(buf) < 3 {
		return Key{Kind: KeyUnknown}, 0
	}
	if buf[1] != '[' && buf[1] != 'O' {
		return Key{Kind: KeyUnknown}, 1
	}

	switch buf[2] {
	case 'C':
		return Key{Kind: KeyRight}, 3
	case 'D':
		return Key{Kind: KeyLeft}, 3
	case 'H':
		return Key{Kind: KeyHome}, 3
	case 'F':
		return Key{Kind: KeyEnd}, 3
	}

	// Anything else is a sequence this UI does not act on; swallow it whole so
	// its tail never lands in the message being typed.
	for i := 2; i < len(buf); i++ {
		if buf[i] >= 0x40 && buf[i] <= 0x7E {
			return Key{Kind: KeyUnknown}, i + 1
		}
	}
	return Key{Kind: KeyUnknown}, 0
}

// Editor is the input line: a rune buffer with a cursor.
type Editor struct {
	runes  []rune
	cursor int
}

func (e *Editor) Apply(key Key) {
	switch key.Kind {
	case KeyRune:
		e.runes = append(e.runes, 0)
		copy(e.runes[e.cursor+1:], e.runes[e.cursor:])
		e.runes[e.cursor] = key.Value
		e.cursor++
	case KeyBackspace:
		if e.cursor > 0 {
			e.runes = append(e.runes[:e.cursor-1], e.runes[e.cursor:]...)
			e.cursor--
		}
	case KeyLeft:
		if e.cursor > 0 {
			e.cursor--
		}
	case KeyRight:
		if e.cursor < len(e.runes) {
			e.cursor++
		}
	case KeyHome:
		e.cursor = 0
	case KeyEnd:
		e.cursor = len(e.runes)
	}
}

func (e *Editor) String() string { return string(e.runes) }
func (e *Editor) Cursor() int    { return e.cursor }
func (e *Editor) Empty() bool    { return len(e.runes) == 0 }

// Take returns the line and clears the editor.
func (e *Editor) Take() string {
	line := string(e.runes)
	e.runes = e.runes[:0]
	e.cursor = 0
	return line
}
