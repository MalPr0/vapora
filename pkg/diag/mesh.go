package diag

import (
	"fmt"
	"sort"
	"strings"
)

// Member is one side of a room, named so a verdict can point at somebody.
type Member struct {
	Name    string
	Profile Profile
}

// Unreachable is one pair that cannot see each other. A room is not all or
// nothing: the usual outcome is that everybody talks except for one pair, and
// naming that pair is the whole point.
type Unreachable struct {
	A, B   string
	Reason string
}

// MeshAdvice is what a set of profiles says about a room.
type MeshAdvice struct {
	// Closes is whether every pair can reach every other pair, which is what a
	// mesh needs: there is no relaying, so a pair that cannot connect stays
	// silent to each other however well the rest of the room works.
	Closes bool
	// Broken lists the pairs that cannot connect, in a stable order.
	Broken []Unreachable
	// Isolated names anyone who cannot reach a single other member. They will
	// see an empty room and no reason for it.
	Isolated []string
	// Hosts is everyone equally suited to open the room: the members the most
	// others could actually join. More than one is a real answer, not a missing
	// one — the names here are local to whoever asked ("you" on one machine is
	// "person 2" on another), so picking between them would nominate a
	// different person on each machine and both would sit waiting. Several
	// names means agree among yourselves; naming them matters because the
	// person asking is often not one of them.
	Hosts []string
	// Exchanges names the pairs where one invite is not enough, meaning the
	// waiting side has to be handed the newcomer's address before their hello
	// gets through. It is the room's version of `punch` sending two invites.
	Exchanges []Unreachable
	Reason    string
}

// MeshOf works out whether a room of these members closes.
//
// A room is not a connection with more people in it: it is every pair at once,
// and Pair already knows what a pair costs. What is new is that a room can be
// partly broken, which no two-party answer can express.
func MeshOf(members []Member) MeshAdvice {
	if len(members) < 2 {
		return MeshAdvice{
			Closes: true,
			Reason: "a room needs at least two profiles to say anything",
		}
	}

	// hostable counts, per member, how many others could join a room they were
	// waiting in. That is not the same as how many pairs work: a side that hands
	// out a new port per destination can connect to plenty of people and still
	// be impossible to aim at, which makes it the worst host in the room.
	hostable := map[string]int{}
	// reached is the plain "can these two talk at all" count, which is what
	// isolation is about.
	reached := map[string]int{}
	var broken, exchanges []Unreachable

	for i := 0; i < len(members); i++ {
		for j := i + 1; j < len(members); j++ {
			a, b := members[i], members[j]
			advice := Pair(a.Profile, b.Profile)
			if !advice.Works {
				broken = append(broken, Unreachable{A: a.Name, B: b.Name, Reason: advice.Reason})
				continue
			}
			if advice.Invites > 1 {
				// The pair connects, but not from one invite alone. In a room
				// that means the waiting side needs the newcomer's address
				// pasted in before their hello survives the trip.
				exchanges = append(exchanges, Unreachable{
					A: a.Name, B: b.Name,
					Reason: "neither side takes a first packet from a stranger, so whoever waits " +
						"has to be given the other's address",
				})
			}
			if advice.Publisher == "you" || advice.Publisher == "either" {
				hostable[a.Name]++
			}
			if advice.Publisher == "them" || advice.Publisher == "either" {
				hostable[b.Name]++
			}
			reached[a.Name]++
			reached[b.Name]++
		}
	}

	var isolated []string
	for _, member := range members {
		if reached[member.Name] == 0 {
			isolated = append(isolated, member.Name)
		}
	}
	sort.Strings(isolated)

	return MeshAdvice{
		Closes:    len(broken) == 0,
		Broken:    broken,
		Isolated:  isolated,
		Hosts:     hostsOf(members, hostable),
		Exchanges: exchanges,
		Reason:    meshReason(members, broken, isolated),
	}
}

// hostsOf lists everyone who could open the room: the members the most others
// could actually join. All of them are returned rather than one, because the
// names are local to whoever ran this and breaking the tie here would nominate
// a different person on each machine.
func hostsOf(members []Member, hostable map[string]int) []string {
	best := 0
	for _, member := range members {
		if hostable[member.Name] > best {
			best = hostable[member.Name]
		}
	}

	var hosts []string
	for _, member := range members {
		if hostable[member.Name] == best {
			hosts = append(hosts, member.Name)
		}
	}
	sort.Strings(hosts)
	return hosts
}

func meshReason(members []Member, broken []Unreachable, isolated []string) string {
	if len(broken) == 0 {
		return fmt.Sprintf("every one of the %d pairs among these %d people can connect, "+
			"so the room closes", pairs(len(members)), len(members))
	}

	if len(isolated) > 0 {
		return fmt.Sprintf("%s cannot reach anybody here and will sit in what looks like an "+
			"empty room. The other %d can still talk among themselves",
			strings.Join(isolated, " and "), len(members)-len(isolated))
	}

	return fmt.Sprintf("%d of the %d pairs cannot connect. Everyone still shows up in the "+
		"room, because who is present travels through the people who can reach each other, "+
		"but those pairs show each other as a dead link and drop off after a few minutes",
		len(broken), pairs(len(members)))
}

func pairs(count int) int { return count * (count - 1) / 2 }
