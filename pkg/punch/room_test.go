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

func (l *roomLog) Message(from Member, payload string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, from.Name+": "+payload)
}

func (l *roomLog) Typing(Member, bool) {}

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

	guest.room.Broadcast("hola")
	waitUntil(t, "the line never arrived", func() bool { return host.log.has("hola") })

	host.room.Broadcast("chau")
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

	bob.room.Broadcast("carol, me escuchas?")
	waitUntil(t, "carol never heard bob once the introducer left", func() bool {
		return carol.log.has("me escuchas")
	})

	carol.room.Broadcast("fuerte y claro")
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
	carol.room.Broadcast("hola alice")
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

	if alice.room.Me().Name != bob.room.Members()[0].Name {
		t.Fatalf("alice calls herself %q, bob calls her %q",
			alice.room.Me().Name, bob.room.Members()[0].Name)
	}
	if bob.room.Me().Name != alice.room.Members()[0].Name {
		t.Fatalf("bob calls himself %q, alice calls him %q",
			bob.room.Me().Name, alice.room.Members()[0].Name)
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
