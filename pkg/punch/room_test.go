package punch

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

type roomLog struct {
	mu    sync.Mutex
	lines []string
}

// A room carries bytes and names nobody, so the log keys on the key itself.
// What to call somebody is the caller's problem, which is the point.
func (l *roomLog) Data(from Member, payload []byte) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, from.Key.String()+": "+string(payload))
}

func (l *roomLog) all() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.lines...)
}

func (l *roomLog) has(want string) bool {
	for _, line := range l.all() {
		if strings.Contains(line, want) {
			return true
		}
	}
	return false
}

type node struct {
	room *Room
	log  *roomLog
	addr *net.UDPAddr
	stop func()
}

// newNode builds one participant: a socket, a mux reading it, and a room.
func newNode(t *testing.T, ctx context.Context, secret Secret) *node {
	t.Helper()

	conn := listen(t)
	t.Cleanup(func() { conn.Close() })

	identity, err := NewIdentity()
	if err != nil {
		t.Fatalf("cannot generate an identity: %v", err)
	}

	mux := NewMux(conn)
	room, err := NewRoom(RoomOptions{Identity: identity, Secret: secret, Mux: mux})
	if err != nil {
		t.Fatalf("cannot build a room: %v", err)
	}

	log := &roomLog{}
	room.Observe(log)

	nodeCtx, cancel := context.WithCancel(ctx)
	go mux.Run(nodeCtx)

	return &node{
		room: room,
		log:  log,
		addr: localAddr(t, conn),
		stop: func() { cancel(); conn.Close() },
	}
}

func waitUntil(t *testing.T, what string, check func() bool) {
	t.Helper()

	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal(what)
}

func TestTwoJoinAndTalk(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	secret, err := NewSecret()
	if err != nil {
		t.Fatalf("cannot generate a secret: %v", err)
	}
	host, guest := newNode(t, ctx, secret), newNode(t, ctx, secret)

	if err := guest.room.Join(ctx, host.room.Invite(host.addr), 8*time.Second); err != nil {
		t.Fatalf("cannot join: %v", err)
	}

	waitUntil(t, "the host never saw the guest", func() bool { return len(host.room.Members()) == 1 })

	guest.room.Broadcast([]byte("hola"))
	waitUntil(t, "the line never arrived", func() bool { return host.log.has("hola") })

	host.room.Broadcast([]byte("chau"))
	waitUntil(t, "the answer never arrived", func() bool { return guest.log.has("chau") })
}

// The point of a mesh: whoever introduces two people never carries a word
// between them.
func TestThreeMeetAndTalkDirectly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	secret, err := NewSecret()
	if err != nil {
		t.Fatalf("cannot generate a secret: %v", err)
	}
	alice := newNode(t, ctx, secret)
	bob, carol := newNode(t, ctx, secret), newNode(t, ctx, secret)

	if err := bob.room.Join(ctx, alice.room.Invite(alice.addr), 8*time.Second); err != nil {
		t.Fatalf("bob cannot join: %v", err)
	}
	if err := carol.room.Join(ctx, alice.room.Invite(alice.addr), 8*time.Second); err != nil {
		t.Fatalf("carol cannot join: %v", err)
	}

	// Everyone ends up knowing everyone, without carol and bob ever having been
	// handed each other's address by hand.
	for _, who := range []*node{alice, bob, carol} {
		waitUntil(t, "the room never converged", func() bool { return len(who.room.Members()) == 2 })
	}

	// Alice introduced them and now leaves entirely. If the path between bob
	// and carol went through her, this is where it would stop working; a
	// broadcast reaching everyone proves nothing on its own, since it is meant
	// to.
	alice.stop()
	time.Sleep(200 * time.Millisecond)

	bob.room.Broadcast([]byte("carol, me escuchas?"))
	waitUntil(t, "carol never heard bob once the introducer left", func() bool {
		return carol.log.has("me escuchas")
	})

	carol.room.Broadcast([]byte("fuerte y claro"))
	waitUntil(t, "bob never heard carol back", func() bool { return bob.log.has("fuerte y claro") })
}

// Anyone can invite, and somebody who joined can bring in the next person
// without the original host doing anything.
func TestAGuestCanInvite(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	secret, err := NewSecret()
	if err != nil {
		t.Fatalf("cannot generate a secret: %v", err)
	}
	alice, bob, carol := newNode(t, ctx, secret), newNode(t, ctx, secret), newNode(t, ctx, secret)

	if err := bob.room.Join(ctx, alice.room.Invite(alice.addr), 8*time.Second); err != nil {
		t.Fatalf("bob cannot join: %v", err)
	}
	// Carol arrives through bob, who was himself a guest.
	if err := carol.room.Join(ctx, bob.room.Invite(bob.addr), 8*time.Second); err != nil {
		t.Fatalf("carol cannot join through bob: %v", err)
	}

	waitUntil(t, "alice never learned about carol", func() bool { return len(alice.room.Members()) == 2 })
	carol.room.Broadcast([]byte("hola alice"))
	waitUntil(t, "alice never heard carol", func() bool { return alice.log.has("hola alice") })
}

// Names are derived, so everyone calls everyone the same thing.
func TestEveryoneAgreesOnNames(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	secret, _ := NewSecret()
	alice, bob := newNode(t, ctx, secret), newNode(t, ctx, secret)

	if err := bob.room.Join(ctx, alice.room.Invite(alice.addr), 8*time.Second); err != nil {
		t.Fatalf("cannot join: %v", err)
	}
	waitUntil(t, "the room never converged", func() bool { return len(alice.room.Members()) == 1 })

	if alice.room.Me().Key != bob.room.Members()[0].Key {
		t.Fatalf("alice calls herself %q, bob calls her %q",
			alice.room.Me().Key, bob.room.Members()[0].Key)
	}
	if bob.room.Me().Key != alice.room.Members()[0].Key {
		t.Fatalf("bob calls himself %q, alice calls him %q",
			bob.room.Me().Key, alice.room.Members()[0].Key)
	}
}

func TestRosterRoundTripAndLimits(t *testing.T) {
	key := PublicKey{1, 2, 3}
	roster := Roster{{Key: key, Addr: &net.UDPAddr{IP: net.IPv4(203, 0, 113, 7), Port: 41001}}}

	parsed, err := ParseRoster(roster.Marshal(), MaxMembers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(parsed) != 1 || parsed[0].Key != key || parsed[0].Addr.String() != "203.0.113.7:41001" {
		t.Fatalf("got %+v", parsed)
	}

	// A roster is attacker controlled, so its size is refused before anything
	// is allocated on its say so.
	huge := make(Roster, 200)
	for i := range huge {
		huge[i] = Entry{Key: PublicKey{byte(i + 1)}, Addr: &net.UDPAddr{IP: net.IPv4(10, 0, 0, 1), Port: 1}}
	}
	if _, err := ParseRoster(huge.Marshal(), MaxMembers); !errors.Is(err, ErrRosterTooLarge) {
		t.Fatalf("got %v", err)
	}
	if _, err := ParseRoster("", MaxMembers); err == nil {
		t.Fatal("an empty roster parsed")
	}
	if _, err := ParseRoster(string([]byte{4, 1, 2}), MaxMembers); !errors.Is(err, ErrMalformedRoster) {
		t.Fatalf("a truncated roster gave %v", err)
	}
}

func TestRoomInviteRoundTrips(t *testing.T) {
	identity, _ := NewIdentity()
	secret, _ := NewSecret()
	invite := RoomInvite{
		Endpoint: &net.UDPAddr{IP: net.IPv4(203, 0, 113, 7), Port: 41001},
		Secret:   secret,
		Host:     identity.Public(),
	}

	parsed, err := ParseRoomInvite(invite.Command("vapora room"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Endpoint.String() != invite.Endpoint.String() || parsed.Host != invite.Host || !parsed.Secret.Equal(secret) {
		t.Fatalf("got %+v", parsed)
	}

	// A one to one invite must say what it is rather than fail as noise.
	oneToOne := Invite{Endpoint: invite.Endpoint, Secret: secret}.Command("vapora punch")
	if _, err := ParseRoomInvite(oneToOne); !errors.Is(err, ErrNotRoomInvite) {
		t.Fatalf("got %v", err)
	}
	if _, err := ParseRoomInvite("nonsense"); !errors.Is(err, ErrNotRoomInvite) {
		t.Fatalf("got %v", err)
	}
}

// TestReachPunchesTowardsAnAddress covers the standoff `punch` already solves
// with a second invite and a room did not: the host only ever answers a hello,
// so between two networks that both refuse a first packet from a stranger, the
// newcomer's hello dies at the host's door and waiting longer never helps.
//
// The filter itself lives in a router, not in this code. What this side has to
// do is start sending — that is what puts the newcomer in its router's list of
// addresses worth accepting from — so that is what is asserted here.
func TestReachPunchesTowardsAnAddress(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	secret, err := NewSecret()
	if err != nil {
		t.Fatal(err)
	}

	// A bare socket standing in for the newcomer, which is not running a room
	// yet: Reach must not need anything of the other end.
	newcomer := listen(t)
	defer newcomer.Close()

	waiting := newNode(t, ctx, secret)
	defer waiting.stop()

	waiting.room.Reach(ctx, localAddr(t, newcomer))

	if err := newcomer.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, 2048)
	read, from, err := newcomer.ReadFromUDP(buffer)
	if err != nil {
		t.Fatalf("the waiting side never punched towards the address it was given: %v", err)
	}
	if from.Port != waiting.addr.Port {
		t.Fatalf("the packet came from %s, not from the waiting room at %s", from, waiting.addr)
	}

	// It is a hello under the room key, which is what the other side is
	// listening for, and it carries no secret of its own.
	frame, err := waiting.room.roomCode.Open(buffer[:read])
	if err != nil {
		t.Fatalf("what arrived is not a room frame: %v", err)
	}
	if frame.Kind != kindHello {
		t.Fatalf("the waiting side sent kind %#x, want a hello", frame.Kind)
	}
}

// Reach grants nothing on its own: an address is not a secret, and whoever is
// on the other end still has to produce a hello under the room key to become a
// member.
func TestReachAloneAddsNobody(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	secret, err := NewSecret()
	if err != nil {
		t.Fatal(err)
	}

	stranger := listen(t)
	defer stranger.Close()

	waiting := newNode(t, ctx, secret)
	defer waiting.stop()

	waiting.room.Reach(ctx, localAddr(t, stranger))
	time.Sleep(300 * time.Millisecond)

	if members := waiting.room.Members(); len(members) != 0 {
		t.Fatalf("punching towards an address made %d member(s) out of nothing", len(members))
	}
}
