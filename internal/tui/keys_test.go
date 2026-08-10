package tui

import "testing"

func TestDecodeKey(t *testing.T) {
	cases := []struct {
		name     string
		input    []byte
		kind     KeyKind
		value    rune
		consumed int
	}{
		{"letter", []byte("a"), KeyRune, 'a', 1},
		{"accented rune", []byte("ñ"), KeyRune, 'ñ', 2},
		{"emoji", []byte("🎉"), KeyRune, '🎉', 4},
		{"enter", []byte("\r"), KeyEnter, 0, 1},
		{"newline", []byte("\n"), KeyEnter, 0, 1},
		{"backspace", []byte{0x7F}, KeyBackspace, 0, 1},
		{"ctrl+c", []byte{0x03}, KeyInterrupt, 0, 1},
		{"left arrow", []byte("\x1b[D"), KeyLeft, 0, 3},
		{"right arrow", []byte("\x1b[C"), KeyRight, 0, 3},
		{"home", []byte("\x1b[H"), KeyHome, 0, 3},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			key, consumed := DecodeKey(testCase.input)
			if key.Kind != testCase.kind || consumed != testCase.consumed {
				t.Fatalf("got kind %d consumed %d", key.Kind, consumed)
			}
			if testCase.value != 0 && key.Value != testCase.value {
				t.Fatalf("got %q", key.Value)
			}
		})
	}
}

// A read can land mid sequence. Reporting zero consumed is what lets the caller
// wait for the rest instead of typing the tail of an escape into the message.
func TestDecodeKeyWaitsForPartialSequences(t *testing.T) {
	if _, consumed := DecodeKey([]byte("\x1b")); consumed != 0 {
		t.Fatalf("a lone escape consumed %d", consumed)
	}
	if _, consumed := DecodeKey([]byte("\x1b[")); consumed != 0 {
		t.Fatalf("a partial sequence consumed %d", consumed)
	}
	if _, consumed := DecodeKey([]byte{0xC3}); consumed != 0 {
		t.Fatalf("half a rune consumed %d", consumed)
	}
}

// An unhandled sequence has to be swallowed whole, or its tail lands in the
// text as stray letters.
func TestDecodeKeySwallowsUnknownSequences(t *testing.T) {
	key, consumed := DecodeKey([]byte("\x1b[5~rest"))
	if key.Kind != KeyUnknown || consumed != 4 {
		t.Fatalf("got kind %d consumed %d", key.Kind, consumed)
	}
}

func TestEditorTyping(t *testing.T) {
	var editor Editor
	for _, current := range "hola" {
		editor.Apply(Key{Kind: KeyRune, Value: current})
	}
	if editor.String() != "hola" || editor.Cursor() != 4 {
		t.Fatalf("got %q cursor %d", editor.String(), editor.Cursor())
	}

	editor.Apply(Key{Kind: KeyBackspace})
	if editor.String() != "hol" {
		t.Fatalf("got %q", editor.String())
	}

	editor.Apply(Key{Kind: KeyHome})
	editor.Apply(Key{Kind: KeyRune, Value: '¡'})
	if editor.String() != "¡hol" || editor.Cursor() != 1 {
		t.Fatalf("got %q cursor %d", editor.String(), editor.Cursor())
	}

	editor.Apply(Key{Kind: KeyEnd})
	editor.Apply(Key{Kind: KeyLeft})
	editor.Apply(Key{Kind: KeyBackspace})
	if editor.String() != "¡hl" {
		t.Fatalf("got %q", editor.String())
	}
}

func TestEditorTakeClears(t *testing.T) {
	var editor Editor
	editor.Apply(Key{Kind: KeyRune, Value: 'x'})

	if line := editor.Take(); line != "x" {
		t.Fatalf("got %q", line)
	}
	if !editor.Empty() || editor.Cursor() != 0 {
		t.Fatal("the editor kept state after Take")
	}
}

func TestEditorIgnoresMovementAtTheEdges(t *testing.T) {
	var editor Editor
	editor.Apply(Key{Kind: KeyLeft})
	editor.Apply(Key{Kind: KeyBackspace})
	editor.Apply(Key{Kind: KeyRight})
	if !editor.Empty() || editor.Cursor() != 0 {
		t.Fatalf("got %q cursor %d", editor.String(), editor.Cursor())
	}
}
