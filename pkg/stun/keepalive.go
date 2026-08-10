package stun

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"
)

// DefaultKeepalive sits below the low end of the UDP idle timeouts seen in the
// wild. RFC 4787 REQ-5 only demands two minutes, but consumer gateways are
// routinely far shorter, and an expired binding silently invalidates an
// endpoint that was already handed out.
const DefaultKeepalive = 25 * time.Second

// Keepalive refreshes the NAT binding of a socket that has nobody to talk to
// yet. A socket waiting for a peer sends nothing, so the outermost NAT drops
// its mapping on inactivity and the endpoint already published stops existing.
//
// It deliberately never reads: the caller owns the only reader of this socket,
// and a second one would steal its datagrams. Refreshing a binding only needs
// the packet to leave, so the answer is left for the owner to discard.
func Keepalive(ctx context.Context, conn *net.UDPConn, servers []string, every time.Duration) error {
	if len(servers) == 0 {
		return errors.New("stun: keepalive needs at least one server")
	}
	if every <= 0 {
		every = DefaultKeepalive
	}

	ticker := time.NewTicker(every)
	defer ticker.Stop()

	for attempt := 0; ; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}

		// Rotating spreads the load instead of hammering one server, and it
		// keeps a single unreachable server from stalling every refresh.
		if err := sendBindingRequest(conn, servers[attempt%len(servers)]); err != nil {
			continue
		}
	}
}

func sendBindingRequest(conn *net.UDPConn, server string) error {
	target, err := net.ResolveUDPAddr("udp4", server)
	if err != nil {
		return fmt.Errorf("stun: cannot resolve %s: %w", server, err)
	}

	_, request, err := buildBindingRequest(queryOptions{})
	if err != nil {
		return err
	}
	if _, err := conn.WriteToUDP(request, target); err != nil {
		return fmt.Errorf("stun: cannot send keepalive to %s: %w", server, err)
	}
	return nil
}
