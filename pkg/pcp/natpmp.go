package pcp

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"time"
)

const (
	natpmpVersion  = 0
	natpmpResponse = 128

	natpmpOpAddress = 0
	natpmpOpMapUDP  = 1
	natpmpOpMapTCP  = 2

	natpmpAddressResponseSize = 12
	natpmpMapResponseSize     = 16
)

func natpmpAddressRequest() []byte {
	return []byte{natpmpVersion, natpmpOpAddress}
}

func natpmpMapRequest(protocol Protocol, internalPort, suggestedPort uint16, lifetime time.Duration) ([]byte, error) {
	opcode, err := natpmpOpcode(protocol)
	if err != nil {
		return nil, err
	}

	request := make([]byte, 12)
	request[0] = natpmpVersion
	request[1] = opcode
	binary.BigEndian.PutUint16(request[4:6], internalPort)
	binary.BigEndian.PutUint16(request[6:8], suggestedPort)
	binary.BigEndian.PutUint32(request[8:12], uint32(lifetime.Seconds()))
	return request, nil
}

func natpmpOpcode(protocol Protocol) (byte, error) {
	switch protocol {
	case ProtocolUDP:
		return natpmpOpMapUDP, nil
	case ProtocolTCP:
		return natpmpOpMapTCP, nil
	default:
		return 0, fmt.Errorf("pcp: NAT-PMP does not carry %s", protocol)
	}
}

type natpmpResponseHeader struct {
	Opcode byte
	Result int
	Epoch  uint32
	Body   []byte
}

func parseNATPMPResponse(wire []byte) (*natpmpResponseHeader, error) {
	if len(wire) < 8 {
		return nil, fmt.Errorf("%w: %d bytes is shorter than a NAT-PMP header", ErrMalformedAnswer, len(wire))
	}
	if wire[0] != natpmpVersion {
		return nil, fmt.Errorf("%w: version %d is not NAT-PMP", ErrMalformedAnswer, wire[0])
	}
	if wire[1] < natpmpResponse {
		return nil, fmt.Errorf("%w: opcode %d is not a response", ErrMalformedAnswer, wire[1])
	}

	return &natpmpResponseHeader{
		Opcode: wire[1] - natpmpResponse,
		Result: int(binary.BigEndian.Uint16(wire[2:4])),
		Epoch:  binary.BigEndian.Uint32(wire[4:8]),
		Body:   wire[8:],
	}, nil
}

func parseNATPMPAddress(wire []byte) (netip.Addr, error) {
	if len(wire) < natpmpAddressResponseSize {
		return netip.Addr{}, fmt.Errorf("%w: address response is %d bytes", ErrMalformedAnswer, len(wire))
	}
	var raw [4]byte
	copy(raw[:], wire[8:12])
	return netip.AddrFrom4(raw), nil
}

type natpmpMapResult struct {
	InternalPort uint16
	ExternalPort uint16
	Lifetime     time.Duration
}

func parseNATPMPMap(wire []byte) (*natpmpMapResult, error) {
	if len(wire) < natpmpMapResponseSize {
		return nil, fmt.Errorf("%w: mapping response is %d bytes", ErrMalformedAnswer, len(wire))
	}
	return &natpmpMapResult{
		InternalPort: binary.BigEndian.Uint16(wire[8:10]),
		ExternalPort: binary.BigEndian.Uint16(wire[10:12]),
		Lifetime:     time.Duration(binary.BigEndian.Uint32(wire[12:16])) * time.Second,
	}, nil
}
