package dht

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

// Wire is the socket as this package needs it. It never reads: datagrams are
// pushed in through Deliver, so the DHT shares one socket — and therefore one
// NAT binding — with whatever else is using it. Announcing a port other than
// the one the conversation will arrive on would be pointless.
type Wire interface {
	Send(payload []byte, to *net.UDPAddr) error
}

// Bootstrap is where a client enters the network. Joining requires knowing at
// least one node, and these are the addresses every implementation ships with.
// They are the one centralised part of an otherwise distributed system, and the
// same kind of dependency as a public STUN server.
var Bootstrap = []string{
	"router.bittorrent.com:6881",
	"dht.transmissionbt.com:6881",
	"router.utorrent.com:6881",
}

const (
	// alpha is how many nodes are asked at once. The spec suggests three; more
	// converges no faster and makes this look like a scan.
	alpha = 3
	// closest is how many near nodes are kept and eventually announced to.
	closest = 8
	// rounds bounds the search. Each one halves the distance in the good case,
	// so this is far more than convergence needs and exists to stop a network
	// full of liars from keeping us here forever.
	rounds       = 12
	queryTimeout = 3 * time.Second
)

var (
	ErrNoNodes = errors.New("dht: no node answered, the network is unreachable from here")
	// ErrNotAnnounced means nodes answered but none accepted the announcement,
	// which leaves nothing for the other side to find. Silently treating that
	// as success is how a rendezvous appears to work and never meets anyone.
	ErrNotAnnounced = errors.New("dht: no node accepted the announcement")
)

// Client is a DHT client: it asks questions and never answers any.
//
// Being a full node would mean serving strangers from the same socket the
// conversation runs on, which is a much larger surface for no benefit to what
// this is for.
type Client struct {
	id   ID
	wire Wire

	mu      sync.Mutex
	pending map[string]chan Dict
	nextTag uint32
}

func NewClient(wire Wire) (*Client, error) {
	id, err := NewID()
	if err != nil {
		return nil, err
	}
	return &Client{id: id, wire: wire, pending: map[string]chan Dict{}}, nil
}

// Deliver takes a datagram that arrived on the shared socket. It reports
// whether it was one of ours: anything that is not a reply to a question we
// asked is left for somebody else to handle.
func (c *Client) Deliver(payload []byte, _ *net.UDPAddr) bool {
	decoded, err := Decode(payload)
	if err != nil {
		return false
	}
	message, ok := decoded.(Dict)
	if !ok {
		return false
	}

	// Only replies are handled. A query addressed to us is not answered at all,
	// which is what makes this a client.
	if kind, _ := message.String("y"); kind != "r" {
		return false
	}
	tag, ok := message.String("t")
	if !ok {
		return false
	}
	reply, ok := message.Dict("r")
	if !ok {
		return false
	}

	c.mu.Lock()
	waiting, known := c.pending[tag]
	if known {
		delete(c.pending, tag)
	}
	c.mu.Unlock()

	if !known {
		// A reply to nothing. Either it is very late or somebody is guessing
		// transaction tags; either way it is not acted on.
		return false
	}
	waiting <- reply
	return true
}

// query sends one question and waits for its answer.
func (c *Client) query(ctx context.Context, to *net.UDPAddr, method string, args Dict) (Dict, error) {
	args["id"] = string(c.id[:])

	c.mu.Lock()
	c.nextTag++
	var tag [4]byte
	binary.BigEndian.PutUint32(tag[:], c.nextTag)
	name := string(tag[:])
	answer := make(chan Dict, 1)
	c.pending[name] = answer
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pending, name)
		c.mu.Unlock()
	}()

	wire, err := Encode(Dict{"t": name, "y": "q", "q": method, "a": args})
	if err != nil {
		return nil, err
	}
	if err := c.wire.Send(wire, to); err != nil {
		return nil, err
	}

	timer := time.NewTimer(queryTimeout)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		return nil, fmt.Errorf("dht: %s did not answer", to)
	case reply := <-answer:
		return reply, nil
	}
}
