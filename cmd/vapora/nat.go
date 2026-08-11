package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/MalPr0/vapora/pkg/diag"
	"github.com/MalPr0/vapora/pkg/stun"
)

const stunTimeout = 4 * time.Second

func runNAT(args []string) error {
	flags := flag.NewFlagSet("nat", flag.ContinueOnError)
	pair := flags.String("pair", "", "combine with the profile the other side reported")
	if err := flags.Parse(args); err != nil {
		return err
	}

	ctx, cancel := signalContext()
	defer cancel()

	fmt.Println("probing NAT behaviour with public STUN servers...")
	report, err := stun.Probe(ctx, stun.DefaultServers, stunTimeout)
	if err != nil {
		return err
	}

	fmt.Printf("local UDP port: %d\n\n", report.LocalPort)
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

	printProfile(diag.Profile{Mapping: report.Mapping, Filtering: report.Filtering}, *pair)
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
