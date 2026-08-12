package pcp

import (
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"net/netip"
	"time"
)

// ServerPort is where both protocols listen.
const ServerPort = 5351

// attempts is deliberately faster than the 3 second first timeout RFC 6887
// section 8.1.1 prescribes. The expected outcome here is silence from a gateway
// that speaks neither protocol, and this runs in front of a waiting user.
var attempts = []time.Duration{250 * time.Millisecond, 750 * time.Millisecond, 2 * time.Second}

// Client talks to one gateway, falling back to NAT-PMP when the gateway answers
// a version 2 request with a version 0 error, which is what RFC 6887 section 9
// prescribes for a legacy server.
type Client struct {
	conn     *net.UDPConn
	gateway  netip.AddrPort
	internal netip.Addr
	version  Version
}

// Dial prepares to talk to a gateway. It opens a socket and works out which
// local address reaches it, which is the address a mapping has to name.
//
// Nothing is sent yet: call Detect to find out whether anybody is listening,
// and which of the two protocols they speak.
func Dial(gateway netip.Addr) (*Client, error) {
	target := netip.AddrPortFrom(gateway, ServerPort)

	conn, err := net.DialUDP("udp4", nil, net.UDPAddrFromAddrPort(target))
	if err != nil {
		return nil, fmt.Errorf("pcp: cannot reach %s: %w", target, err)
	}

	local, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		conn.Close()
		return nil, fmt.Errorf("pcp: unexpected local address type %T", conn.LocalAddr())
	}
	internal, _ := netip.AddrFromSlice(local.IP)

	return &Client{conn: conn, gateway: target, internal: internal.Unmap()}, nil
}

// Close releases the socket.
func (c *Client) Close() error { return c.conn.Close() }

// Gateway is the router being asked.
func (c *Client) Gateway() netip.AddrPort { return c.gateway }

// Internal is this machine's address on the route to that gateway, which is
// the one a mapping is installed for.
func (c *Client) Internal() netip.Addr { return c.internal }

// Version is what Detect found, and VersionUnknown before it has run.
func (c *Client) Version() Version { return c.version }

// Detect asks the gateway which protocol it speaks, if any.
func (c *Client) Detect(ctx context.Context) (Version, error) {
	wire, err := c.exchange(ctx, pcpRequest(opcodeAnnounce, 0, c.internal, nil))
	if err != nil {
		return VersionUnknown, err
	}

	if response, err := parsePCPResponse(wire); err == nil {
		if response.Opcode != opcodeAnnounce {
			return VersionUnknown, fmt.Errorf("%w: asked ANNOUNCE, got opcode %d", ErrWrongOpcode, response.Opcode)
		}
		if response.Result != resultSuccess {
			return VersionUnknown, &ResultError{Version: VersionPCP, Code: response.Result}
		}
		c.version = VersionPCP
		return VersionPCP, nil
	}

	// A NAT-PMP only gateway answers a version 2 request with version 0 and
	// UNSUPP_VERSION, so the rejection is itself the detection.
	header, err := parseNATPMPResponse(wire)
	if err != nil {
		return VersionUnknown, fmt.Errorf("%w: %w", ErrNotSupported, err)
	}
	if header.Result != resultUnsuppVersion && header.Result != resultSuccess {
		return VersionUnknown, &ResultError{Version: VersionNATPMP, Code: header.Result}
	}

	if err := c.confirmNATPMP(ctx); err != nil {
		return VersionUnknown, err
	}
	c.version = VersionNATPMP
	return VersionNATPMP, nil
}

func (c *Client) confirmNATPMP(ctx context.Context) error {
	wire, err := c.exchange(ctx, natpmpAddressRequest())
	if err != nil {
		return err
	}

	header, err := parseNATPMPResponse(wire)
	if err != nil {
		return err
	}
	if header.Opcode != natpmpOpAddress {
		return fmt.Errorf("%w: asked for the public address, got opcode %d", ErrWrongOpcode, header.Opcode)
	}
	if header.Result != resultSuccess {
		return &ResultError{Version: VersionNATPMP, Code: header.Result}
	}
	return nil
}

// ExternalIP is the address the gateway believes it owns on its WAN side.
func (c *Client) ExternalIP(ctx context.Context) (netip.Addr, error) {
	if c.version == VersionNATPMP {
		wire, err := c.exchange(ctx, natpmpAddressRequest())
		if err != nil {
			return netip.Addr{}, err
		}
		if _, err := parseNATPMPResponse(wire); err != nil {
			return netip.Addr{}, err
		}
		return parseNATPMPAddress(wire)
	}

	// PCP has no dedicated action: a zero lifetime MAP is the probe that
	// reports the external address without leaving state behind.
	mapping, err := c.Map(ctx, MapRequest{Protocol: ProtocolUDP, InternalPort: 9, Lifetime: 0})
	if err != nil {
		return netip.Addr{}, err
	}
	return mapping.ExternalIP, nil
}

// MapRequest asks for a door. A suggested external port is a suggestion: the
// gateway is free to hand back a different one, and what it returns is the
// only port worth telling anybody about.
type MapRequest struct {
	Protocol              Protocol
	InternalPort          uint16
	SuggestedExternalPort uint16
	SuggestedExternalIP   netip.Addr
	Lifetime              time.Duration
	// Nonce identifies the mapping across renewals. A zero value is filled
	// from crypto/rand; a renewal MUST reuse the nonce of the original.
	Nonce [nonceSize]byte
}

// Mapping is what the gateway actually granted, which regularly differs from
// what was asked for — a shorter lease, another port, or both.
type Mapping struct {
	Version      Version
	Gateway      netip.AddrPort
	Protocol     Protocol
	InternalPort uint16
	ExternalPort uint16
	ExternalIP   netip.Addr
	Lifetime     time.Duration
	Nonce        [nonceSize]byte
	Created      time.Time
}

// RenewAt is halfway through the lifetime, as RFC 6887 section 11.2.1 asks.
func (m *Mapping) RenewAt() time.Time {
	return m.Created.Add(m.Lifetime / 2)
}

// Map asks the gateway to open a port, in whichever protocol it speaks.
//
// A lease is not a pinhole: it governs this router only. Anything further out
// still expires its binding on inactivity, so a mapping does not remove the
// need to keep sending.
func (c *Client) Map(ctx context.Context, request MapRequest) (*Mapping, error) {
	if request.Nonce == ([nonceSize]byte{}) {
		if _, err := rand.Read(request.Nonce[:]); err != nil {
			return nil, fmt.Errorf("pcp: cannot generate a mapping nonce: %w", err)
		}
	}
	if request.SuggestedExternalPort == 0 {
		request.SuggestedExternalPort = request.InternalPort
	}

	if c.version == VersionNATPMP {
		return c.mapNATPMP(ctx, request)
	}
	return c.mapPCP(ctx, request)
}

func (c *Client) mapPCP(ctx context.Context, request MapRequest) (*Mapping, error) {
	payload := mapPayload(request.Nonce, request.Protocol, request.InternalPort, request.SuggestedExternalPort, request.SuggestedExternalIP)

	wire, err := c.exchange(ctx, pcpRequest(opcodeMap, request.Lifetime, c.internal, payload))
	if err != nil {
		return nil, err
	}

	response, err := parsePCPResponse(wire)
	if err != nil {
		return nil, err
	}
	if response.Opcode != opcodeMap {
		return nil, fmt.Errorf("%w: asked MAP, got opcode %d", ErrWrongOpcode, response.Opcode)
	}
	if response.Result != resultSuccess {
		return nil, &ResultError{Version: VersionPCP, Code: response.Result}
	}

	result, err := parseMapPayload(response.Payload)
	if err != nil {
		return nil, err
	}
	if result.Nonce != request.Nonce {
		return nil, ErrWrongNonce
	}

	return &Mapping{
		Version:      VersionPCP,
		Gateway:      c.gateway,
		Protocol:     request.Protocol,
		InternalPort: result.InternalPort,
		ExternalPort: result.ExternalPort,
		ExternalIP:   result.ExternalIP,
		Lifetime:     response.Lifetime,
		Nonce:        request.Nonce,
		Created:      time.Now(),
	}, nil
}

func (c *Client) mapNATPMP(ctx context.Context, request MapRequest) (*Mapping, error) {
	payload, err := natpmpMapRequest(request.Protocol, request.InternalPort, request.SuggestedExternalPort, request.Lifetime)
	if err != nil {
		return nil, err
	}

	wire, err := c.exchange(ctx, payload)
	if err != nil {
		return nil, err
	}

	header, err := parseNATPMPResponse(wire)
	if err != nil {
		return nil, err
	}
	if expected, _ := natpmpOpcode(request.Protocol); header.Opcode != expected {
		return nil, fmt.Errorf("%w: asked opcode %d, got %d", ErrWrongOpcode, expected, header.Opcode)
	}
	if header.Result != resultSuccess {
		return nil, &ResultError{Version: VersionNATPMP, Code: header.Result}
	}

	result, err := parseNATPMPMap(wire)
	if err != nil {
		return nil, err
	}

	external, err := c.ExternalIP(ctx)
	if err != nil {
		external = netip.Addr{}
	}

	return &Mapping{
		Version:      VersionNATPMP,
		Gateway:      c.gateway,
		Protocol:     request.Protocol,
		InternalPort: result.InternalPort,
		ExternalPort: result.ExternalPort,
		ExternalIP:   external,
		Lifetime:     result.Lifetime,
		Nonce:        request.Nonce,
		Created:      time.Now(),
	}, nil
}

// Refresh renews a mapping in place. Reusing the nonce is what makes the
// gateway extend the existing mapping instead of creating another one.
func (c *Client) Refresh(ctx context.Context, mapping *Mapping) (*Mapping, error) {
	return c.Map(ctx, MapRequest{
		Protocol:              mapping.Protocol,
		InternalPort:          mapping.InternalPort,
		SuggestedExternalPort: mapping.ExternalPort,
		SuggestedExternalIP:   mapping.ExternalIP,
		Lifetime:              mapping.Lifetime,
		Nonce:                 mapping.Nonce,
	})
}

// Unmap releases a mapping by asking for a zero lifetime.
func (c *Client) Unmap(ctx context.Context, mapping *Mapping) error {
	_, err := c.Map(ctx, MapRequest{
		Protocol:              mapping.Protocol,
		InternalPort:          mapping.InternalPort,
		SuggestedExternalPort: mapping.ExternalPort,
		Lifetime:              0,
		Nonce:                 mapping.Nonce,
	})
	return err
}

func (c *Client) exchange(ctx context.Context, request []byte) ([]byte, error) {
	buffer := make([]byte, 1100)

	for _, wait := range attempts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if _, err := c.conn.Write(request); err != nil {
			return nil, fmt.Errorf("pcp: cannot send to %s: %w", c.gateway, err)
		}

		deadline := time.Now().Add(wait)
		if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
			deadline = ctxDeadline
		}
		if err := c.conn.SetReadDeadline(deadline); err != nil {
			return nil, fmt.Errorf("pcp: cannot set deadline: %w", err)
		}

		n, err := c.conn.Read(buffer)
		if err == nil {
			answer := make([]byte, n)
			copy(answer, buffer[:n])
			return answer, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrNoAnswer, c.gateway)
}
