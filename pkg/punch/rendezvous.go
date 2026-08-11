package punch

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/MalPr0/vapora/pkg/dht"
)

// RendezvousKey is where a secret says to meet on the DHT.
//
// It is derived rather than being the secret itself, and the derivation only
// goes one way: somebody watching the DHT sees a 20 byte key with an address
// under it and cannot work back to the secret, so they cannot read anything.
// What they can do is see that an address is there, which is the cost of using
// a public network at all.
func RendezvousKey(secret Secret) (dht.ID, error) {
	material, err := deriveKey(secret, nil, rendezvousInfo)
	if err != nil {
		return dht.ID{}, err
	}

	var key dht.ID
	copy(key[:], material)
	return key, nil
}

const rendezvousInfo = "vapora rendezvous v1"

// Rendezvous publishes where to find this side and reports where the other side
// said it was.
//
// Nothing it returns is trusted. The DHT will happily hand back addresses that
// were never announced — there are nodes on it that answer every key with
// whatever they like — so an address from here is a place to try, never a peer.
// Becoming a peer still requires producing a frame under the secret.
type Rendezvous struct {
	client *dht.Client
	key    dht.ID
	port   int
}

func NewRendezvous(wire dht.Wire, secret Secret, port int) (*Rendezvous, error) {
	key, err := RendezvousKey(secret)
	if err != nil {
		return nil, err
	}

	client, err := dht.NewClient(wire)
	if err != nil {
		return nil, err
	}
	return &Rendezvous{client: client, key: key, port: port}, nil
}

// Deliver hands a datagram to the DHT client, and reports whether it was one of
// its replies. The socket is shared, so anything else has to be left alone.
func (r *Rendezvous) Deliver(payload []byte, from *net.UDPAddr) bool {
	return r.client.Deliver(payload, from)
}

// Publish announces this side and keeps announcing: the network drops an
// announcement after a while, so one is only good for as long as it takes to be
// forgotten.
//
// Every round also reports whoever else is there, which is how both sides find
// each other when neither knew an address to begin with.
func (r *Rendezvous) Publish(ctx context.Context, found func([]*net.UDPAddr)) error {
	for {
		peers, err := r.client.Announce(ctx, r.key, r.port)
		if err != nil {
			return fmt.Errorf("punch: cannot reach the DHT: %w", err)
		}
		if found != nil && len(peers) > 0 {
			found(peers)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(republish):
		}
	}
}

// Find asks where the other side is, without publishing anything.
func (r *Rendezvous) Find(ctx context.Context) ([]*net.UDPAddr, error) {
	return r.client.Lookup(ctx, r.key)
}

// republish is well inside the time the network keeps an announcement, which
// implementations put at fifteen minutes to two hours.
const republish = 8 * time.Minute
