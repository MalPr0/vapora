package upnp

import (
	"context"
	"fmt"
	"net"
	"time"
)

const maxHops = 3

var carrierGradeNAT = &net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}

// Hop is one gateway of the NAT chain with the mapping created on it.
type Hop struct {
	Gateway      *Gateway
	InternalHost string
	ExternalIP   string
}

// Chain holds every mapping created to reach this host from the internet. In a
// single NAT setup it has one hop; behind double NAT it maps each router in
// turn, each one pointing at the WAN address of the previous one.
type Chain struct {
	Hops     []Hop
	Protocol string
	Port     int

	// Reason explains why the chain stopped before reaching a public address.
	Reason string
}

// ChainOptions configures how far the search for routers goes, and where it
// starts. The defaults are enough for a home network; the upstream address
// matters when the first router's WAN side is itself behind another one.
type ChainOptions struct {
	Protocol         string
	Port             int
	Description      string
	Lease            time.Duration
	DiscoveryTimeout time.Duration
	// Upstream forces the address of the next router instead of guessing it.
	Upstream string
}

// OpenChain maps the port on every gateway between this host and the internet.
// The returned chain must always be closed, even on error, because hops may
// have been mapped before the failure.
func OpenChain(ctx context.Context, opts ChainOptions) (*Chain, error) {
	chain := &Chain{Protocol: opts.Protocol, Port: opts.Port}

	gateway, err := Discover(ctx, opts.DiscoveryTimeout)
	if err != nil {
		return chain, err
	}

	client := gateway.LocalAddress
	upstreamHint := opts.Upstream

	for hop := 0; hop < maxHops; hop++ {
		externalIP, err := gateway.ExternalIP(ctx)
		if err != nil {
			return chain, err
		}

		if err := gateway.AddPortMappingFor(ctx, opts.Protocol, opts.Port, opts.Port, client, opts.Description, opts.Lease); err != nil {
			return chain, fmt.Errorf("upnp: mapping on %s failed: %w", gateway.FriendlyName, err)
		}
		chain.Hops = append(chain.Hops, Hop{Gateway: gateway, InternalHost: client, ExternalIP: externalIP})

		if !isPrivate(externalIP) {
			if isCarrierGrade(externalIP) {
				chain.Reason = fmt.Sprintf("%s belongs to the carrier grade NAT range, your ISP has to open the port", externalIP)
			}
			return chain, nil
		}

		next := upstreamHint
		if next == "" {
			next = guessUpstream(externalIP)
		}
		upstreamHint = ""
		if next == "" {
			chain.Reason = fmt.Sprintf("external address %s is private and no upstream router could be derived from it", externalIP)
			return chain, nil
		}

		upstream, err := DiscoverAt(ctx, next, opts.DiscoveryTimeout)
		if err != nil {
			chain.Reason = fmt.Sprintf("upstream router %s does not answer UPnP (%v), forward %s/%d to %s there by hand", next, err, opts.Protocol, opts.Port, externalIP)
			return chain, nil
		}

		client = externalIP
		gateway = upstream
	}

	chain.Reason = fmt.Sprintf("gave up after %d NAT hops", maxHops)
	return chain, nil
}

// PublicAddress is the host:port a peer on the internet should dial, and false
// when the chain never reached a public address.
func (c *Chain) PublicAddress() (string, bool) {
	if len(c.Hops) == 0 {
		return "", false
	}
	last := c.Hops[len(c.Hops)-1].ExternalIP
	if isPrivate(last) || isCarrierGrade(last) {
		return net.JoinHostPort(last, fmt.Sprint(c.Port)), false
	}
	return net.JoinHostPort(last, fmt.Sprint(c.Port)), true
}

// Close removes every mapping, from the outermost gateway inwards.
func (c *Chain) Close(ctx context.Context) error {
	var firstErr error
	for i := len(c.Hops) - 1; i >= 0; i-- {
		if err := c.Hops[i].Gateway.DeletePortMapping(ctx, c.Protocol, c.Port); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	c.Hops = nil
	return firstErr
}

func isPrivate(address string) bool {
	ip := net.ParseIP(address)
	return ip != nil && (ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified())
}

func isCarrierGrade(address string) bool {
	ip := net.ParseIP(address)
	return ip != nil && carrierGradeNAT.Contains(ip)
}

// GuessUpstream assumes the upstream router owns the .1 address of the subnet
// the current gateway got its WAN address from, which is the usual layout of a
// modem plus router cascade.
func GuessUpstream(externalIP string) string {
	return guessUpstream(externalIP)
}

func guessUpstream(externalIP string) string {
	ip := net.ParseIP(externalIP).To4()
	if ip == nil {
		return ""
	}
	candidate := net.IPv4(ip[0], ip[1], ip[2], 1)
	if candidate.Equal(ip) {
		return ""
	}
	return candidate.String()
}
