package diag

import (
	"strings"
	"testing"
)

func member(name string, profile Profile) Member { return Member{Name: name, Profile: profile} }

func TestAMeshClosesOnlyWhenEveryPairCan(t *testing.T) {
	cases := []struct {
		name    string
		members []Member
		closes  bool
		broken  int
	}{
		{
			"three restricted homes, the common case",
			[]Member{member("ANA", conePort), member("BETO", conePort), member("CARO", conePort)},
			true, 0,
		},
		{
			"one symmetric side reaches the open one and nobody else",
			[]Member{member("ANA", symPort), member("BETO", coneOpen), member("CARO", conePort)},
			false, 1,
		},
		{
			"two symmetric sides break every pair between them",
			[]Member{member("ANA", symPort), member("BETO", symOpen), member("CARO", coneOpen)},
			false, 1,
		},
		{
			"a single profile says nothing",
			[]Member{member("ANA", conePort)},
			true, 0,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			advice := MeshOf(testCase.members)
			if advice.Closes != testCase.closes {
				t.Fatalf("closes=%v, want %v (%s)", advice.Closes, testCase.closes, advice.Reason)
			}
			if len(advice.Broken) != testCase.broken {
				t.Fatalf("%d broken pairs, want %d: %+v", len(advice.Broken), testCase.broken, advice.Broken)
			}
			if advice.Reason == "" {
				t.Fatal("a verdict with no reason is not actionable")
			}
			for _, pair := range advice.Broken {
				if pair.A == "" || pair.B == "" || pair.Reason == "" {
					t.Fatalf("a broken pair that names nobody is useless: %+v", pair)
				}
			}
		})
	}
}

// A room is not all or nothing. The usual outcome is that everyone talks except
// one pair, and naming that pair is the entire point of asking.
func TestAPartlyBrokenRoomNamesThePair(t *testing.T) {
	advice := MeshOf([]Member{
		member("ANA", symPort), member("BETO", coneOpen), member("CARO", conePort),
	})

	if advice.Closes {
		t.Fatal("a room with an unreachable pair was called closed")
	}
	if len(advice.Broken) != 1 {
		t.Fatalf("want exactly the ANA/CARO pair, got %+v", advice.Broken)
	}

	broken := advice.Broken[0]
	if !(broken.A == "ANA" && broken.B == "CARO") {
		t.Fatalf("the wrong pair was blamed: %s and %s", broken.A, broken.B)
	}
	// ANA still reaches BETO, so she is not isolated and must not be reported
	// as somebody staring at an empty room.
	if len(advice.Isolated) != 0 {
		t.Fatalf("%v was called isolated while still reaching somebody", advice.Isolated)
	}
}

// Somebody who reaches nobody sees an empty room and no reason for it, which is
// the one outcome worth calling out separately.
func TestIsolationIsReportedSeparately(t *testing.T) {
	advice := MeshOf([]Member{
		member("ANA", symPort), member("BETO", conePort), member("CARO", conePort),
	})

	if len(advice.Isolated) != 1 || advice.Isolated[0] != "ANA" {
		t.Fatalf("isolated=%v, want just ANA", advice.Isolated)
	}
	if !strings.Contains(advice.Reason, "ANA") {
		t.Fatalf("the reason does not name who is stranded: %q", advice.Reason)
	}
}

// Three restricted homes connect, but never from one invite alone: the waiting
// side has to be handed the newcomer's address first. That is exactly the case
// that made a room fail where punch worked, so it has to be reported.
func TestExchangesAreReportedWhenOneInviteIsNotEnough(t *testing.T) {
	advice := MeshOf([]Member{
		member("ANA", conePort), member("BETO", conePort), member("CARO", conePort),
	})

	if !advice.Closes {
		t.Fatalf("three restricted homes were called unreachable: %s", advice.Reason)
	}
	if len(advice.Exchanges) != 3 {
		t.Fatalf("%d of 3 pairs were flagged as needing an exchange: %+v",
			len(advice.Exchanges), advice.Exchanges)
	}

	// An open side takes a first packet from anywhere, so nothing to exchange.
	open := MeshOf([]Member{member("ANA", coneOpen), member("BETO", coneOpen)})
	if len(open.Exchanges) != 0 {
		t.Fatalf("two open sides were told to exchange addresses: %+v", open.Exchanges)
	}
}

// Whoever runs it, everyone in the room has to be told the same thing —
// including who should host, or two people will both sit waiting.
func TestTheVerdictDoesNotDependOnWhoAsks(t *testing.T) {
	members := []Member{
		member("ANA", conePort), member("BETO", coneOpen), member("CARO", symPort),
	}
	rotated := []Member{members[2], members[0], members[1]}
	reversed := []Member{members[2], members[1], members[0]}

	first := MeshOf(members)
	for _, order := range [][]Member{rotated, reversed} {
		other := MeshOf(order)
		if other.Closes != first.Closes || len(other.Broken) != len(first.Broken) {
			t.Fatalf("the order changed the verdict: %+v vs %+v", first, other)
		}
		if strings.Join(other.Hosts, ",") != strings.Join(first.Hosts, ",") {
			t.Fatalf("the order changed who hosts: %v vs %v", first.Hosts, other.Hosts)
		}
	}
}

// Hosting is what needs a reachable address, so the person the most others can
// reach is the one worth waiting.
func TestTheMostReachableMemberHosts(t *testing.T) {
	advice := MeshOf([]Member{
		member("ANA", symPort), member("BETO", coneOpen), member("CARO", coneOpen),
	})

	// BETO and CARO are equally good and ANA is impossible to aim at, so both
	// are named and ANA is not.
	if strings.Join(advice.Hosts, ",") != "BETO,CARO" {
		t.Fatalf("hosts=%v, want both of the sides everybody can join", advice.Hosts)
	}

	// When one member is plainly the best host, that is the only name.
	single := MeshOf([]Member{
		member("ANA", symPort), member("BETO", coneOpen), member("CARO", conePort),
	})
	if len(single.Hosts) != 1 || single.Hosts[0] != "BETO" {
		t.Fatalf("hosts=%v, want just the side both others can join", single.Hosts)
	}
}

// Two people running this separately see different names for the same room, so
// a host chosen by name would nominate a different person on each machine and
// both would sit waiting. A tie has to come back empty.
func TestATiedHostIsNotGuessed(t *testing.T) {
	here := MeshOf([]Member{member("you", conePort), member("person 1", conePort)})
	there := MeshOf([]Member{member("person 1", conePort), member("you", conePort)})

	if len(here.Hosts) != 2 || len(there.Hosts) != 2 {
		t.Fatalf("a tie nominated %v on one side and %v on the other", here.Hosts, there.Hosts)
	}
}
