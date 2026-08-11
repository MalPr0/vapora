package dht

import (
	"context"
	"net"
	"sort"
	"sync"
)

// found is what a search accumulates: the peers already at the key, and the
// nodes near enough to be worth announcing to.
type found struct {
	mu     sync.Mutex
	peers  []*net.UDPAddr
	near   []Node
	tokens map[string]string
	asked  map[string]bool
	// answered counts the nodes that replied. Without it there is no way to
	// tell "nobody has announced this key" from "nothing on this network can
	// reach the DHT at all", and those call for opposite advice.
	answered int
	// accepted counts the nodes that took the announcement. An announcement
	// nobody accepted is indistinguishable from never having made one, and the
	// caller has to be able to tell.
	accepted int
}

// Lookup finds the addresses somebody else announced under this key.
//
// The search walks towards the key: each node asked returns either the peers it
// knows or the nodes it knows that are nearer, and the nearest are asked next.
// It converges because every round moves closer under the XOR metric.
//
// An empty result with no error means nobody has announced that key. A network
// that cannot reach the DHT at all returns ErrNoNodes instead, because the two
// call for opposite advice: wait, or stop waiting.
func (c *Client) Lookup(ctx context.Context, key ID) ([]*net.UDPAddr, error) {
	result, err := c.search(ctx, key)
	if err != nil {
		return nil, err
	}
	if result.answered == 0 {
		return nil, ErrNoNodes
	}
	return result.peers, nil
}

// Announce leaves this address under the key, where Lookup will find it.
//
// The port announced is the one the conversation will use, because the DHT
// shares its socket: a router that maps them separately would otherwise have
// everyone dialling a port that leads nowhere.
func (c *Client) Announce(ctx context.Context, key ID, port int) ([]*net.UDPAddr, error) {
	result, err := c.search(ctx, key)
	if err != nil {
		return nil, err
	}

	// Announcing to the nodes nearest the key is what makes it findable: a
	// lookup walks to the same place and asks them.
	var waiting sync.WaitGroup
	for _, node := range result.near {
		token, ok := result.tokens[node.Addr.String()]
		if !ok {
			// Without the token from that node's own reply it would refuse the
			// announcement. The token is what stops this from being a way to
			// publish somebody else's address.
			continue
		}

		waiting.Add(1)
		go func(node Node, token string) {
			defer waiting.Done()
			if _, err := c.query(ctx, node.Addr, "announce_peer", Dict{
				"info_hash":    string(key[:]),
				"port":         port,
				"token":        token,
				"implied_port": 0,
			}); err == nil {
				result.mu.Lock()
				result.accepted++
				result.mu.Unlock()
			}
		}(node, token)
	}
	waiting.Wait()

	if result.answered == 0 {
		return nil, ErrNoNodes
	}
	if result.accepted == 0 {
		return result.peers, ErrNotAnnounced
	}
	return result.peers, nil
}

// search is the iterative walk both operations are built on.
func (c *Client) search(ctx context.Context, key ID) (*found, error) {
	result := &found{tokens: map[string]string{}, asked: map[string]bool{}}

	seeds, err := c.seed(ctx)
	if err != nil {
		return nil, err
	}
	result.near = seeds

	for round := 0; round < rounds; round++ {
		batch := result.next(key)
		if len(batch) == 0 {
			break
		}

		var waiting sync.WaitGroup
		for _, node := range batch {
			waiting.Add(1)
			go func(node Node) {
				defer waiting.Done()
				c.ask(ctx, node, key, result)
			}(node)
		}
		waiting.Wait()

		if ctx.Err() != nil {
			break
		}
	}
	return result, nil
}

// seed resolves the bootstrap addresses and asks each one where the key lives.
// Their answers are what the search starts from.
func (c *Client) seed(ctx context.Context) ([]Node, error) {
	var nodes []Node
	for _, name := range Bootstrap {
		addr, err := net.ResolveUDPAddr("udp4", name)
		if err != nil {
			continue
		}
		// A bootstrap node's ID is not known until it answers, and the zero ID
		// only decides who is asked first.
		nodes = append(nodes, Node{Addr: addr})
	}

	if len(nodes) == 0 {
		return nil, ErrNoNodes
	}
	return nodes, nil
}

// ask puts one question to one node and folds the answer into the search.
func (c *Client) ask(ctx context.Context, node Node, key ID, result *found) {
	reply, err := c.query(ctx, node.Addr, "get_peers", Dict{"info_hash": string(key[:])})
	if err != nil {
		return
	}

	var id ID
	if raw, ok := reply.String("id"); ok && len(raw) == IDSize {
		copy(id[:], raw)
	}

	result.mu.Lock()
	defer result.mu.Unlock()

	result.answered++

	// The token is what this node will require back before accepting an
	// announcement, so it is kept against the node that issued it.
	if token, ok := reply.String("token"); ok {
		result.tokens[node.Addr.String()] = token
	}

	if values, ok := reply.List("values"); ok {
		for _, peer := range parsePeers(values) {
			if !containsAddr(result.peers, peer) {
				result.peers = append(result.peers, peer)
			}
		}
	}

	if packed, ok := reply.String("nodes"); ok {
		for _, near := range parseNodes(packed) {
			if !containsNode(result.near, near) {
				result.near = append(result.near, near)
			}
		}
	}

	// The node that answered is a real one, so record its ID now that we know
	// it: the search is ordered by distance and a zero ID would sort wrongly.
	for i := range result.near {
		if sameAddr(result.near[i].Addr, node.Addr) {
			result.near[i].ID = id
		}
	}
}

// next picks the nearest nodes not yet asked, and marks them asked.
func (f *found) next(key ID) []Node {
	f.mu.Lock()
	defer f.mu.Unlock()

	sort.SliceStable(f.near, func(i, j int) bool {
		return closer(f.near[i].ID, f.near[j].ID, key)
	})
	// Keeping only the nearest bounds both the memory a hostile reply can cost
	// and who ends up being announced to.
	if len(f.near) > closest*4 {
		f.near = f.near[:closest*4]
	}

	var batch []Node
	for _, node := range f.near {
		if len(batch) == alpha {
			break
		}
		key := node.Addr.String()
		if f.asked[key] {
			continue
		}
		f.asked[key] = true
		batch = append(batch, node)
	}
	return batch
}

func containsAddr(list []*net.UDPAddr, addr *net.UDPAddr) bool {
	for _, current := range list {
		if sameAddr(current, addr) {
			return true
		}
	}
	return false
}

func containsNode(list []Node, node Node) bool {
	for _, current := range list {
		if sameAddr(current.Addr, node.Addr) {
			return true
		}
	}
	return false
}
