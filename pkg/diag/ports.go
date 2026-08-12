// Package diag attributes NAT behaviour to a specific router. A STUN report
// only describes the whole chain end to end, so telling apart which of two
// cascaded NATs is filtering takes an experiment, not a measurement.
package diag

import (
	"context"
	"net"
	"time"

	"github.com/MalPr0/vapora/pkg/stun"
	"github.com/MalPr0/vapora/pkg/upnp"
)

// PortMapper is the slice of an IGD this experiment needs. It is declared here,
// on the consumer side, so the experiment can run against a fake gateway.
type PortMapper interface {
	AddPortMapping(ctx context.Context, protocol string, externalPort, internalPort int, description string, lease time.Duration) error
	GetPortMapping(ctx context.Context, protocol string, externalPort int) (*upnp.PortMapping, error)
	DeletePortMapping(ctx context.Context, protocol string, externalPort int) error
}

// FilterProbe measures RFC 5780 behaviour on a socket the caller owns.
type FilterProbe interface {
	Filtering(ctx context.Context, conn *net.UDPConn) (stun.Filtering, string, error)
	Endpoint(ctx context.Context, conn *net.UDPConn) (*net.UDPAddr, error)
}

// STUNProbe is the FilterProbe backed by internal/stun.
type STUNProbe struct {
	Servers []string
	Timeout time.Duration
}

// Filtering classifies one socket, and names the server that answered so two
// measurements can be checked for having asked the same question.
func (p STUNProbe) Filtering(ctx context.Context, conn *net.UDPConn) (stun.Filtering, string, error) {
	return stun.ProbeFilteringAny(ctx, conn, p.servers(), p.timeout())
}

// Endpoint reports where the world sees this socket, which the experiment
// records before and after: an address that moved on its own invalidates the
// comparison.
func (p STUNProbe) Endpoint(ctx context.Context, conn *net.UDPConn) (*net.UDPAddr, error) {
	endpoint, _, err := stun.FirstEndpoint(ctx, conn, p.servers(), p.timeout())
	return endpoint, err
}

func (p STUNProbe) servers() []string {
	if len(p.Servers) == 0 {
		return stun.DefaultServers
	}
	return p.Servers
}

func (p STUNProbe) timeout() time.Duration {
	if p.Timeout <= 0 {
		return 4 * time.Second
	}
	return p.Timeout
}
