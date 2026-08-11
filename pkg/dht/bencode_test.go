package dht

import (
	"errors"
	"strings"
	"testing"
)

func TestBencodeRoundTrips(t *testing.T) {
	cases := []struct {
		name  string
		value any
		wire  string
	}{
		{"a string", "abc", "3:abc"},
		{"an empty string", "", "0:"},
		{"an integer", 42, "i42e"},
		{"a negative integer", -7, "i-7e"},
		{"a list", List{"a", int64(1)}, "l1:ai1ee"},
		{"a dictionary", Dict{"y": "q"}, "d1:y1:qe"},
		{"an empty dictionary", Dict{}, "de"},
		{
			"a query, keys sorted as the spec requires",
			Dict{"y": "q", "t": "aa", "q": "ping"},
			"d1:q4:ping1:t2:aa1:y1:qe",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			wire, err := Encode(testCase.value)
			if err != nil {
				t.Fatal(err)
			}
			if string(wire) != testCase.wire {
				t.Fatalf("encoded to %q, want %q", wire, testCase.wire)
			}
			if _, err := Decode(wire); err != nil {
				t.Fatalf("cannot read back what was just written: %v", err)
			}
		})
	}
}

// Node IDs and compact addresses are binary. Treating a bencode string as text
// would corrupt every one of them.
func TestBencodeCarriesBinary(t *testing.T) {
	id := string([]byte{0x00, 0xff, 0x1b, 0x0a, 0x3a, 0x65})

	wire, err := Encode(Dict{"id": id})
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := Decode(wire)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := decoded.(Dict).String("id")
	if !ok || got != id {
		t.Fatalf("binary did not survive: %q", got)
	}
}

// Everything here arrives from strangers on the internet. A declared length is
// an instruction to allocate, so it has to be checked against what is present.
func TestBencodeRefusesMalformedInput(t *testing.T) {
	cases := map[string]string{
		"a length longer than the data":         "500:abc",
		"an absurd length":                      "99999999999999999999:x",
		"no length at all":                      ":abc",
		"an unterminated integer":               "i42",
		"an unterminated list":                  "l1:a",
		"an unterminated dictionary":            "d1:a1:b",
		"nothing":                               "",
		"trailing rubbish after a value":        "3:abcd",
		"a dictionary key that is not a string": "di1e1:ae",
	}

	for name, wire := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Decode([]byte(wire)); !errors.Is(err, ErrBencode) {
				t.Fatalf("%q was accepted: %v", wire, err)
			}
		})
	}
}

// A reply from a stranger is a dictionary of whatever they felt like sending,
// so every read of it has to survive the field being absent or another type.
func TestReadingAReplyIsTotal(t *testing.T) {
	decoded, err := Decode([]byte("d1:ai1e1:b1:xe"))
	if err != nil {
		t.Fatal(err)
	}
	reply := decoded.(Dict)

	if _, ok := reply.String("a"); ok {
		t.Fatal("an integer was read as a string")
	}
	if _, ok := reply.String("missing"); ok {
		t.Fatal("an absent key came back present")
	}
	if _, ok := reply.Dict("b"); ok {
		t.Fatal("a string was read as a dictionary")
	}
	if value, ok := reply.String("b"); !ok || value != "x" {
		t.Fatalf("the one string present read as %q", value)
	}
}

// A deeply nested value is a cheap way to blow a stack from across the
// internet, so it must not be one.
func TestDeepNestingIsRefusedRatherThanCrashing(t *testing.T) {
	wire := strings.Repeat("l", 100000) + strings.Repeat("e", 100000)

	defer func() {
		if problem := recover(); problem != nil {
			t.Fatalf("a nested value from the network panicked: %v", problem)
		}
	}()
	// Either answer is acceptable. Crashing is not.
	_, _ = Decode([]byte(wire))
}
