// Package pcp speaks PCP (RFC 6887) and NAT-PMP (RFC 6886) to a gateway. Both
// live on UDP 5351 and RFC 6887 section 9 defines how they interoperate, so one
// client covers the two.
package pcp

import (
	"errors"
	"fmt"
)

var (
	// ErrNoAnswer means the gateway stayed silent, which is what a router
	// without either protocol enabled looks like.
	ErrNoAnswer = errors.New("pcp: the gateway did not answer")

	ErrMalformedAnswer = errors.New("pcp: malformed answer")
	ErrWrongOpcode     = errors.New("pcp: answer does not match the request opcode")
	ErrWrongNonce      = errors.New("pcp: answer carries a different mapping nonce")
	ErrNotSupported    = errors.New("pcp: the gateway speaks neither PCP nor NAT-PMP")
)

// Version is which of the two protocols the gateway answered with.
type Version int

const (
	VersionUnknown Version = iota
	VersionPCP
	VersionNATPMP
)

func (v Version) String() string {
	switch v {
	case VersionPCP:
		return "PCP"
	case VersionNATPMP:
		return "NAT-PMP"
	default:
		return "none"
	}
}

// Protocol is the IANA number carried in a mapping request.
type Protocol uint8

const (
	ProtocolTCP Protocol = 6
	ProtocolUDP Protocol = 17
)

func (p Protocol) String() string {
	switch p {
	case ProtocolTCP:
		return "TCP"
	case ProtocolUDP:
		return "UDP"
	default:
		return fmt.Sprintf("protocol %d", uint8(p))
	}
}

// ResultError is a gateway that answered and refused.
type ResultError struct {
	Version Version
	Code    int
}

func (e *ResultError) Error() string {
	return fmt.Sprintf("pcp: %s gateway refused with %s", e.Version, e.description())
}

func (e *ResultError) description() string {
	names := pcpResultNames
	if e.Version == VersionNATPMP {
		names = natpmpResultNames
	}
	if name, ok := names[e.Code]; ok {
		return fmt.Sprintf("%s (%d)", name, e.Code)
	}
	return fmt.Sprintf("result %d", e.Code)
}

const (
	resultSuccess        = 0
	resultUnsuppVersion  = 1
	resultNotAuthorized  = 2
	resultNoResources    = 8
	resultUnsuppProtocol = 9
)

var pcpResultNames = map[int]string{
	0:  "SUCCESS",
	1:  "UNSUPP_VERSION",
	2:  "NOT_AUTHORIZED",
	3:  "MALFORMED_REQUEST",
	4:  "UNSUPP_OPCODE",
	5:  "UNSUPP_OPTION",
	6:  "MALFORMED_OPTION",
	7:  "NETWORK_FAILURE",
	8:  "NO_RESOURCES",
	9:  "UNSUPP_PROTOCOL",
	10: "USER_EX_QUOTA",
	11: "CANNOT_PROVIDE_EXTERNAL",
	12: "ADDRESS_MISMATCH",
	13: "EXCESSIVE_REMOTE_PEERS",
}

var natpmpResultNames = map[int]string{
	0: "Success",
	1: "Unsupported Version",
	2: "Not Authorized/Refused",
	3: "Network Failure",
	4: "Out of resources",
	5: "Unsupported opcode",
}

// IsUnsupportedVersion reports the answer a NAT-PMP only gateway gives to a PCP
// request, which is how the two protocols are told apart.
func IsUnsupportedVersion(err error) bool {
	var result *ResultError
	return errors.As(err, &result) && result.Code == resultUnsuppVersion
}
