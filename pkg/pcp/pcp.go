package pcp

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"time"
)

const (
	pcpVersion    = 2
	pcpHeaderSize = 24
	pcpResponseR  = 0x80

	opcodeAnnounce = 0
	opcodeMap      = 1

	mapPayloadSize = 36
	nonceSize      = 12
)

// pcpRequest builds the 24 byte header every PCP request shares. The client
// address is the internal address of this host as the gateway sees it, which
// is how the gateway spots a request that crossed a NAT it does not know about.
func pcpRequest(opcode byte, lifetime time.Duration, client netip.Addr, payload []byte) []byte {
	request := make([]byte, pcpHeaderSize, pcpHeaderSize+len(payload))
	request[0] = pcpVersion
	request[1] = opcode
	binary.BigEndian.PutUint32(request[4:8], uint32(lifetime.Seconds()))

	mapped := netip.AddrFrom16(client.As16())
	copy(request[8:24], mapped.AsSlice())

	return append(request, payload...)
}

type pcpResponse struct {
	Opcode   byte
	Result   int
	Lifetime time.Duration
	Epoch    uint32
	Payload  []byte
}

func parsePCPResponse(wire []byte) (*pcpResponse, error) {
	if len(wire) < pcpHeaderSize {
		return nil, fmt.Errorf("%w: %d bytes is shorter than a PCP header", ErrMalformedAnswer, len(wire))
	}
	if wire[0] != pcpVersion {
		return nil, fmt.Errorf("%w: version %d is not PCP", ErrMalformedAnswer, wire[0])
	}
	if wire[1]&pcpResponseR == 0 {
		return nil, fmt.Errorf("%w: the R bit says this is a request", ErrMalformedAnswer)
	}

	return &pcpResponse{
		Opcode:   wire[1] &^ pcpResponseR,
		Result:   int(wire[3]),
		Lifetime: time.Duration(binary.BigEndian.Uint32(wire[4:8])) * time.Second,
		Epoch:    binary.BigEndian.Uint32(wire[8:12]),
		Payload:  wire[pcpHeaderSize:],
	}, nil
}

// mapPayload is the MAP opcode body. It carries no FILTER option on purpose:
// RFC 6887 section 13.3 says that in its absence the mapping uses endpoint
// independent filtering, which is the whole reason this package exists.
func mapPayload(nonce [nonceSize]byte, protocol Protocol, internalPort, suggestedPort uint16, suggestedIP netip.Addr) []byte {
	payload := make([]byte, mapPayloadSize)
	copy(payload[0:12], nonce[:])
	payload[12] = byte(protocol)
	binary.BigEndian.PutUint16(payload[16:18], internalPort)
	binary.BigEndian.PutUint16(payload[18:20], suggestedPort)

	if suggestedIP.IsValid() {
		copy(payload[20:36], netip.AddrFrom16(suggestedIP.As16()).AsSlice())
	}
	return payload
}

type mapResult struct {
	Nonce        [nonceSize]byte
	Protocol     Protocol
	InternalPort uint16
	ExternalPort uint16
	ExternalIP   netip.Addr
}

func parseMapPayload(payload []byte) (*mapResult, error) {
	if len(payload) < mapPayloadSize {
		return nil, fmt.Errorf("%w: MAP payload is %d bytes", ErrMalformedAnswer, len(payload))
	}

	result := &mapResult{
		Protocol:     Protocol(payload[12]),
		InternalPort: binary.BigEndian.Uint16(payload[16:18]),
		ExternalPort: binary.BigEndian.Uint16(payload[18:20]),
	}
	copy(result.Nonce[:], payload[0:12])

	var raw [16]byte
	copy(raw[:], payload[20:36])
	address := netip.AddrFrom16(raw)
	if address.Is4In6() {
		address = address.Unmap()
	}
	result.ExternalIP = address

	return result, nil
}
