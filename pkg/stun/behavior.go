package stun

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"
)

// DefaultServers are public STUN servers on distinct networks, which is what
// makes the mapping comparison meaningful.
var DefaultServers = []string{
	"stun.l.google.com:19302",
	"stun.cloudflare.com:3478",
	"stun.nextcloud.com:443",
	"stun.sipgate.net:3478",
}

// FirstEndpoint returns the public endpoint as soon as any server answers,
// which is all a peer needs before it can hand out an invite.
func FirstEndpoint(ctx context.Context, conn *net.UDPConn, servers []string, timeout time.Duration) (*net.UDPAddr, string, error) {
	var lastErr error
	for _, server := range servers {
		mapped, err := Query(ctx, conn, server, timeout)
		if err != nil {
			lastErr = err
			continue
		}
		return mapped, server, nil
	}
	if lastErr == nil {
		lastErr = errors.New("stun: no servers to query")
	}
	return nil, "", fmt.Errorf("stun: no server answered: %w", lastErr)
}

// Mapping is how the NAT allocates the external port of a socket.
type Mapping int

const (
	// MappingUnknown is the zero value: not measured, which is not the same as
	// measured and inconclusive.
	MappingUnknown Mapping = iota
	// MappingEndpointIndependent keeps one external port per local socket no
	// matter the destination. Hole punching works.
	MappingEndpointIndependent
	// MappingAddressDependent allocates a new external port per destination,
	// the classic symmetric NAT. Hole punching needs a relay instead.
	MappingAddressDependent
)

// String names the behaviour in both vocabularies — the RFC's and the one
// everybody actually says out loud — because the two rarely appear together
// and readers arrive knowing one or the other.
func (m Mapping) String() string {
	switch m {
	case MappingEndpointIndependent:
		return "endpoint-independent (cone)"
	case MappingAddressDependent:
		return "address-dependent (symmetric)"
	default:
		return "unknown"
	}
}

// Observation is one server's answer: where it says this socket is, or why it
// could not say. Failures are kept because agreement between servers is the
// evidence, and a server that stayed silent is not a server that disagreed.
type Observation struct {
	Server   string
	ServerIP string
	Mapped   *net.UDPAddr
	Err      error
}

// Report is everything a round of probing found: what each server saw, and
// the two classifications that follow from it.
//
// Mapping and Filtering answer different questions and only one of them can be
// changed from this side. Mapping is whether your address stays the same for
// every destination; filtering is who is allowed to send you a first packet.
type Report struct {
	LocalPort       int
	Observations    []Observation
	Mapping         Mapping
	Filtering       Filtering
	FilteringServer string
	PortPreserved   bool
	HolePunchViable bool
}

// Probe queries every server from a single UDP socket and classifies the NAT.
func Probe(ctx context.Context, servers []string, timeout time.Duration) (*Report, error) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{})
	if err != nil {
		return nil, fmt.Errorf("stun: cannot open local socket: %w", err)
	}
	defer conn.Close()

	return ProbeWith(ctx, conn, servers, timeout)
}

// ProbeWith runs the probe on an existing socket, so a caller can keep using
// the very same mapping the report describes.
func ProbeWith(ctx context.Context, conn *net.UDPConn, servers []string, timeout time.Duration) (*Report, error) {
	local, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return nil, fmt.Errorf("stun: unexpected local address type %T", conn.LocalAddr())
	}
	report := &Report{LocalPort: local.Port}

	for _, server := range servers {
		observation := Observation{Server: server}
		if resolved, err := net.ResolveUDPAddr("udp4", server); err == nil {
			observation.ServerIP = resolved.IP.String()
		}
		observation.Mapped, observation.Err = Query(ctx, conn, server, timeout)
		report.Observations = append(report.Observations, observation)
	}

	report.classify()
	report.Filtering, report.FilteringServer, _ = ProbeFilteringAny(ctx, conn, servers, timeout)
	return report, nil
}

func (r *Report) classify() {
	var mapped []*net.UDPAddr
	for _, observation := range r.Observations {
		if observation.Err == nil && observation.Mapped != nil {
			mapped = append(mapped, observation.Mapped)
		}
	}
	if len(mapped) < 2 {
		r.Mapping = MappingUnknown
		return
	}

	r.PortPreserved = mapped[0].Port == r.LocalPort
	for _, address := range mapped[1:] {
		if address.Port != mapped[0].Port || !address.IP.Equal(mapped[0].IP) {
			r.Mapping = MappingAddressDependent
			return
		}
	}
	r.Mapping = MappingEndpointIndependent
	r.HolePunchViable = true
}

// PublicEndpoint is the endpoint a peer should send packets to, valid only when
// the mapping is endpoint-independent.
func (r *Report) PublicEndpoint() (*net.UDPAddr, bool) {
	for _, observation := range r.Observations {
		if observation.Err == nil && observation.Mapped != nil {
			return observation.Mapped, r.Mapping == MappingEndpointIndependent
		}
	}
	return nil, false
}
