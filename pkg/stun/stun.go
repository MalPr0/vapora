package stun

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"time"
)

const (
	bindingRequest  = 0x0001
	bindingResponse = 0x0101
	magicCookie     = 0x2112A442
	headerSize      = 20

	attrMappedAddress    = 0x0001
	attrChangeRequest    = 0x0003
	attrChangedAddress   = 0x0005
	attrXORMappedAddress = 0x0020
	attrOtherAddress     = 0x802C

	changeIPFlag   = 0x04
	changePortFlag = 0x02

	familyIPv4 = 0x01
)

var (
	// ErrNoMappedAddress means a server answered without saying what it saw,
	// which happens with misconfigured servers and with things that are not
	// STUN servers at all.
	ErrNoMappedAddress     = errors.New("stun: response carries no mapped address")
	errTransactionMismatch = errors.New("stun: transaction id mismatch")
)

// Response is what a server reported about this socket.
type Response struct {
	// Mapped is the public endpoint the server saw.
	Mapped *net.UDPAddr
	// Other is the alternate IP and port the server can answer from, present
	// only on servers implementing RFC 5780.
	Other *net.UDPAddr
	// Source is where the answer actually came from.
	Source *net.UDPAddr
}

type queryOptions struct {
	changeIP   bool
	changePort bool
	// acceptAnySource allows the answer to arrive from an address other than
	// the one queried, which is the point of the filtering tests.
	acceptAnySource bool
}

// Query asks a STUN server for the public endpoint of conn. Reusing the same
// conn across servers is what makes the NAT mapping behaviour observable.
func Query(ctx context.Context, conn *net.UDPConn, server string, timeout time.Duration) (*net.UDPAddr, error) {
	response, err := query(ctx, conn, server, timeout, queryOptions{})
	if err != nil {
		return nil, err
	}
	return response.Mapped, nil
}

func query(ctx context.Context, conn *net.UDPConn, server string, timeout time.Duration, opts queryOptions) (*Response, error) {
	target, err := net.ResolveUDPAddr("udp4", server)
	if err != nil {
		return nil, fmt.Errorf("stun: cannot resolve %s: %w", server, err)
	}

	transactionID, request, err := buildBindingRequest(opts)
	if err != nil {
		return nil, err
	}

	deadline := time.Now().Add(timeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return nil, fmt.Errorf("stun: cannot set deadline: %w", err)
	}
	defer conn.SetDeadline(time.Time{})

	if _, err := conn.WriteToUDP(request, target); err != nil {
		return nil, fmt.Errorf("stun: cannot send binding request to %s: %w", server, err)
	}

	buffer := make([]byte, 1500)
	for {
		n, from, err := conn.ReadFromUDP(buffer)
		if err != nil {
			return nil, fmt.Errorf("stun: no answer from %s: %w", server, err)
		}
		if !opts.acceptAnySource && !from.IP.Equal(target.IP) {
			continue
		}

		response, err := parseBindingResponse(buffer[:n], transactionID)
		if errors.Is(err, errTransactionMismatch) {
			continue
		}
		if err != nil {
			return nil, err
		}
		response.Source = from
		return response, nil
	}
}

func buildBindingRequest(opts queryOptions) ([12]byte, []byte, error) {
	var transactionID [12]byte
	if _, err := rand.Read(transactionID[:]); err != nil {
		return transactionID, nil, fmt.Errorf("stun: cannot generate transaction id: %w", err)
	}

	var body []byte
	if opts.changeIP || opts.changePort {
		var flags byte
		if opts.changeIP {
			flags |= changeIPFlag
		}
		if opts.changePort {
			flags |= changePortFlag
		}
		body = make([]byte, 8)
		binary.BigEndian.PutUint16(body[0:2], attrChangeRequest)
		binary.BigEndian.PutUint16(body[2:4], 4)
		body[7] = flags
	}

	request := make([]byte, headerSize, headerSize+len(body))
	binary.BigEndian.PutUint16(request[0:2], bindingRequest)
	binary.BigEndian.PutUint16(request[2:4], uint16(len(body)))
	binary.BigEndian.PutUint32(request[4:8], magicCookie)
	copy(request[8:20], transactionID[:])
	return transactionID, append(request, body...), nil
}

func parseBindingResponse(payload []byte, transactionID [12]byte) (*Response, error) {
	if len(payload) < headerSize {
		return nil, errors.New("stun: response shorter than a header")
	}
	if binary.BigEndian.Uint16(payload[0:2]) != bindingResponse {
		return nil, fmt.Errorf("stun: unexpected message type 0x%04x", binary.BigEndian.Uint16(payload[0:2]))
	}
	if string(payload[8:20]) != string(transactionID[:]) {
		return nil, errTransactionMismatch
	}

	length := int(binary.BigEndian.Uint16(payload[2:4]))
	if headerSize+length > len(payload) {
		return nil, errors.New("stun: truncated response body")
	}

	response := &Response{}
	var fallback *net.UDPAddr
	body := payload[headerSize : headerSize+length]

	for len(body) >= 4 {
		attrType := binary.BigEndian.Uint16(body[0:2])
		attrLength := int(binary.BigEndian.Uint16(body[2:4]))
		if 4+attrLength > len(body) {
			break
		}
		value := body[4 : 4+attrLength]

		switch attrType {
		case attrXORMappedAddress:
			if address, err := decodeAddress(value, true); err == nil {
				response.Mapped = address
			}
		case attrMappedAddress:
			if address, err := decodeAddress(value, false); err == nil {
				fallback = address
			}
		case attrOtherAddress, attrChangedAddress:
			if address, err := decodeAddress(value, false); err == nil {
				response.Other = address
			}
		}

		advance := 4 + attrLength
		if padding := attrLength % 4; padding != 0 {
			advance += 4 - padding
		}
		if advance > len(body) {
			break
		}
		body = body[advance:]
	}

	if response.Mapped == nil {
		response.Mapped = fallback
	}
	if response.Mapped == nil {
		return nil, ErrNoMappedAddress
	}
	return response, nil
}

func decodeAddress(value []byte, xored bool) (*net.UDPAddr, error) {
	if len(value) < 8 || value[1] != familyIPv4 {
		return nil, errors.New("stun: unsupported address attribute")
	}

	port := binary.BigEndian.Uint16(value[2:4])
	address := make(net.IP, 4)
	copy(address, value[4:8])

	if xored {
		port ^= magicCookie >> 16
		var cookie [4]byte
		binary.BigEndian.PutUint32(cookie[:], magicCookie)
		for i := range address {
			address[i] ^= cookie[i]
		}
	}
	return &net.UDPAddr{IP: address, Port: int(port)}, nil
}
