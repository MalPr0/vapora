package pcp

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"net/netip"
	"testing"
	"time"
)

// A gateway that answers however the test says. Routers are the only thing
// these tests cannot have, and they are also the thing that behaves worst:
// answering the wrong opcode, granting a different port, or claiming a lease
// nobody asked for.
type gateway struct {
	conn   *net.UDPConn
	answer func(request []byte) []byte
}

func stand(t *testing.T, srv *gateway) netip.AddrPort {
	t.Helper()

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	srv.conn = conn

	go func() {
		buffer := make([]byte, 2048)
		for {
			read, from, err := conn.ReadFromUDP(buffer)
			if err != nil {
				return
			}
			if reply := srv.answer(append([]byte(nil), buffer[:read]...)); reply != nil {
				_, _ = conn.WriteToUDP(reply, from)
			}
		}
	}()

	return conn.LocalAddr().(*net.UDPAddr).AddrPort()
}

// dialTo points a client at a test gateway.
//
// Dial connects its socket to the well known port, so the address cannot be
// changed afterwards — writes would keep going to 5351 whatever the field
// says. The client is built directly instead, which is the same thing Dial
// does with a port this test can choose.
func dialTo(t *testing.T, at netip.AddrPort) *Client {
	t.Helper()

	conn, err := net.DialUDP("udp4", nil, net.UDPAddrFromAddrPort(at))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })

	local := conn.LocalAddr().(*net.UDPAddr)
	internal, _ := netip.AddrFromSlice(local.IP)

	return &Client{conn: conn, gateway: at, internal: internal.Unmap()}
}

// refusePCP is how a NAT-PMP only gateway answers a version 2 request: with
// version 0 and UNSUPP_VERSION. That rejection is itself the detection, which
// is why silence means "nothing here" rather than "try the older one".
func refusePCP() []byte {
	// Eight bytes: version, opcode, result, and the epoch a real one carries.
	reply := make([]byte, 8)
	reply[0] = 0
	reply[1] = 128
	binary.BigEndian.PutUint16(reply[2:4], 1) // UNSUPP_VERSION
	return reply
}

// natpmpReply builds an answer to a NAT-PMP request, echoing its opcode.
func natpmpReply(opcode byte, result uint16, payload []byte) []byte {
	// The header is eight bytes: version, opcode, result, and the epoch. The
	// payload that follows differs per opcode.
	reply := make([]byte, 8, 8+len(payload))
	reply[0] = 0 // NAT-PMP version
	reply[1] = opcode + 128
	binary.BigEndian.PutUint16(reply[2:4], result)
	return append(reply, payload...)
}

func TestDialWorksOutTheAddressThatReachesTheGateway(t *testing.T) {
	client, err := Dial(netip.MustParseAddr("192.0.2.1"))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	if !client.Internal().IsValid() {
		t.Fatal("no local address was worked out")
	}
	if client.Gateway().Addr().String() != "192.0.2.1" {
		t.Fatalf("gateway is %s", client.Gateway())
	}
	if client.Version() != VersionUnknown {
		t.Fatalf("a version was claimed before anything was asked: %s", client.Version())
	}
}

// Silence is not the same as "does not speak this". A gateway that says
// nothing might not be there at all, so it comes back as a failed exchange
// rather than a verdict about what it supports — and, either way, promptly.
func TestASilentGatewayGivesUpQuickly(t *testing.T) {
	at := stand(t, &gateway{answer: func([]byte) []byte { return nil }})
	client := dialTo(t, at)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	started := time.Now()
	version, err := client.Detect(ctx)
	if err == nil {
		t.Fatalf("silence was read as %s", version)
	}
	if elapsed := time.Since(started); elapsed > 4*time.Second {
		t.Fatalf("giving up took %s", elapsed)
	}
}

// A gateway that answers with something that is neither protocol is a device
// that does not speak this, and says so as ErrNotSupported.
func TestGibberishIsNotSupported(t *testing.T) {
	at := stand(t, &gateway{answer: func([]byte) []byte {
		return []byte("hello there")
	}})
	client := dialTo(t, at)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if _, err := client.Detect(ctx); !errors.Is(err, ErrNotSupported) {
		t.Fatalf("gibberish gave %v", err)
	}
}

// PCP is tried first; a gateway that only knows the older protocol says so by
// refusing the version, and the client has to fall back rather than give up.
func TestNATPMPIsUsedWhenPCPIsRefused(t *testing.T) {
	at := stand(t, &gateway{answer: func(request []byte) []byte {
		if request[0] == 2 { // a PCP request
			return refusePCP()
		}
		// NAT-PMP: answer the external address query.
		return natpmpReply(0, 0, []byte{203, 0, 113, 7})
	}})

	client := dialTo(t, at)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	version, err := client.Detect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if version != VersionNATPMP {
		t.Fatalf("detected %s, want NAT-PMP", version)
	}
	if client.Version() != VersionNATPMP {
		t.Fatal("the client did not remember what it detected")
	}
}

func TestExternalIPIsReadOverNATPMP(t *testing.T) {
	at := stand(t, &gateway{answer: func(request []byte) []byte {
		if request[0] == 2 {
			return refusePCP()
		}
		return natpmpReply(0, 0, []byte{203, 0, 113, 7})
	}})

	client := dialTo(t, at)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if _, err := client.Detect(ctx); err != nil {
		t.Fatal(err)
	}

	address, err := client.ExternalIP(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if address.String() != "203.0.113.7" {
		t.Fatalf("read %s", address)
	}
}

// A gateway is free to grant a different port and a shorter lease than asked
// for, and what it granted is the only thing worth telling anybody about.
func TestWhatTheGatewayGrantedIsWhatComesBack(t *testing.T) {
	at := stand(t, &gateway{answer: func(request []byte) []byte {
		if request[0] == 2 {
			return refusePCP()
		}
		if request[1] == 0 { // the address query during detection
			return natpmpReply(0, 0, []byte{203, 0, 113, 7})
		}

		// A mapping response: internal port echoed, external port and lease
		// chosen by the gateway rather than by us.
		payload := make([]byte, 8)
		copy(payload[0:2], request[4:6]) // the internal port we asked for
		binary.BigEndian.PutUint16(payload[2:4], 51002)
		binary.BigEndian.PutUint32(payload[4:8], 600)
		return natpmpReply(request[1], 0, payload)
	}})

	client := dialTo(t, at)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if _, err := client.Detect(ctx); err != nil {
		t.Fatal(err)
	}

	mapping, err := client.Map(ctx, MapRequest{
		Protocol:              ProtocolUDP,
		InternalPort:          41000,
		SuggestedExternalPort: 41000,
		Lifetime:              time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}

	if mapping.ExternalPort != 51002 {
		t.Fatalf("external port %d, want what the gateway granted", mapping.ExternalPort)
	}
	if mapping.Lifetime != 600*time.Second {
		t.Fatalf("lifetime %s, want the granted 600s", mapping.Lifetime)
	}

	// A renewal has to fall well inside the lease, or the mapping expires
	// between refreshes and the door shuts without anybody noticing.
	renew := mapping.RenewAt()
	if !renew.After(mapping.Created) || !renew.Before(mapping.Created.Add(mapping.Lifetime)) {
		t.Fatalf("renewal at %s is not inside a %s lease starting %s",
			renew, mapping.Lifetime, mapping.Created)
	}
}

// A gateway that answers a different question has to be caught, or a mapping
// gets read out of a reply about something else.
func TestAnAnswerToAnotherQuestionIsRefused(t *testing.T) {
	at := stand(t, &gateway{answer: func(request []byte) []byte {
		if request[0] == 2 {
			return refusePCP()
		}
		// Always answers as if asked for the external address.
		return natpmpReply(0, 0, []byte{203, 0, 113, 7})
	}})

	client := dialTo(t, at)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if _, err := client.Detect(ctx); err != nil {
		t.Fatal(err)
	}

	_, err := client.Map(ctx, MapRequest{
		Protocol:              ProtocolUDP,
		InternalPort:          41000,
		SuggestedExternalPort: 41000,
		Lifetime:              time.Hour,
	})
	if err == nil {
		t.Fatal("an answer about something else was read as a mapping")
	}
	if !errors.Is(err, ErrWrongOpcode) && !errors.Is(err, ErrMalformedAnswer) {
		t.Fatalf("got %v", err)
	}
}

// Truncated answers are what a half-working router actually sends.
func TestTruncatedAnswersAreRefused(t *testing.T) {
	for _, short := range [][]byte{
		{},
		{0},
		{0, 128},
		{0, 129, 0, 0},  // a mapping response with no payload
		make([]byte, 8), // half of one
	} {
		if _, err := parseNATPMPMap(short); err == nil {
			t.Fatalf("%v was accepted as a mapping", short)
		}
	}

	// And a header shorter than a header.
	for _, short := range [][]byte{{}, {0}, {0, 128}} {
		if _, err := parseNATPMPResponse(short); err == nil {
			t.Fatalf("%v was accepted as a response header", short)
		}
	}
}

func TestNamesRenderForPeople(t *testing.T) {
	if VersionPCP.String() == "" || VersionNATPMP.String() == "" || VersionUnknown.String() == "" {
		t.Fatal("a version renders as nothing")
	}
	if ProtocolUDP.String() != "UDP" || ProtocolTCP.String() != "TCP" {
		t.Fatalf("protocols render as %q and %q", ProtocolUDP, ProtocolTCP)
	}

	refusal := &ResultError{Version: VersionPCP, Code: 2}
	if refusal.Error() == "" {
		t.Fatal("a refusal renders as nothing")
	}
}
