// Package names turns a public key into something a person can say out loud.
//
// It is deliberately separate from the transport. Deriving a name from a key
// means everyone computes the same one for everyone, with nothing sent and
// nothing to trust, and nobody chooses how they appear on somebody else's
// screen — but none of that is the business of moving bytes, and an application
// that has its own idea of identity should be able to ignore all of it.
package names

import (
	"crypto/hkdf"
	"crypto/sha256"
	"encoding/binary"
)

// KeySize is the width of the keys this names, which is an X25519 public key.
const KeySize = 32

type Key = [KeySize]byte

const info = "vapora nickname v1"

// Full is the whole name: an adjective, a colour and an animal.
func Full(key Key) string {
	adjective, colour, animal := parts(key)
	return adjective + " " + colour + " " + animal
}

// Colour is the word a renderer should paint this participant in, so the label
// and the ink agree instead of the colour being an unrelated hash of the text.
func Colour(key Key) string {
	_, colour, _ := parts(key)
	return colour
}

// Of returns the three words from longest suffix to shortest: an animal, then a
// colour before it, then an adjective before that.
func Of(key Key, words int) string {
	adjective, colour, animal := parts(key)
	switch {
	case words <= 1:
		return animal
	case words == 2:
		return colour + " " + animal
	default:
		return adjective + " " + colour + " " + animal
	}
}

func parts(key Key) (adjective, colour, animal string) {
	material, err := hkdf.Key(sha256.New, key[:], nil, info, 12)
	if err != nil {
		return adjectives[0], colours[0], animals[0]
	}
	return adjectives[int(binary.BigEndian.Uint32(material[0:4]))%len(adjectives)],
		colours[int(binary.BigEndian.Uint32(material[4:8]))%len(colours)],
		animals[int(binary.BigEndian.Uint32(material[8:12]))%len(animals)]
}

// Short picks the shortest name that tells these participants apart: a bare
// animal while that is unique, a colour in front of it when two share one, and
// an adjective in front of that in the rare group where two still match.
//
// It is a function of the whole set rather than of a key on its own, because
// "short enough" only means anything against who else is present. Everyone
// computes it from the same set, so the names agree without being sent.
func Short(keys []Key) map[Key]string {
	short := make(map[Key]string, len(keys))

	for words := 1; words <= 3; words++ {
		counts := map[string]int{}
		for _, key := range keys {
			counts[Of(key, words)]++
		}

		remaining := keys[:0:0]
		for _, key := range keys {
			if counts[Of(key, words)] == 1 || words == 3 {
				short[key] = Of(key, words)
				continue
			}
			remaining = append(remaining, key)
		}
		if len(remaining) == 0 {
			break
		}
		keys = remaining
	}
	return short
}

// Animals is the vocabulary, exposed because a caller may need to derive names
// from something other than a public key and should not have to ship its own
// word list to do it.
func Animals() []string { return animals }

// IndexOfAnimal finds a word in the vocabulary, for a caller that needs the one
// after it.
func IndexOfAnimal(name string) int {
	for i, animal := range animals {
		if animal == name {
			return i
		}
	}
	return 0
}
