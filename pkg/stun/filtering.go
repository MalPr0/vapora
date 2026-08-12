package stun

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"
)

// Filtering is which remote endpoints the NAT lets reach an existing mapping.
// It is what decides whether a peer can send the first packet.
type Filtering int

const (
	// FilteringUnknown is the zero value: not measured, which is not the same
	// as measured and open.
	FilteringUnknown Filtering = iota
	// FilteringEndpointIndependent accepts packets from anyone once the
	// mapping exists, so an unannounced peer gets through.
	FilteringEndpointIndependent
	// FilteringAddressDependent only accepts hosts already contacted.
	FilteringAddressDependent
	// FilteringAddressAndPortDependent only accepts the exact endpoint
	// already contacted, the strictest common case.
	FilteringAddressAndPortDependent
)

// String names the filtering in both vocabularies, as Mapping does.
func (f Filtering) String() string {
	switch f {
	case FilteringEndpointIndependent:
		return "endpoint-independent (full cone)"
	case FilteringAddressDependent:
		return "address-dependent (restricted cone)"
	case FilteringAddressAndPortDependent:
		return "address-and-port-dependent (port restricted cone)"
	default:
		return "unknown"
	}
}

// AcceptsUnknownPeers reports whether a peer that was never contacted can send
// the first packet into an existing mapping.
func (f Filtering) AcceptsUnknownPeers() bool {
	return f == FilteringEndpointIndependent
}

var errNoAlternateAddress = errors.New("stun: server does not advertise an alternate address")

// ProbeFiltering runs the RFC 5780 filtering tests against a server that
// advertises an alternate address, asking it to answer from a different IP and
// port to see what the NAT lets back in.
func ProbeFiltering(ctx context.Context, conn *net.UDPConn, server string, timeout time.Duration) (Filtering, error) {
	base, err := query(ctx, conn, server, timeout, queryOptions{})
	if err != nil {
		return FilteringUnknown, err
	}
	if base.Other == nil {
		return FilteringUnknown, fmt.Errorf("%w: %s", errNoAlternateAddress, server)
	}

	// Answer from another IP and port: only an endpoint-independent filter
	// lets that reach the mapping.
	_, err = query(ctx, conn, server, timeout, queryOptions{changeIP: true, changePort: true, acceptAnySource: true})
	if err == nil {
		return FilteringEndpointIndependent, nil
	}

	// Same IP, different port: getting through means the filter keys on the
	// address only.
	_, err = query(ctx, conn, server, timeout, queryOptions{changePort: true, acceptAnySource: true})
	if err == nil {
		return FilteringAddressDependent, nil
	}

	return FilteringAddressAndPortDependent, nil
}

// ProbeFilteringAny tries every server until one supports the tests.
func ProbeFilteringAny(ctx context.Context, conn *net.UDPConn, servers []string, timeout time.Duration) (Filtering, string, error) {
	var lastErr error
	for _, server := range servers {
		filtering, err := ProbeFiltering(ctx, conn, server, timeout)
		if err != nil {
			lastErr = err
			continue
		}
		return filtering, server, nil
	}
	if lastErr == nil {
		lastErr = errors.New("stun: no servers to probe")
	}
	return FilteringUnknown, "", fmt.Errorf("stun: no server supports the filtering tests: %w", lastErr)
}
