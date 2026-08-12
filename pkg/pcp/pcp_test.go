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

func TestPCPRequestHeaderLayout(t *testing.T) {
	request := pcpRequest(opcodeMap, 2*time.Hour, netip.MustParseAddr("192.168.1.20"), nil)

	if len(request) != pcpHeaderSize {
		t.Fatalf("header is %d bytes, want %d", len(request), pcpHeaderSize)
	}
	if request[0] != pcpVersion {
		t.Fatalf("version is %d", request[0])
	}
	if request[1] != opcodeMap {
		t.Fatalf("opcode byte is 0x%02x, the R bit must be clear on a request", request[1])
	}
	if got := binary.BigEndian.Uint32(request[4:8]); got != 7200 {
		t.Fatalf("lifetime is %d seconds", got)
	}

	// The client address travels as IPv4-mapped IPv6, per RFC 6887 section 7.1.
	want := netip.MustParseAddr("::ffff:192.168.1.20").As16()
	if [16]byte(request[8:24]) != want {
		t.Fatalf("client address is %v", request[8:24])
	}
}

// The MAP request must carry no FILTER option: RFC 6887 section 13.3 makes its
// absence mean endpoint-independent filtering, which is the point.
func TestMapPayloadCarriesNoFilterOption(t *testing.T) {
	nonce := [nonceSize]byte{1, 2, 3}
	payload := mapPayload(nonce, ProtocolUDP, 41001, 41001, netip.Addr{})

	if len(payload) != mapPayloadSize {
		t.Fatalf("payload is %d bytes, any extra would be an option", len(payload))
	}
	if [nonceSize]byte(payload[0:12]) != nonce {
		t.Fatal("nonce did not survive")
	}
	if payload[12] != byte(ProtocolUDP) {
		t.Fatalf("protocol is %d", payload[12])
	}
	if got := binary.BigEndian.Uint16(payload[16:18]); got != 41001 {
		t.Fatalf("internal port is %d", got)
	}
	if got := binary.BigEndian.Uint16(payload[18:20]); got != 41001 {
		t.Fatalf("suggested external port is %d", got)
	}
}

func TestParsePCPResponse(t *testing.T) {
	wire := make([]byte, pcpHeaderSize)
	wire[0] = pcpVersion
	wire[1] = pcpResponseR | opcodeMap
	wire[3] = resultNotAuthorized
	binary.BigEndian.PutUint32(wire[4:8], 3600)

	response, err := parsePCPResponse(wire)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if response.Opcode != opcodeMap || response.Result != resultNotAuthorized || response.Lifetime != time.Hour {
		t.Fatalf("got %+v", response)
	}
}

func TestParsePCPResponseRejectsJunk(t *testing.T) {
	short := make([]byte, 4)
	if _, err := parsePCPResponse(short); !errors.Is(err, ErrMalformedAnswer) {
		t.Fatalf("got %v", err)
	}

	request := make([]byte, pcpHeaderSize)
	request[0] = pcpVersion
	request[1] = opcodeMap // R bit clear: this is a request, not an answer
	if _, err := parsePCPResponse(request); !errors.Is(err, ErrMalformedAnswer) {
		t.Fatalf("got %v", err)
	}

	wrongVersion := make([]byte, pcpHeaderSize)
	wrongVersion[0] = 1
	if _, err := parsePCPResponse(wrongVersion); !errors.Is(err, ErrMalformedAnswer) {
		t.Fatalf("got %v", err)
	}
}

func TestParseMapPayloadUnmapsIPv4(t *testing.T) {
	payload := make([]byte, mapPayloadSize)
	payload[12] = byte(ProtocolUDP)
	binary.BigEndian.PutUint16(payload[16:18], 41001)
	binary.BigEndian.PutUint16(payload[18:20], 51001)
	copy(payload[20:36], netip.MustParseAddr("::ffff:203.0.113.7").AsSlice())

	result, err := parseMapPayload(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExternalPort != 51001 || result.ExternalIP.String() != "203.0.113.7" {
		t.Fatalf("got port %d ip %s", result.ExternalPort, result.ExternalIP)
	}
}

func TestNATPMPRequests(t *testing.T) {
	if got := natpmpAddressRequest(); len(got) != 2 || got[0] != 0 || got[1] != 0 {
		t.Fatalf("address request is %v", got)
	}

	request, err := natpmpMapRequest(ProtocolUDP, 41001, 41001, time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if request[1] != natpmpOpMapUDP {
		t.Fatalf("opcode is %d", request[1])
	}
	if got := binary.BigEndian.Uint32(request[8:12]); got != 3600 {
		t.Fatalf("lifetime is %d", got)
	}

	if _, err := natpmpMapRequest(Protocol(99), 1, 1, time.Hour); err == nil {
		t.Fatal("NAT-PMP only carries TCP and UDP")
	}
}

func TestParseNATPMPAddress(t *testing.T) {
	wire := make([]byte, natpmpAddressResponseSize)
	wire[1] = natpmpResponse + natpmpOpAddress
	copy(wire[8:12], []byte{203, 0, 113, 7})

	header, err := parseNATPMPResponse(wire)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if header.Opcode != natpmpOpAddress || header.Result != resultSuccess {
		t.Fatalf("got %+v", header)
	}

	address, err := parseNATPMPAddress(wire)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if address.String() != "203.0.113.7" {
		t.Fatalf("got %s", address)
	}
}

// fakeGateway answers with whatever the test scripted, or stays silent.
type fakeGateway struct {
	conn    *net.UDPConn
	answers [][]byte
	sent    int
}

func newFakeGateway(t *testing.T, answers ...[]byte) *fakeGateway {
	t.Helper()

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("cannot listen: %v", err)
	}

	gateway := &fakeGateway{conn: conn, answers: answers}
	go gateway.serve()
	t.Cleanup(func() { conn.Close() })
	return gateway
}

func (g *fakeGateway) serve() {
	buffer := make([]byte, 1500)
	for {
		_, from, err := g.conn.ReadFromUDP(buffer)
		if err != nil {
			return
		}
		if g.sent >= len(g.answers) {
			continue
		}
		_, _ = g.conn.WriteToUDP(g.answers[g.sent], from)
		g.sent++
	}
}

// dialFake points a client at the fake gateway, bypassing the fixed 5351 port.
func dialFake(t *testing.T, gateway *fakeGateway) *Client {
	t.Helper()

	target := gateway.conn.LocalAddr().(*net.UDPAddr)
	conn, err := net.DialUDP("udp4", nil, target)
	if err != nil {
		t.Fatalf("cannot dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	address, _ := netip.AddrFromSlice(target.IP)
	return &Client{
		conn:     conn,
		gateway:  netip.AddrPortFrom(address.Unmap(), uint16(target.Port)),
		internal: netip.MustParseAddr("127.0.0.1"),
	}
}

func TestDetectFindsPCP(t *testing.T) {
	answer := make([]byte, pcpHeaderSize)
	answer[0] = pcpVersion
	answer[1] = pcpResponseR | opcodeAnnounce

	client := dialFake(t, newFakeGateway(t, answer))
	version, err := client.Detect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if version != VersionPCP {
		t.Fatalf("got %s", version)
	}
}

// A NAT-PMP only gateway rejects the version 2 request, and that rejection is
// itself how the protocol is identified.
func TestDetectFallsBackToNATPMP(t *testing.T) {
	rejection := make([]byte, 8)
	rejection[1] = natpmpResponse
	binary.BigEndian.PutUint16(rejection[2:4], resultUnsuppVersion)

	address := make([]byte, natpmpAddressResponseSize)
	address[1] = natpmpResponse + natpmpOpAddress
	copy(address[8:12], []byte{10, 0, 0, 84})

	client := dialFake(t, newFakeGateway(t, rejection, address))
	version, err := client.Detect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if version != VersionNATPMP {
		t.Fatalf("got %s", version)
	}

	external, err := client.ExternalIP(context.Background())
	if err == nil && external.String() != "192.0.2.10" {
		t.Fatalf("got %s", external)
	}
}

func TestDetectReportsSilence(t *testing.T) {
	client := dialFake(t, newFakeGateway(t))

	start := time.Now()
	if _, err := client.Detect(context.Background()); !errors.Is(err, ErrNoAnswer) {
		t.Fatalf("got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("a silent gateway took %s to give up", elapsed)
	}
}

func TestDetectHonoursContextCancellation(t *testing.T) {
	client := dialFake(t, newFakeGateway(t))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := client.Detect(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}

func TestMapRejectsAForeignNonce(t *testing.T) {
	answer := make([]byte, pcpHeaderSize+mapPayloadSize)
	answer[0] = pcpVersion
	answer[1] = pcpResponseR | opcodeMap
	answer[pcpHeaderSize] = 0xFF // a nonce the request never sent

	client := dialFake(t, newFakeGateway(t, answer))
	client.version = VersionPCP

	_, err := client.Map(context.Background(), MapRequest{Protocol: ProtocolUDP, InternalPort: 41001, Lifetime: time.Hour})
	if !errors.Is(err, ErrWrongNonce) {
		t.Fatalf("got %v", err)
	}
}

func TestResultErrorNamesTheCode(t *testing.T) {
	err := &ResultError{Version: VersionPCP, Code: resultUnsuppProtocol}
	if got := err.Error(); got != "pcp: PCP gateway refused with UNSUPP_PROTOCOL (9)" {
		t.Fatalf("got %q", got)
	}
	if !IsUnsupportedVersion(&ResultError{Version: VersionNATPMP, Code: resultUnsuppVersion}) {
		t.Fatal("an unsupported version must be recognised")
	}
	if IsUnsupportedVersion(&ResultError{Code: resultNoResources}) {
		t.Fatal("no resources is not an unsupported version")
	}
}
