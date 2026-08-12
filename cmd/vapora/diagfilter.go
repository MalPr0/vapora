package main

import (
	"context"
	"fmt"
	"net"

	"github.com/MalPr0/vapora/pkg/diag"
	"github.com/MalPr0/vapora/pkg/upnp"
)

func runFilterExperiment(ctx context.Context, subjectPort, externalPort int) error {
	gateway, err := upnp.Discover(ctx, discoveryTimeout)
	if err != nil {
		return err
	}

	subject, err := net.ListenUDP("udp4", &net.UDPAddr{Port: subjectPort})
	if err != nil {
		return fmt.Errorf("cannot open the subject socket: %w", err)
	}
	defer subject.Close()

	control, err := net.ListenUDP("udp4", &net.UDPAddr{})
	if err != nil {
		return fmt.Errorf("cannot open the control socket: %w", err)
	}
	defer control.Close()

	fmt.Println("differential filter test (this takes a minute)")
	experiment := diag.FilterExperiment{
		Mapper: gateway,
		Probe:  diag.STUNProbe{Timeout: stunTimeout},
		Label:  mappingLabel,
	}

	result, err := experiment.Run(ctx, subject, control, externalPort)
	if err != nil {
		return err
	}
	printFilterResult(result)
	return nil
}

func printFilterResult(result *diag.FilterResult) {
	fmt.Printf("  subject  local %d, external %d asked\n", result.SubjectPort, result.ExternalPortAsked)
	fmt.Printf("  control  local %d, no mapping\n\n", result.ControlPort)

	fmt.Printf("  %-9s %-34s %s\n", "", "baseline", "with the mapping")
	fmt.Printf("  %-9s %-34s %s\n", "subject", result.BaselineSubject, result.MappedSubject)
	fmt.Printf("  %-9s %-34s %s\n", "control", result.BaselineControl, result.MappedControl)

	if result.Installed != nil {
		fmt.Printf("\n  mapping  %s %d -> %s:%d, enabled=%t, lease %s\n",
			result.Installed.Protocol, result.Installed.ExternalPort,
			result.Installed.InternalHost, result.Installed.InternalPort,
			result.Installed.Enabled, result.Installed.LeaseDuration)
	}
	if result.EndpointBefore != nil && result.EndpointAfter != nil {
		fmt.Printf("  endpoint %s before, %s after\n", result.EndpointBefore, result.EndpointAfter)
	}

	for _, confounder := range result.Confounders {
		fmt.Printf("  warning: %s\n", confounder)
	}

	fmt.Printf("\nverdict: %s\n", result.Verdict)
	fmt.Println(advice(result.Verdict))
}

func advice(verdict diag.Verdict) string {
	switch verdict {
	case diag.VerdictAlreadyOpen:
		return "  a one way invite already works. run: vapora punch"
	case diag.VerdictInnerRouterFilters:
		return "  a UPnP mapping makes this host reachable by a peer it never contacted,\n" +
			"  so a one way link is enough once punch opens the pinhole for itself"
	case diag.VerdictUpstreamFilters:
		return "  the router that filters is out of reach: no mapping here changes it.\n" +
			"  a one way link needs a rendezvous instead"
	default:
		return "  the comparison did not hold. Re-run it on a quiet network"
	}
}
