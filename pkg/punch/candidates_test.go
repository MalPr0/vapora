package punch

import (
	"context"
	"net"
	"testing"
	"time"
)

// This is the case that took a real router to find: two people behind the same
// one see each other on the roster, punch at a public address that needs the
// router to turn a packet around, and stay silent — while both talk happily to
// everybody outside.
//
// Here the unreachable address is a documentation one that answers nothing.
// The second candidate is the loopback the two of them are actually on.
func TestAPairFallsBackToTheAddressThatWorks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	secret, err := NewSecret()
	if err != nil {
		t.Fatal(err)
	}

	host := newNode(t, ctx, secret)
	defer host.stop()
	joiner := newNode(t, ctx, secret)
	defer joiner.stop()

	// The address everybody would be told about, and which goes nowhere from
	// where these two are standing.
	dead := &net.UDPAddr{IP: net.IPv4(203, 0, 113, 9), Port: 41001}

	invite := RoomInvite{Endpoint: dead, Secret: secret, Host: host.room.identity.Public()}
	joinCtx, stop := context.WithTimeout(ctx, 20*time.Second)
	defer stop()

	go func() { _ = joiner.room.Join(joinCtx, invite, 20*time.Second) }()

	// The working address arrives the way a roster or a hello delivers one.
	waitUntil(t, "the joiner to know about the host", func() bool {
		_, known := joiner.room.member(host.room.identity.Public())
		return known
	})
	member, _ := joiner.room.member(host.room.identity.Public())
	member.candidates.consider(host.addr)

	// Health is not the signal: a session that has never heard anything reports
	// itself alive, because silence before the first word is not trouble.
	waitUntil(t, "the pair to open on the candidate that works", func() bool {
		return member.session.established()
	})

	if peer := member.session.Peer(); !sameEndpoint(peer, host.addr) {
		t.Fatalf("settled on %s, want the address that answers (%s)", peer, host.addr)
	}
}

// A datagram from an address no route claims has to be offered to the member
// sessions before it is dropped. Without that, the address a pair proves works
// is thrown away: greet cannot open a pair-key frame, and nothing else looks.
func TestARoomAdoptsAFrameFromAnUnroutedAddress(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	secret, err := NewSecret()
	if err != nil {
		t.Fatal(err)
	}

	host := newNode(t, ctx, secret)
	defer host.stop()

	// A member the host knows of but has no path to, the way a roster leaves
	// somebody: named, with an address that goes nowhere.
	other, err := NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	dead := &net.UDPAddr{IP: net.IPv4(203, 0, 113, 9), Port: 41001}
	member, _, err := host.room.ensureMember(ctx, other.Public(), dead)
	if err != nil {
		t.Fatal(err)
	}

	// They punch from an address the host was never told about and no route
	// claims. It has to reach their session anyway.
	stranger := listen(t)
	defer stranger.Close()

	codec, err := other.PairCodec(host.room.identity.Public(), secret)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stranger.WriteToUDP(codec.Seal(kindPunch, pad()), host.addr); err != nil {
		t.Fatal(err)
	}

	waitUntil(t, "the host to settle on the address that reached it", func() bool {
		return sameEndpoint(member.session.Peer(), localAddr(t, stranger))
	})
}

// A path that is already working must not be moved by somebody else holding the
// same invite. That is the hijack this guards against, and it is the reason the
// rule above only applies while a path is still opening.
func TestALivePathIsNotMoved(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	secret, err := NewSecret()
	if err != nil {
		t.Fatal(err)
	}

	host := newNode(t, ctx, secret)
	defer host.stop()
	joiner := newNode(t, ctx, secret)
	defer joiner.stop()

	invite := RoomInvite{Endpoint: host.addr, Secret: secret, Host: host.room.identity.Public()}
	if err := joiner.room.Join(ctx, invite, 20*time.Second); err != nil {
		t.Fatal(err)
	}

	member, known := host.room.member(joiner.room.identity.Public())
	if !known {
		t.Fatal("the host never learned about the joiner")
	}
	settled := member.session.Peer()

	// The joiner's own key, from a different address, on a live path.
	stranger := listen(t)
	defer stranger.Close()

	codec, err := joiner.room.identity.PairCodec(host.room.identity.Public(), secret)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stranger.WriteToUDP(codec.Seal(kindMessage, "movete"), host.addr); err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)

	if !sameEndpoint(member.session.Peer(), settled) {
		t.Fatalf("a live path moved from %s to %s", settled, member.session.Peer())
	}
}

// Candidates are bounded, deduplicated and only rotated while nothing answers.
func TestCandidatesAreBoundedAndDeduplicated(t *testing.T) {
	first := &net.UDPAddr{IP: net.IPv4(203, 0, 113, 1), Port: 1}
	set := newCandidates(first)

	if set.consider(first) {
		t.Fatal("the same address was added twice")
	}
	if set.consider(&net.UDPAddr{IP: net.IPv4(203, 0, 113, 1), Port: 1}) {
		t.Fatal("an equal address was added again")
	}
	if set.consider(nil) {
		t.Fatal("nothing was added as an address")
	}

	for i := 2; i < 20; i++ {
		set.consider(&net.UDPAddr{IP: net.IPv4(203, 0, 113, byte(i)), Port: 1})
	}
	if got := len(set.all()); got > maxCandidates {
		t.Fatalf("a padded roster grew the set to %d", got)
	}

	// Rotation must come back around rather than run off the end.
	seen := map[string]bool{}
	for i := 0; i < len(set.all())*2; i++ {
		seen[set.next().String()] = true
	}
	if len(seen) != len(set.all()) {
		t.Fatalf("rotation visited %d of %d candidates", len(seen), len(set.all()))
	}

	// One candidate means there is nothing to rotate to.
	if only := newCandidates(first); only.next() != nil {
		t.Fatal("a single candidate rotated to something")
	}
}

// Two sides rotating through candidates can stay out of phase forever: each
// tries the address the other has just left. What breaks the loop is that a
// punch which opens under the pair key is proof the address it came from works,
// and only the two of them hold that key.
//
// This is the one place a session follows an address it was not expecting, and
// it is deliberately narrow: it applies only while no path exists.
func TestAnAuthenticatedPunchSettlesTheAddressBeforeAPathExists(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	secret, err := NewSecret()
	if err != nil {
		t.Fatal(err)
	}
	here, err := NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	there, err := NewIdentity()
	if err != nil {
		t.Fatal(err)
	}

	conn := listen(t)
	defer conn.Close()
	mux := NewMux(conn)
	go mux.Run(ctx)

	mine, err := here.PairCodec(there.Public(), secret)
	if err != nil {
		t.Fatal(err)
	}
	theirs, err := there.PairCodec(here.Public(), secret)
	if err != nil {
		t.Fatal(err)
	}

	session := NewSession(mux, mine, nil)
	// Aimed at the candidate that does not work, the way a roster leaves it.
	session.SetPeer(&net.UDPAddr{IP: net.IPv4(203, 0, 113, 9), Port: 41001})
	mux.Fallback(session)
	go session.Run(ctx)

	// Their punch arrives from the candidate that does work.
	peer := listen(t)
	defer peer.Close()
	if _, err := peer.WriteToUDP(theirs.Seal(kindPunch, pad()), localAddr(t, conn)); err != nil {
		t.Fatal(err)
	}

	waitUntil(t, "the session to settle where the punch came from", func() bool {
		return sameEndpoint(session.Peer(), localAddr(t, peer))
	})
}
