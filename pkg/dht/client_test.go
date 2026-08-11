package dht

import (
	"context"
	"encoding/binary"
	"net"
	"sync"
	"testing"
	"time"
)

// fakeWire stands in for the socket. Every query is handed to a responder that
// plays the part of the network, so a search can be walked end to end without
// touching the internet.
type fakeWire struct {
	client *Client

	mu      sync.Mutex
	sent    []Dict
	respond func(query Dict, to *net.UDPAddr) (Dict, bool)
}

func (f *fakeWire) Send(payload []byte, to *net.UDPAddr) error {
	decoded, err := Decode(payload)
	if err != nil {
		return err
	}
	query := decoded.(Dict)

	f.mu.Lock()
	f.sent = append(f.sent, query)
	respond := f.respond
	f.mu.Unlock()

	if respond == nil {
		return nil
	}
	reply, ok := respond(query, to)
	if !ok {
		return nil
	}

	tag, _ := query.String("t")
	wire, err := Encode(Dict{"t": tag, "y": "r", "r": reply})
	if err != nil {
		return err
	}
	// Answers arrive on the socket, which is the only way in.
	go f.client.Deliver(wire, to)
	return nil
}

func (f *fakeWire) queries(method string) []Dict {
	f.mu.Lock()
	defer f.mu.Unlock()

	var matching []Dict
	for _, query := range f.sent {
		if name, _ := query.String("q"); name == method {
			matching = append(matching, query)
		}
	}
	return matching
}

func newTestClient(t *testing.T) (*Client, *fakeWire) {
	t.Helper()

	wire := &fakeWire{}
	client, err := NewClient(wire)
	if err != nil {
		t.Fatal(err)
	}
	wire.client = client
	return client, wire
}

func compact(ip string, port int) string {
	addr := &net.UDPAddr{IP: net.ParseIP(ip), Port: port}
	return packAddr(addr)
}

func packNode(id ID, ip string, port int) string {
	return string(id[:]) + compact(ip, port)
}

// A lookup has to reach the peers a node reports, whether they come back on the
// first answer or after a walk towards the key.
func TestLookupFindsPeersBehindAWalk(t *testing.T) {
	client, wire := newTestClient(t)

	var near ID
	near[0] = 0x01

	wire.respond = func(query Dict, to *net.UDPAddr) (Dict, bool) {
		args, _ := query.Dict("a")
		hash, _ := args.String("info_hash")

		// The bootstrap nodes only point further in.
		if to.Port == 6881 {
			return Dict{
				"id":    string(near[:]),
				"token": "tok",
				"nodes": packNode(near, "198.51.100.9", 4001),
			}, true
		}
		// The node that is actually near the key holds the peers.
		if len(hash) == IDSize {
			return Dict{
				"id":     string(near[:]),
				"token":  "tok",
				"values": List{compact("203.0.113.5", 41001)},
			}, true
		}
		return nil, false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var key ID
	key[0] = 0x02

	peers, err := client.Lookup(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if len(peers) != 1 {
		t.Fatalf("found %d peers, want the one that was announced: %v", len(peers), peers)
	}
	if peers[0].String() != "203.0.113.5:41001" {
		t.Fatalf("found %s", peers[0])
	}
}

// An announcement is worthless without the token from that same node's reply,
// and sending one anyway would be rejected. It must not be attempted.
func TestAnnounceOnlyGoesWhereATokenCameFrom(t *testing.T) {
	client, wire := newTestClient(t)

	var withToken, without ID
	withToken[0] = 0x01
	without[0] = 0x02

	wire.respond = func(query Dict, to *net.UDPAddr) (Dict, bool) {
		if to.Port == 6881 {
			return Dict{
				"id": string(withToken[:]),
				"nodes": packNode(withToken, "198.51.100.1", 4001) +
					packNode(without, "198.51.100.2", 4002),
			}, true
		}
		if to.Port == 4001 {
			return Dict{"id": string(withToken[:]), "token": "tok"}, true
		}
		// 4002 answers without a token, so it must never be announced to.
		return Dict{"id": string(without[:])}, true
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := client.Announce(ctx, ID{}, 41001); err != nil {
		t.Fatal(err)
	}

	announcements := wire.queries("announce_peer")
	if len(announcements) == 0 {
		t.Fatal("nothing was announced at all")
	}
	for _, announcement := range announcements {
		args, _ := announcement.Dict("a")
		if token, _ := args.String("token"); token != "tok" {
			t.Fatalf("announced with token %q", token)
		}
	}
}

// The port announced has to be the one the conversation will arrive on. The
// whole point of sharing a socket is that they are the same.
func TestAnnounceCarriesThePortItWasGiven(t *testing.T) {
	client, wire := newTestClient(t)

	wire.respond = func(query Dict, to *net.UDPAddr) (Dict, bool) {
		return Dict{"id": string(make([]byte, IDSize)), "token": "tok"}, true
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := client.Announce(ctx, ID{}, 41337); err != nil {
		t.Fatal(err)
	}

	announcements := wire.queries("announce_peer")
	if len(announcements) == 0 {
		t.Fatal("nothing was announced")
	}
	args, _ := announcements[0].Dict("a")
	if port, _ := args["port"].(int64); port != 41337 {
		t.Fatalf("announced port %v, want the one the socket is on", args["port"])
	}
}

// This is a client: a query addressed to it is not answered. Answering would
// mean serving strangers from the socket the conversation runs on.
func TestQueriesFromStrangersAreIgnored(t *testing.T) {
	client, wire := newTestClient(t)

	query, err := Encode(Dict{"t": "aa", "y": "q", "q": "ping", "a": Dict{"id": "x"}})
	if err != nil {
		t.Fatal(err)
	}

	from := &net.UDPAddr{IP: net.ParseIP("198.51.100.1"), Port: 6881}
	if client.Deliver(query, from) {
		t.Fatal("a query was claimed as ours")
	}
	if len(wire.sent) != 0 {
		t.Fatal("a stranger's query was answered")
	}
}

// A reply nobody asked for is somebody guessing transaction tags. Acting on it
// would let a stranger steer the search.
func TestUnmatchedRepliesAreDropped(t *testing.T) {
	client, _ := newTestClient(t)

	reply, err := Encode(Dict{"t": "zz", "y": "r", "r": Dict{
		"id":     string(make([]byte, IDSize)),
		"values": List{compact("203.0.113.5", 41001)},
	}})
	if err != nil {
		t.Fatal(err)
	}

	from := &net.UDPAddr{IP: net.ParseIP("198.51.100.1"), Port: 6881}
	if client.Deliver(reply, from) {
		t.Fatal("a reply to a question nobody asked was accepted")
	}
}

// Anything that is not ours has to be left alone: the socket is shared with the
// conversation, and swallowing its datagrams would break it.
func TestDeliverLeavesForeignTrafficAlone(t *testing.T) {
	client, _ := newTestClient(t)

	from := &net.UDPAddr{IP: net.ParseIP("198.51.100.1"), Port: 6881}
	for _, payload := range [][]byte{
		{0x00, 0x01, 0x02},
		[]byte("not bencode at all"),
		{},
	} {
		if client.Deliver(payload, from) {
			t.Fatalf("%q was claimed by the DHT", payload)
		}
	}
}

// A node that points at a private address is either broken or aiming us at
// something inside our own network.
func TestAddressesInsideTheNetworkAreRefused(t *testing.T) {
	for _, ip := range []string{
		"10.0.0.1", "192.168.1.1", "172.16.0.1", "127.0.0.1",
		"169.254.1.1", "100.64.0.1", "0.0.0.0", "224.0.0.1",
	} {
		if addr := parseAddr(compact(ip, 6881)); addr != nil {
			t.Fatalf("%s was accepted as a peer", ip)
		}
	}

	if addr := parseAddr(compact("203.0.113.5", 41001)); addr == nil {
		t.Fatal("a routable address was refused")
	}

	// A port of zero leads nowhere.
	var packed [6]byte
	copy(packed[:4], net.ParseIP("203.0.113.5").To4())
	binary.BigEndian.PutUint16(packed[4:], 0)
	if addr := parseAddr(string(packed[:])); addr != nil {
		t.Fatal("port zero was accepted")
	}
}

// The search is ordered by XOR distance, which is what makes it converge.
func TestCloserOrdersByXor(t *testing.T) {
	var target, near, far ID
	target[0] = 0b1000_0000
	near[0] = 0b1000_0001
	far[0] = 0b0000_0000

	if !closer(near, far, target) {
		t.Fatal("the nearer node did not sort first")
	}
	if closer(far, near, target) {
		t.Fatal("the ordering is not antisymmetric")
	}
	if closer(near, near, target) {
		t.Fatal("a node was reported closer than itself")
	}
}

// Half the implementations out there are old. Dropping a reply over a short
// tail would lose the good nodes that came with it.
func TestATruncatedNodeListKeepsWhatIsWhole(t *testing.T) {
	var id ID
	id[0] = 0x01

	packed := packNode(id, "203.0.113.5", 41001) + "garbage"
	nodes := parseNodes(packed)

	if len(nodes) != 1 {
		t.Fatalf("read %d nodes, want the one that was complete", len(nodes))
	}
	if nodes[0].Addr.String() != "203.0.113.5:41001" {
		t.Fatalf("read %s", nodes[0].Addr)
	}
}
