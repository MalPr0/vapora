package main

import (
	"context"
	"flag"
	"fmt"
	"net/netip"
	"time"

	"github.com/MalPr0/vapora/pkg/pcp"
)

const pcpProbeTimeout = 6 * time.Second

func probeGateways(ctx context.Context, candidates []gatewayCandidate) {
	fmt.Println("gateways")
	for _, candidate := range candidates {
		fmt.Printf("  %-16s %-18s %s\n", candidate.Address, candidate.Source, probePortControl(ctx, candidate.Address))
	}
}

type gatewayCandidate struct {
	Address string
	Source  string
}

func probePortControl(ctx context.Context, address string) string {
	parsed, err := netip.ParseAddr(address)
	if err != nil {
		return fmt.Sprintf("invalid address: %v", err)
	}

	client, err := pcp.Dial(parsed)
	if err != nil {
		return fmt.Sprintf("unreachable: %v", err)
	}
	defer client.Close()

	probeCtx, cancel := context.WithTimeout(ctx, pcpProbeTimeout)
	defer cancel()

	version, err := client.Detect(probeCtx)
	if err != nil {
		return fmt.Sprintf("no port control (%v)", err)
	}

	external, err := client.ExternalIP(probeCtx)
	if err != nil {
		return fmt.Sprintf("%s, external unknown (%v)", version, err)
	}
	return fmt.Sprintf("%s, external %s", version, external)
}

func runDiag(args []string) error {
	flags := flag.NewFlagSet("diag", flag.ContinueOnError)
	gateway := flags.String("gateway", "", "probe this address instead of searching")
	upstream := flags.String("upstream", "", "address of the upstream router when behind double NAT")
	only := flags.String("only", "", "run just one part: pcp or filter")
	subjectPort := flags.Int("port", 0, "local UDP port for the subject socket, 0 lets the OS choose")
	externalPort := flags.Int("external", 0, "external port to ask the IGD for, 0 mirrors the subject port")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *only != "" && *only != "pcp" && *only != "filter" {
		return fmt.Errorf("invalid -only %q, expected pcp or filter", *only)
	}

	ctx, cancel := signalContext()
	defer cancel()

	if *only != "filter" {
		candidates, err := gatewayCandidates(ctx, *gateway, *upstream)
		if err != nil {
			// No router here answers UPnP. That is a finding, not a reason to
			// stop: the rest of the diagnosis is exactly what somebody on such
			// a network needs, and refusing to run it leaves them with nothing.
			fmt.Printf("gateways\n  none found: %v\n", err)
		} else {
			probeGateways(ctx, candidates)
		}
	}
	if *only == "pcp" {
		return nil
	}

	fmt.Println()
	if err := runFilterExperiment(ctx, *subjectPort, *externalPort); err != nil {
		fmt.Printf("differential filter test unavailable: %v\n", err)
		fmt.Println("it needs a router that speaks UPnP, to open one socket and compare.")
	}

	// The classification runs either way: it is what says whether punching can
	// work at all, and it needs nothing from any router.
	fmt.Println()
	return runNAT(nil)
}
