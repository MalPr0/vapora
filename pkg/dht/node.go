package dht

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
)

// IDSize is the width of a node ID and of a key, in bytes. Both are SHA-1 sized
// because that is what the network speaks; nothing here relies on SHA-1 for
// security, and the key this package is given is derived elsewhere.
const IDSize = 20

// ID is a node identity or a key to look up — the network uses one space for
// both, and distance between them is what organises it.
type ID [IDSize]byte

// NewID generates a random identity for this client.
//
// Real nodes derive theirs from their address so the network can resist a
// single machine claiming to be everywhere. This one only asks questions and
// stores nothing, so it has nothing to be trusted with.
func NewID() (ID, error) {
	var id ID
	if _, err := rand.Read(id[:]); err != nil {
		return id, fmt.Errorf("dht: cannot generate an id: %w", err)
	}
	return id, nil
}

// closer reports whether a is nearer to target than b, under the XOR metric the
// whole network is organised by.
func closer(a, b, target ID) bool {
	for i := 0; i < IDSize; i++ {
		left, right := a[i]^target[i], b[i]^target[i]
		if left != right {
			return left < right
		}
	}
	return false
}

// Node is one participant in the network: an ID and where to reach it.
type Node struct {
	ID   ID
	Addr *net.UDPAddr
}

const (
	compactNode = IDSize + 6 // id, then a four byte address and a two byte port
	compactPeer = 6
)

// parseNodes reads the "nodes" field of a reply: node IDs and addresses packed
// end to end with no separators.
//
// A short tail is ignored rather than rejected. Half the implementations on
// this network are old, and dropping an otherwise good reply over trailing
// rubbish loses the nodes that came with it.
func parseNodes(packed string) []Node {
	var nodes []Node
	for len(packed) >= compactNode {
		var node Node
		copy(node.ID[:], packed[:IDSize])
		node.Addr = parseAddr(packed[IDSize:compactNode])
		if node.Addr != nil {
			nodes = append(nodes, node)
		}
		packed = packed[compactNode:]
	}
	return nodes
}

// parseAddr reads four bytes of address and two of port, big endian.
func parseAddr(packed string) *net.UDPAddr {
	if len(packed) < compactPeer {
		return nil
	}
	port := binary.BigEndian.Uint16([]byte(packed[4:6]))
	if port == 0 {
		return nil
	}

	ip := net.IPv4(packed[0], packed[1], packed[2], packed[3])
	if !routable(ip) {
		// A node that names a private or reserved address is either broken or
		// pointing us at something inside our own network.
		return nil
	}
	return &net.UDPAddr{IP: ip, Port: int(port)}
}

// routable rejects the addresses that cannot be a peer on the internet.
// Following one of these means sending traffic somewhere on the local network
// on a stranger's say-so.
func routable(ip net.IP) bool {
	v4 := ip.To4()
	if v4 == nil {
		return false
	}
	switch {
	case v4[0] == 0, v4[0] == 10, v4[0] == 127:
		return false
	case v4[0] == 169 && v4[1] == 254:
		return false
	case v4[0] == 172 && v4[1] >= 16 && v4[1] <= 31:
		return false
	case v4[0] == 192 && v4[1] == 168:
		return false
	case v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127:
		return false
	case v4[0] >= 224:
		return false
	}
	return true
}

// packAddr writes an address in the compact form the network expects.
func packAddr(addr *net.UDPAddr) string {
	v4 := addr.IP.To4()
	if v4 == nil {
		return ""
	}
	var packed [compactPeer]byte
	copy(packed[:4], v4)
	binary.BigEndian.PutUint16(packed[4:], uint16(addr.Port))
	return string(packed[:])
}

// parsePeers reads the "values" field, which is a list of addresses rather than
// one packed string.
func parsePeers(values List) []*net.UDPAddr {
	var peers []*net.UDPAddr
	for _, value := range values {
		packed, ok := value.(string)
		if !ok {
			continue
		}
		if addr := parseAddr(packed); addr != nil {
			peers = append(peers, addr)
		}
	}
	return peers
}

func sameAddr(a, b *net.UDPAddr) bool {
	return a != nil && b != nil && a.Port == b.Port && bytes.Equal(a.IP.To4(), b.IP.To4())
}
