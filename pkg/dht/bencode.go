// Package dht speaks just enough of the BitTorrent mainline DHT to leave an
// address under a key and to find one somebody else left.
//
// It is a client, not a node: it answers no queries and keeps no routing table
// between runs. Everything here exists to serve two operations, announce and
// lookup, against a network of millions of nodes that already exists.
package dht

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
)

var ErrBencode = errors.New("dht: malformed bencode")

// Bencode values are one of four things. Strings carry bytes, not text: node
// IDs and compact addresses are binary and would not survive being treated as
// UTF-8.
type (
	// Dict is a bencode dictionary. Keys are sorted on the wire, which matters
	// because a query is hashed by some implementations.
	Dict map[string]any
	List []any
)

// Encode writes a value in bencode.
func Encode(value any) ([]byte, error) {
	var out []byte
	return appendValue(out, value)
}

func appendValue(out []byte, value any) ([]byte, error) {
	switch typed := value.(type) {
	case string:
		return appendString(out, typed), nil
	case []byte:
		return appendString(out, string(typed)), nil
	case int:
		return appendInt(out, int64(typed)), nil
	case int64:
		return appendInt(out, typed), nil
	case List:
		out = append(out, 'l')
		for _, item := range typed {
			var err error
			if out, err = appendValue(out, item); err != nil {
				return nil, err
			}
		}
		return append(out, 'e'), nil
	case Dict:
		return appendDict(out, typed)
	default:
		return nil, fmt.Errorf("dht: cannot encode %T", value)
	}
}

func appendDict(out []byte, dict Dict) ([]byte, error) {
	keys := make([]string, 0, len(dict))
	for key := range dict {
		keys = append(keys, key)
	}
	// Sorted keys are required by the spec, and a receiver that verifies is
	// entitled to drop anything else.
	sort.Strings(keys)

	out = append(out, 'd')
	for _, key := range keys {
		out = appendString(out, key)
		var err error
		if out, err = appendValue(out, dict[key]); err != nil {
			return nil, err
		}
	}
	return append(out, 'e'), nil
}

func appendString(out []byte, value string) []byte {
	out = strconv.AppendInt(out, int64(len(value)), 10)
	out = append(out, ':')
	return append(out, value...)
}

func appendInt(out []byte, value int64) []byte {
	out = append(out, 'i')
	out = strconv.AppendInt(out, value, 10)
	return append(out, 'e')
}

// Decode reads one value. Everything here arrives from strangers on the
// internet, so every length is checked against what is actually left rather
// than trusted: a declared length is an instruction to allocate.
func Decode(data []byte) (any, error) {
	value, rest, err := decodeValue(data)
	if err != nil {
		return nil, err
	}
	if len(rest) != 0 {
		return nil, fmt.Errorf("%w: %d trailing bytes", ErrBencode, len(rest))
	}
	return value, nil
}

func decodeValue(data []byte) (any, []byte, error) {
	if len(data) == 0 {
		return nil, nil, fmt.Errorf("%w: nothing to read", ErrBencode)
	}

	switch data[0] {
	case 'i':
		return decodeInt(data)
	case 'l':
		return decodeList(data)
	case 'd':
		return decodeDict(data)
	default:
		return decodeString(data)
	}
}

func decodeString(data []byte) (any, []byte, error) {
	colon := -1
	for i, current := range data {
		if current == ':' {
			colon = i
			break
		}
		if current < '0' || current > '9' {
			return nil, nil, fmt.Errorf("%w: %q is not a length", ErrBencode, current)
		}
		// A length nobody could satisfy is a claim, not a string.
		if i > 10 {
			return nil, nil, fmt.Errorf("%w: absurd string length", ErrBencode)
		}
	}
	if colon < 1 {
		return nil, nil, fmt.Errorf("%w: string with no length", ErrBencode)
	}

	size, err := strconv.Atoi(string(data[:colon]))
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrBencode, err)
	}
	if size > len(data)-colon-1 {
		return nil, nil, fmt.Errorf("%w: declared %d bytes, %d present", ErrBencode, size, len(data)-colon-1)
	}
	return string(data[colon+1 : colon+1+size]), data[colon+1+size:], nil
}

func decodeInt(data []byte) (any, []byte, error) {
	for i := 1; i < len(data); i++ {
		if data[i] != 'e' {
			continue
		}
		value, err := strconv.ParseInt(string(data[1:i]), 10, 64)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: %v", ErrBencode, err)
		}
		return value, data[i+1:], nil
	}
	return nil, nil, fmt.Errorf("%w: unterminated integer", ErrBencode)
}

func decodeList(data []byte) (any, []byte, error) {
	list := List{}
	rest := data[1:]

	for len(rest) > 0 {
		if rest[0] == 'e' {
			return list, rest[1:], nil
		}
		item, remaining, err := decodeValue(rest)
		if err != nil {
			return nil, nil, err
		}
		list = append(list, item)
		rest = remaining
	}
	return nil, nil, fmt.Errorf("%w: unterminated list", ErrBencode)
}

func decodeDict(data []byte) (any, []byte, error) {
	dict := Dict{}
	rest := data[1:]

	for len(rest) > 0 {
		if rest[0] == 'e' {
			return dict, rest[1:], nil
		}

		key, remaining, err := decodeString(rest)
		if err != nil {
			return nil, nil, err
		}
		value, remaining, err := decodeValue(remaining)
		if err != nil {
			return nil, nil, err
		}
		dict[key.(string)] = value
		rest = remaining
	}
	return nil, nil, fmt.Errorf("%w: unterminated dictionary", ErrBencode)
}

// String pulls a string out of a dictionary, which is most of what reading a
// reply amounts to.
func (d Dict) String(key string) (string, bool) {
	value, ok := d[key].(string)
	return value, ok
}

func (d Dict) Dict(key string) (Dict, bool) {
	value, ok := d[key].(Dict)
	return value, ok
}

func (d Dict) List(key string) (List, bool) {
	value, ok := d[key].(List)
	return value, ok
}
