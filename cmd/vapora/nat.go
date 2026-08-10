package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/MalPr0/vapora/pkg/stun"
)

const stunTimeout = 4 * time.Second

func runNAT(args []string) error {
	flags := flag.NewFlagSet("nat", flag.ContinueOnError)
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
	return nil
}
