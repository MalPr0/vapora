package stun

import (
	"encoding/binary"
	"net"
	"testing"
)

func TestParseBindingResponseDecodesXORMappedAddress(t *testing.T) {
	transactionID := [12]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	response := buildResponse(t, transactionID, attribute{attrXORMappedAddress, xorAddress("203.0.113.7", 59506)})

	parsed, err := parseBindingResponse(response, transactionID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Mapped.String() != "203.0.113.7:59506" {
		t.Fatalf("got %s", parsed.Mapped)
	}
}

func TestParseBindingResponseReadsOtherAddress(t *testing.T) {
	transactionID := [12]byte{7}
	response := buildResponse(t, transactionID,
		attribute{attrXORMappedAddress, xorAddress("203.0.113.7", 59506)},
		attribute{attrOtherAddress, plainAddress("203.0.113.9", 3479)})

	parsed, err := parseBindingResponse(response, transactionID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Other == nil || parsed.Other.String() != "203.0.113.9:3479" {
		t.Fatalf("got %v", parsed.Other)
	}
}

func TestBuildBindingRequestCarriesChangeRequest(t *testing.T) {
	_, request, err := buildBindingRequest(queryOptions{changeIP: true, changePort: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := binary.BigEndian.Uint16(request[2:4]); got != 8 {
		t.Fatalf("body length is %d", got)
	}
	if got := binary.BigEndian.Uint16(request[20:22]); got != attrChangeRequest {
		t.Fatalf("attribute is 0x%04x", got)
	}
	if got := request[27]; got != changeIPFlag|changePortFlag {
		t.Fatalf("flags are 0x%02x", got)
	}

	_, plain, err := buildBindingRequest(queryOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plain) != headerSize {
		t.Fatalf("a plain request must carry no attributes, got %d bytes", len(plain))
	}
}

func TestFilteringAcceptsUnknownPeers(t *testing.T) {
	if !FilteringEndpointIndependent.AcceptsUnknownPeers() {
		t.Fatal("a full cone must accept unknown peers")
	}
	for _, filtering := range []Filtering{FilteringAddressDependent, FilteringAddressAndPortDependent, FilteringUnknown} {
		if filtering.AcceptsUnknownPeers() {
			t.Fatalf("%s must not accept unknown peers", filtering)
		}
	}
}

func TestParseBindingResponseRejectsForeignTransaction(t *testing.T) {
	response := buildResponse(t, [12]byte{9}, attribute{attrXORMappedAddress, xorAddress("192.0.2.1", 1234)})

	if _, err := parseBindingResponse(response, [12]byte{1}); err != errTransactionMismatch {
		t.Fatalf("got %v", err)
	}
}

func TestClassifyDetectsSymmetricNAT(t *testing.T) {
	independent := &Report{LocalPort: 5000, Observations: []Observation{
		{Mapped: &net.UDPAddr{IP: net.IPv4(203, 0, 113, 5), Port: 5000}},
		{Mapped: &net.UDPAddr{IP: net.IPv4(203, 0, 113, 5), Port: 5000}},
	}}
	independent.classify()
	if independent.Mapping != MappingEndpointIndependent || !independent.HolePunchViable || !independent.PortPreserved {
		t.Fatalf("got %s viable=%v preserved=%v", independent.Mapping, independent.HolePunchViable, independent.PortPreserved)
	}

	symmetric := &Report{LocalPort: 5000, Observations: []Observation{
		{Mapped: &net.UDPAddr{IP: net.IPv4(203, 0, 113, 5), Port: 5000}},
		{Mapped: &net.UDPAddr{IP: net.IPv4(203, 0, 113, 5), Port: 6001}},
	}}
	symmetric.classify()
	if symmetric.Mapping != MappingAddressDependent || symmetric.HolePunchViable {
		t.Fatalf("got %s viable=%v", symmetric.Mapping, symmetric.HolePunchViable)
	}

	if _, viable := symmetric.PublicEndpoint(); viable {
		t.Fatal("a symmetric NAT must not advertise a usable endpoint")
	}
}

type attribute struct {
	kind  uint16
	value []byte
}

func buildResponse(t *testing.T, transactionID [12]byte, attributes ...attribute) []byte {
	t.Helper()

	var body []byte
	for _, attr := range attributes {
		header := make([]byte, 4)
		binary.BigEndian.PutUint16(header[0:2], attr.kind)
		binary.BigEndian.PutUint16(header[2:4], uint16(len(attr.value)))
		body = append(body, header...)
		body = append(body, attr.value...)
	}

	response := make([]byte, headerSize)
	binary.BigEndian.PutUint16(response[0:2], bindingResponse)
	binary.BigEndian.PutUint16(response[2:4], uint16(len(body)))
	binary.BigEndian.PutUint32(response[4:8], magicCookie)
	copy(response[8:20], transactionID[:])
	return append(response, body...)
}

func xorAddress(address string, port uint16) []byte {
	value := plainAddress(address, port^(magicCookie>>16))

	var cookie [4]byte
	binary.BigEndian.PutUint32(cookie[:], magicCookie)
	for i := 0; i < 4; i++ {
		value[4+i] ^= cookie[i]
	}
	return value
}

func plainAddress(address string, port uint16) []byte {
	value := make([]byte, 8)
	value[1] = familyIPv4
	binary.BigEndian.PutUint16(value[2:4], port)
	copy(value[4:8], net.ParseIP(address).To4())
	return value
}
