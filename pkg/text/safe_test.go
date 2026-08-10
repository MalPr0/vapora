package text

import (
	"strings"
	"testing"
)

func TestSafeStripsTerminalControlSequences(t *testing.T) {
	cases := []struct {
		name string
		line string
		want string
	}{
		{"clear screen", "hola\x1b[2Jchau", "hola[2Jchau"},
		{"cursor move", "\x1b[10;10Hhola", "[10;10Hhola"},
		{"clipboard write", "hola\x1b]52;c;aGkK\x07chau", "hola]52;c;aGkKchau"},
		{"carriage return overwrite", "real\rfake", "realfake"},
		{"bell", "hola\x07", "hola"},
		{"backspace", "hola\x08\x08", "hola"},
		{"c1 control", "holachau", "holachau"},
		{"bidi override", "hola‮chau", "holachau"},
		{"bidi isolate", "hola⁦chau", "holachau"},
		{"tab becomes space", "a\tb", "a b"},
		{"newline dropped", "a\nb", "ab"},
		{"empty", "", ""},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := Safe(testCase.line); got != testCase.want {
				t.Fatalf("got %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestSafeKeepsWhatHumansWrite(t *testing.T) {
	for _, line := range []string{
		"hola, ¿cómo andás?",
		"año, ñandú, über, çava",
		"emoji 🎉 y más 🚀",
		"日本語とEspañol",
		"symbols: <>&\"'/\\|@#$%",
	} {
		if got := Safe(line); got != line {
			t.Fatalf("got %q, want %q untouched", got, line)
		}
	}
}

func TestSafeReplacesInvalidUTF8(t *testing.T) {
	got := Safe(string([]byte{'a', 0xFF, 'b'}))
	if got != "a�b" {
		t.Fatalf("got %q", got)
	}
}

func TestSafeLimitTruncates(t *testing.T) {
	long := ""
	for i := 0; i < 50; i++ {
		long += "x"
	}

	got := SafeLimit(long, 10)
	if got != "xxxxxxx..." {
		t.Fatalf("got %q", got)
	}
	if SafeLimit("corto", 10) != "corto" {
		t.Fatal("a short line must not be truncated")
	}
}

// The limit counts what is rendered, so a line made of stripped controls is
// not silently truncated into nothing.
func TestSafeLimitCountsRenderedRunes(t *testing.T) {
	if got := SafeLimit("\x1b\x1b\x1b\x1b\x1babc", 6); got != "abc" {
		t.Fatalf("got %q", got)
	}
}

func TestValidAcceptsOnlyText(t *testing.T) {
	for _, line := range []string{"hola", "¿cómo andás?", "emoji 🎉", "日本語", ""} {
		if !Valid(line) {
			t.Fatalf("%q should be valid text", line)
		}
	}

	for _, line := range []string{
		"hola\x1b[2Jchau",
		"real\rfake",
		string([]byte{'a', 0xFF, 'b'}),
		"hola\x00chau",
		// A tab is normalised to a space on the way out, so one arriving
		// means the sender is not this program.
		"a b\tc",
		"bidi‮chau",
		strings.Repeat("x", MaxRendered+1),
	} {
		if Valid(line) {
			t.Fatalf("%q should not be valid text", line)
		}
	}
}

// Anything Safe produces has to satisfy Valid, or a line would be sanitised on
// the way out and rejected on the way in.
func TestSafeAlwaysProducesValidText(t *testing.T) {
	for _, line := range []string{
		"hola",
		"hola\x1b[2Jchau",
		strings.Repeat("x", MaxRendered*3),
		strings.Repeat("ñ", MaxRendered+50),
		string([]byte{0xFF, 0xFE, 'a'}),
		"a\tb\rc\x00d",
		"",
	} {
		if got := Safe(line); !Valid(got) {
			t.Fatalf("Safe(%.20q) produced %d runes that Valid rejects", line, len([]rune(got)))
		}
	}
}
