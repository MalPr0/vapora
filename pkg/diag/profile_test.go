package diag

import (
	"errors"
	"strings"
	"testing"

	"github.com/MalPr0/vapora/pkg/stun"
)

func profile(mapping stun.Mapping, filtering stun.Filtering) Profile {
	return Profile{Mapping: mapping, Filtering: filtering}
}

var (
	coneOpen = profile(stun.MappingEndpointIndependent, stun.FilteringEndpointIndependent)
	conePort = profile(stun.MappingEndpointIndependent, stun.FilteringAddressAndPortDependent)
	coneAddr = profile(stun.MappingEndpointIndependent, stun.FilteringAddressDependent)
	symPort  = profile(stun.MappingAddressDependent, stun.FilteringAddressAndPortDependent)
	symOpen  = profile(stun.MappingAddressDependent, stun.FilteringEndpointIndependent)
)

func TestProfileRoundTrips(t *testing.T) {
	for _, want := range []Profile{coneOpen, conePort, coneAddr, symPort, symOpen, {}} {
		got, err := ParseProfile(want.Code())
		if err != nil {
			t.Fatalf("%s: %v", want.Code(), err)
		}
		if got != want {
			t.Fatalf("%s came back as %+v", want.Code(), got)
		}
	}

	// A code is going to be read out over a chat window, so losing a character
	// has to fail loudly rather than describe a different network.
	broken := conePort.Code()
	for _, mangled := range []string{
		broken[:len(broken)-1],
		strings.Replace(broken, "CONE", "SYM", 1),
		"CONE-PORT",
		"",
		"nonsense",
	} {
		if _, err := ParseProfile(mangled); !errors.Is(err, ErrBadProfile) {
			t.Fatalf("%q was accepted", mangled)
		}
	}

	// Case and stray spaces are what a paste actually looks like.
	if _, err := ParseProfile("  " + strings.ToLower(conePort.Code()) + "  "); err != nil {
		t.Fatalf("a pasted code was rejected: %v", err)
	}
}

func TestPairVerdicts(t *testing.T) {
	cases := []struct {
		name      string
		mine      Profile
		theirs    Profile
		works     bool
		invites   int
		publisher string
	}{
		{"both restricted, the common case", conePort, conePort, true, 2, "either"},
		{"one takes anything", coneOpen, conePort, true, 1, "you"},
		{"the other takes anything", conePort, coneOpen, true, 1, "them"},
		{"both take anything", coneOpen, coneOpen, true, 1, "either"},
		{"address dependent still needs two", coneAddr, conePort, true, 2, "either"},
		{"symmetric against an open side aims itself", symPort, coneOpen, true, 1, "them"},
		{"open side against symmetric publishes", coneOpen, symPort, true, 1, "you"},
		{"symmetric against a restricted side is out of reach", symPort, conePort, false, 0, ""},
		{"two symmetric sides are hopeless", symPort, symOpen, false, 0, ""},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			advice := Pair(testCase.mine, testCase.theirs)
			if advice.Works != testCase.works {
				t.Fatalf("works=%v, want %v (%s)", advice.Works, testCase.works, advice.Reason)
			}
			if advice.Invites != testCase.invites {
				t.Fatalf("invites=%d, want %d", advice.Invites, testCase.invites)
			}
			if testCase.publisher != "" && advice.Publisher != testCase.publisher {
				t.Fatalf("publisher=%q, want %q", advice.Publisher, testCase.publisher)
			}
			if advice.Reason == "" {
				t.Fatal("a verdict with no reason is not actionable")
			}
		})
	}
}

// Whoever runs it, the two sides have to be told the same thing.
func TestPairIsSymmetric(t *testing.T) {
	all := []Profile{coneOpen, conePort, coneAddr, symPort, symOpen}

	for _, mine := range all {
		for _, theirs := range all {
			here, there := Pair(mine, theirs), Pair(theirs, mine)
			if here.Works != there.Works || here.Invites != there.Invites {
				t.Fatalf("%s vs %s: one side was told %+v and the other %+v",
					mine.Code(), theirs.Code(), here, there)
			}
			// The publisher is the one thing that has to be mirrored rather
			// than matched, or both sides would sit waiting for each other.
			if here.Publisher == "you" && there.Publisher != "them" {
				t.Fatalf("%s vs %s: both sides were told to publish", mine.Code(), theirs.Code())
			}
		}
	}
}
