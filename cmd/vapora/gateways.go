package main

import (
	"context"
	"fmt"

	"github.com/MalPr0/vapora/pkg/upnp"
)

// gatewayCandidates lists the routers worth probing, in the order they can be
// learned: what the user named, the IGD that answered SSDP, and the upstream
// router derived from that IGD's own external address.
func gatewayCandidates(ctx context.Context, gateway, upstream string) ([]gatewayCandidate, error) {
	if gateway != "" {
		return []gatewayCandidate{{Address: gateway, Source: "flag"}}, nil
	}

	found, err := upnp.Discover(ctx, discoveryTimeout)
	if err != nil {
		if upstream == "" {
			return nil, err
		}
		return []gatewayCandidate{{Address: upstream, Source: "flag"}}, nil
	}

	var candidates []gatewayCandidate
	if address, err := found.Address(); err == nil {
		candidates = append(candidates, gatewayCandidate{Address: address, Source: "igd"})
	}

	if upstream != "" {
		return append(candidates, gatewayCandidate{Address: upstream, Source: "flag"}), nil
	}

	externalIP, err := found.ExternalIP(ctx)
	if err != nil {
		return candidates, nil
	}
	fmt.Printf("igd %s reports external %s\n\n", found.FriendlyName, externalIP)

	if next := upnp.GuessUpstream(externalIP); next != "" {
		candidates = append(candidates, gatewayCandidate{Address: next, Source: "guessed upstream"})
	}
	return candidates, nil
}
