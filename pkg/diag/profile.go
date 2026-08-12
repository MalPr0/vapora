package diag

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"

	"github.com/MalPr0/vapora/pkg/stun"
)

// ErrBadProfile covers a code that is not one, including one that lost a
// character on its way through a chat window — which is why the code carries a
// check byte at all.
var ErrBadProfile = errors.New("diag: not a network profile")

// Profile is everything about one side that decides whether two people can
// reach each other. It is small on purpose: it has to survive being read out
// over a chat window.
type Profile struct {
	Mapping   stun.Mapping
	Filtering stun.Filtering
}

var (
	mappingCodes = map[stun.Mapping]string{
		stun.MappingEndpointIndependent: "CONE",
		stun.MappingAddressDependent:    "SYM",
	}
	filteringCodes = map[stun.Filtering]string{
		stun.FilteringEndpointIndependent:     "OPEN",
		stun.FilteringAddressDependent:        "ADDR",
		stun.FilteringAddressAndPortDependent: "PORT",
	}
)

// Code is the shareable form. Connectivity is a property of the pair, not of
// either end, so this exists to be pasted to the other side rather than read
// on its own.
func (p Profile) Code() string {
	mapping, filtering := mappingCodes[p.Mapping], filteringCodes[p.Filtering]
	if mapping == "" {
		mapping = "UNKNOWN"
	}
	if filtering == "" {
		filtering = "UNKNOWN"
	}

	body := mapping + "-" + filtering
	return body + "-" + check(body)
}

// check catches a code that lost a character on its way through a chat window,
// which otherwise reads as a different network entirely.
func check(body string) string {
	sum := sha256.Sum256([]byte("vapora profile v1 " + body))
	return fmt.Sprintf("%02X", sum[0])
}

// ParseProfile reads a code somebody sent back. Case and stray whitespace are
// tolerated because that is what a paste looks like; a lost character is not,
// because it would otherwise describe a different network entirely.
func ParseProfile(code string) (Profile, error) {
	parts := strings.Split(strings.ToUpper(strings.TrimSpace(code)), "-")
	if len(parts) != 3 {
		return Profile{}, fmt.Errorf("%w: %q", ErrBadProfile, code)
	}

	body := parts[0] + "-" + parts[1]
	if parts[2] != check(body) {
		return Profile{}, fmt.Errorf("%w: %q does not check out, a character was probably lost", ErrBadProfile, code)
	}

	profile := Profile{}
	for mapping, name := range mappingCodes {
		if name == parts[0] {
			profile.Mapping = mapping
		}
	}
	for filtering, name := range filteringCodes {
		if name == parts[1] {
			profile.Filtering = filtering
		}
	}
	return profile, nil
}

// Advice is what two profiles say about each other.
type Advice struct {
	// Works is whether a direct path is reachable at all.
	Works bool
	// Invites is how many have to change hands: one when a side can take an
	// unannounced first packet, two when neither can.
	Invites int
	// Publisher says which side has to be the one waiting, when it matters.
	Publisher string
	Reason    string
}

// Pair says what to expect of two sides together.
//
// No measurement of one end can answer this: a side that is perfectly open
// still cannot reach somebody whose address is unpredictable. That is why the
// profile is made to be shared rather than merely reported.
func Pair(mine, theirs Profile) Advice {
	mineSym := mine.Mapping == stun.MappingAddressDependent
	theirsSym := theirs.Mapping == stun.MappingAddressDependent

	switch {
	case mineSym && theirsSym:
		return Advice{
			Reason: "both sides hand out a new port per destination, so neither can be told " +
				"where to aim. Nothing but a relay reaches across this",
		}

	case mineSym || theirsSym:
		// A side that picks a fresh port per destination cannot be aimed at,
		// but it can still do the aiming, as long as the other end accepts a
		// first packet from an address it never contacted.
		open, symmetric := theirs, "you"
		publisher := "them"
		if theirsSym {
			open, symmetric, publisher = mine, "they", "you"
		}
		if open.Filtering != stun.FilteringEndpointIndependent {
			return Advice{
				Reason: fmt.Sprintf("%s hand out a new port per destination, so the other side "+
					"cannot be told where to aim, and that side will not take a packet from an "+
					"address it never contacted. Only a relay gets across this", symmetric),
			}
		}
		return Advice{
			Works: true, Invites: 1, Publisher: publisher,
			Reason: "one side takes a first packet from anywhere, so it publishes the invite " +
				"and the other side joins. Only that direction works",
		}

	case mine.Filtering == stun.FilteringEndpointIndependent && theirs.Filtering == stun.FilteringEndpointIndependent:
		// Telling both sides to publish is how two people end up waiting for
		// each other, which is the quietest way for this to fail.
		return Advice{
			Works: true, Invites: 1, Publisher: "either",
			Reason: "both sides take a first packet from anywhere, so a single invite is enough " +
				"in either direction. Agree on who publishes, because if you both wait you " +
				"will both wait forever",
		}

	case mine.Filtering == stun.FilteringEndpointIndependent || theirs.Filtering == stun.FilteringEndpointIndependent:
		publisher := "you"
		if mine.Filtering != stun.FilteringEndpointIndependent {
			publisher = "them"
		}
		return Advice{
			Works: true, Invites: 1, Publisher: publisher,
			Reason: "one side takes a first packet from anywhere, so a single invite is enough " +
				"as long as that side is the one waiting",
		}

	default:
		return Advice{
			Works: true, Invites: 2, Publisher: "either",
			Reason: "neither side takes a first packet from an address it never contacted, so " +
				"one invite is never enough: both have to be exchanged and both sides have to " +
				"be running at the same time",
		}
	}
}
