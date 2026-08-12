package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/MalPr0/vapora/pkg/diag"
	"github.com/MalPr0/vapora/pkg/stun"
)

const stunTimeout = 4 * time.Second

func runNAT(args []string) error {
	flags := flag.NewFlagSet("nat", flag.ContinueOnError)
	pair := flags.String("pair", "", "combine with the profile the other side reported")
	room := flags.String("room", "", "comma separated profiles of everyone who will be in a room")
	port := flags.Int("port", 0, "measure this UDP port instead of one the OS picks")
	if err := flags.Parse(args); err != nil {
		return err
	}

	ctx, cancel := signalContext()
	defer cancel()

	fmt.Println("probing NAT behaviour with public STUN servers...")

	// Filtering is a property of a port, not of a machine. A firewall rule that
	// opens one port says nothing about any other, so measuring an ephemeral
	// port on a server whose rule names 41000 answers the wrong question — and
	// answers it wrongly, reporting the closed default rather than the port
	// that will actually be used.
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{Port: *port})
	if err != nil {
		return fmt.Errorf("cannot open UDP port %d: %w", *port, err)
	}
	defer conn.Close()

	report, err := stun.ProbeWith(ctx, conn, stun.DefaultServers, stunTimeout)
	if err != nil {
		return err
	}

	fmt.Printf("local UDP port: %d\n", report.LocalPort)
	if *port == 0 {
		fmt.Println("  (this is a port the OS picked. If a firewall rule opens one specific")
		fmt.Println("   port, measure that one instead: vapora nat -port <the open port>)")
	}
	fmt.Println()
	for _, observation := range report.Observations {
		if observation.Err != nil {
			fmt.Printf("  %-32s %s\n", observation.Server, observation.Err)
			continue
		}
		fmt.Printf("  %-32s (%s) sees %s\n", observation.Server, observation.ServerIP, observation.Mapped)
	}

	fmt.Printf("\nmapping:   %s\n", report.Mapping)
	fmt.Printf("filtering: %s", report.Filtering)
	if report.FilteringServer != "" {
		fmt.Printf(" (per %s)", report.FilteringServer)
	}
	fmt.Println()
	fmt.Printf("port preserved: %t\n", report.PortPreserved)

	if !report.HolePunchViable {
		fmt.Println("\nUDP hole punching is not viable from this network, a relay is required")
		return nil
	}

	fmt.Println("\nUDP hole punching is viable, both sides run: vapora punch")
	if report.Filtering.AcceptsUnknownPeers() {
		fmt.Println("this NAT also accepts unannounced peers, so an invite can be handed out one way")
	} else {
		fmt.Println("this NAT drops packets from peers it never contacted, so both sides must punch at once")
	}

	mine := diag.Profile{Mapping: report.Mapping, Filtering: report.Filtering}
	if *room != "" {
		return printMesh(mine, *room)
	}
	printProfile(mine, *pair)
	return nil
}

// printProfile reports this side in a form that can be pasted to the other one,
// and pairs it with theirs when they sent it back. Nothing measured here can
// say whether a connection will work: that is a property of the two ends
// together, so the answer only exists once both halves are in one place.
func printProfile(mine diag.Profile, theirs string) {
	fmt.Printf("\nyour profile: %s\n", mine.Code())

	if theirs == "" {
		fmt.Println("send that to whoever you are connecting to, and run")
		fmt.Println("  vapora nat -pair <the profile they send back>")
		return
	}

	other, err := diag.ParseProfile(theirs)
	if err != nil {
		fmt.Printf("\ncannot read their profile: %v\n", err)
		return
	}

	advice := diag.Pair(mine, other)
	fmt.Printf("\ntogether: %s\n", advice.Reason)
	switch {
	case !advice.Works:
		fmt.Println("verdict: a direct path is not reachable between these two networks")
	case advice.Invites == 1 && advice.Publisher == "either":
		fmt.Println("verdict: works, one invite either way. Agree on who runs `vapora punch`:")
		fmt.Println("         if you both wait for each other, you both wait forever")
	case advice.Invites == 1 && advice.Publisher == "you":
		fmt.Println("verdict: works. You run `vapora punch` and send the line; they join with it")
	case advice.Invites == 1 && advice.Publisher == "them":
		fmt.Println("verdict: works. They run `vapora punch` and send the line; you join with it")
	default:
		fmt.Println("verdict: works, but one invite is not enough. Whoever joins gets a second")
		fmt.Println("         line to send back, and it has to be pasted into the waiting side")
	}
}

// printMesh answers the room question, which no pair of profiles can: a room is
// every pair at once, and it can be partly broken in a way two-party advice has
// no way to express.
func printMesh(mine diag.Profile, others string) error {
	members := []diag.Member{{Name: "you", Profile: mine}}

	for i, code := range strings.Split(others, ",") {
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		profile, err := diag.ParseProfile(code)
		if err != nil {
			return fmt.Errorf("profile %d: %w", i+1, err)
		}
		members = append(members, diag.Member{Name: fmt.Sprintf("person %d", i+1), Profile: profile})
	}

	if len(members) < 2 {
		return errors.New("a room needs at least one other profile to say anything")
	}

	advice := diag.MeshOf(members)
	fmt.Printf("\nyour profile: %s\n", mine.Code())
	fmt.Printf("\nroom of %d: %s\n", len(members), advice.Reason)

	for _, pair := range advice.Broken {
		fmt.Printf("  %s and %s cannot connect: %s\n", pair.A, pair.B, pair.Reason)
	}

	switch {
	case !advice.Closes:
		fmt.Println("\nverdict: this room does not close. The pairs above stay silent to each")
		fmt.Println("         other however well the rest of it works, because nobody relays")
	case len(advice.Exchanges) > 0:
		fmt.Printf("\nverdict: works, but not from one invite. %s opens the room.\n", hostName(advice.Hosts))
		fmt.Println("         Everyone who cannot get in sends back the address their side")
		fmt.Println("         prints, for whoever is waiting to paste in")
	default:
		fmt.Printf("\nverdict: works. %s opens the room and shares the invite.\n", hostName(advice.Hosts))
		fmt.Println("         Anyone already in can invite the next person with !invite")
	}
	return nil
}

// hostName turns the candidates into words. Several names is an instruction to
// agree, not a failure to answer — and they are named rather than left as "any
// of you", because the person reading this is often not among them.
func hostName(hosts []string) string {
	switch len(hosts) {
	case 0:
		return "nobody here"
	case 1:
		return hosts[0]
	case 2:
		return strings.Join(hosts, " or ") + " (agree which)"
	default:
		return "any of " + strings.Join(hosts, ", ") + " (agree which)"
	}
}
