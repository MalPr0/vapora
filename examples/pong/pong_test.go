package main

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/MalPr0/vapora/pkg/punch"
)

// Two real sessions over loopback, built through the exported API only — the
// same way the game builds them. If a game cannot be assembled from outside the
// transport, the layering is a claim rather than a fact.
func linked(t *testing.T) (*table, *table) {
	t.Helper()

	secret, err := punch.NewSecret()
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	left := built(t, ctx, secret, punch.RoleInviter)
	right := built(t, ctx, secret, punch.RoleJoiner)

	left.session.SetPeer(right.session.Peer())
	right.session.SetPeer(left.session.Peer())

	go left.session.Open(ctx, 10*time.Second)
	go right.session.Open(ctx, 10*time.Second)

	for _, side := range []*table{left, right} {
		if err := side.session.Established(deadline(t, ctx)); err != nil {
			t.Fatalf("the path never opened: %v", err)
		}
	}
	return left, right
}

func deadline(t *testing.T, ctx context.Context) context.Context {
	t.Helper()
	timed, cancel := context.WithTimeout(ctx, 10*time.Second)
	t.Cleanup(cancel)
	return timed
}

func built(t *testing.T, ctx context.Context, secret punch.Secret, role punch.Role) *table {
	t.Helper()

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })

	codec, err := punch.NewSecretCodec(secret, role)
	if err != nil {
		t.Fatal(err)
	}

	mux := punch.NewMux(conn)
	session := punch.NewSession(mux, codec, nil)
	mux.Fallback(session)

	side := &table{session: session, incoming: make(chan []byte, 64)}
	session.Observe(punch.ObserverFunc(func(payload []byte) {
		select {
		case side.incoming <- payload:
		default:
		}
	}))

	go mux.Run(ctx)
	go session.Run(ctx)
	session.SetPeer(conn.LocalAddr().(*net.UDPAddr))

	return side
}

// The host simulates and the guest is told. What has to arrive is a ball that
// moves — a frozen one would mean the state is not crossing.
func TestTheBallReachesTheOtherSide(t *testing.T) {
	host, guest := linked(t)

	world := newGame()
	seen := map[[2]uint16]bool{}

	for tick := 0; tick < 60; tick++ {
		world.tick()
		host.session.Send(encodeState(world.state))

		select {
		case payload := <-guest.incoming:
			state, ok := decodeState(payload)
			if !ok {
				t.Fatalf("the guest could not read what the host sent: %v", payload)
			}
			seen[[2]uint16{state.BallX, state.BallY}] = true
		case <-time.After(2 * time.Second):
			t.Fatal("no state arrived at the guest")
		}
	}

	if len(seen) < 10 {
		t.Fatalf("the ball only ever appeared in %d places, so it is not moving", len(seen))
	}
}

// The guest is authoritative about exactly one thing, and it has to get there.
func TestThePaddleReachesTheHost(t *testing.T) {
	host, guest := linked(t)

	world := newGame()
	for i := 0; i < 6; i++ {
		world.move(1, paddleSpeed)
	}
	guest.session.Send(encodePaddle(world.paddle[1]))

	select {
	case payload := <-host.incoming:
		y, ok := decodePaddle(payload)
		if !ok {
			t.Fatalf("the host could not read the paddle: %v", payload)
		}
		if y != world.paddle[1] {
			t.Fatalf("the paddle arrived at %d, sent from %d", y, world.paddle[1])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the paddle never reached the host")
	}
}

// Losing a packet must not break anything, because every one carries the whole
// world. This is the property that lets the game skip acknowledgements
// entirely, and it is the opposite of what a conversation needs.
func TestALostPacketCostsNothing(t *testing.T) {
	world := newGame()

	for tick := 0; tick < 20; tick++ {
		world.tick()
	}
	before := world.state

	// Whatever the guest last received, the next packet replaces it whole.
	restored, ok := decodeState(encodeState(before))
	if !ok {
		t.Fatal("a state did not survive its own encoding")
	}
	if restored != before {
		t.Fatalf("the world changed shape on the wire: %+v vs %+v", restored, before)
	}
}

// Everything here arrives from the network. A length that does not match is not
// this program, and a position that cannot exist must be clamped rather than
// used to index a screen.
func TestNonsenseFromTheNetworkIsRefused(t *testing.T) {
	for _, payload := range [][]byte{
		nil,
		{tagState},
		{tagState, 1, 2},
		{tagPaddle, 1, 2, 3},
		append([]byte{99}, make([]byte, stateBytes)...),
	} {
		if _, ok := decodeState(payload); ok {
			t.Fatalf("%v was accepted as a state", payload)
		}
		if _, ok := decodePaddle(payload); ok && len(payload) != 3 {
			t.Fatalf("%v was accepted as a paddle", payload)
		}
	}

	// A peer claiming the ball is off the field gets clamped onto it.
	wild := encodeState(State{BallX: 65535, BallY: 65535, LeftY: 65535, RightY: 65535})
	state, ok := decodeState(wild)
	if !ok {
		t.Fatal("a well formed state was refused")
	}
	if state.BallX > fieldWidth || state.BallY > fieldHeight ||
		state.LeftY > fieldHeight || state.RightY > fieldHeight {
		t.Fatalf("a position off the field survived: %+v", state)
	}
}

// The game is eleven bytes plus a tag. Worth asserting: the reason this design
// works at thirty ticks a second is that a tick is nothing.
func TestATickIsSmall(t *testing.T) {
	if size := len(encodeState(State{})); size > 16 {
		t.Fatalf("a tick is %d bytes", size)
	}
	if size := len(encodePaddle(0)); size > 4 {
		t.Fatalf("a paddle update is %d bytes", size)
	}
}

// Either side can ask to start again, but only the host decides. The guest
// sends the shortest message in the protocol and learns the new score the same
// way it learns everything else: from the next state.
func TestEitherSideCanAskToStartAgain(t *testing.T) {
	host, guest := linked(t)

	world := newGame()
	world.state.LeftScore = 9
	world.state.RightScore = 4

	guest.session.Send(encodeReset())

	select {
	case payload := <-host.incoming:
		if !isReset(payload) {
			t.Fatalf("the host did not recognise a reset: %v", payload)
		}
		world.reset()
	case <-time.After(2 * time.Second):
		t.Fatal("the reset never reached the host")
	}

	if world.state.LeftScore != 0 || world.state.RightScore != 0 {
		t.Fatalf("the score survived a reset: %+v", world.state)
	}

	// And the guest sees it without being told anything else.
	host.session.Send(encodeState(world.state))
	select {
	case payload := <-guest.incoming:
		state, ok := decodeState(payload)
		if !ok || state.LeftScore != 0 || state.RightScore != 0 {
			t.Fatalf("the guest did not see the reset: %+v", state)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no state followed the reset")
	}
}

// A reset is one byte and must not be mistaken for anything else, in either
// direction.
func TestResetIsNotConfusedWithAPaddle(t *testing.T) {
	if _, ok := decodePaddle(encodeReset()); ok {
		t.Fatal("a reset was read as a paddle")
	}
	if _, ok := decodeState(encodeReset()); ok {
		t.Fatal("a reset was read as a state")
	}
	if isReset(encodePaddle(500)) {
		t.Fatal("a paddle was read as a reset")
	}
	if isReset(encodeState(State{})) {
		t.Fatal("a state was read as a reset")
	}
}
