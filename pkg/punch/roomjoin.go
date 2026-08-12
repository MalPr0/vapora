package punch

import (
	"context"
	"fmt"
	"net"
	"time"
)

// helloInterval matches the punch cadence: a hello is a punch that also says
// who is arriving.
const helloInterval = punchInterval

// Join enters a room through somebody already in it.
//
// The hello is the only thing sealed with the room key, because it is the only
// thing an arriving member shares with the room. The answer comes back on the
// pair channel, which is what pins the greeter: anybody who saw the invite can
// open a hello, but only the member whose key is on the invite can answer one.
func (r *Room) Join(ctx context.Context, invite RoomInvite, timeout time.Duration) error {
	if invite.Endpoint == nil || invite.Host.isZero() {
		return fmt.Errorf("%w: nothing to join", ErrNotRoomInvite)
	}

	host, _, err := r.ensureMember(ctx, invite.Host, invite.Endpoint)
	if err != nil {
		return err
	}

	joinCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	go r.helloLoop(joinCtx, invite.Endpoint)

	// The path opening is what says the greeter answered under the pair key.
	// ensureMember already started the punching; waiting again here would run a
	// second loop against the same peer.
	if err := host.session.Established(joinCtx); err != nil {
		return fmt.Errorf("punch: nobody answered at %s: %w", invite.Endpoint, err)
	}
	r.sendRoster(host)
	return nil
}

// Reach punches towards an address without joining anything. A room only ever
// answers a hello, so between two networks that refuse a first packet from a
// stranger the newcomer's hello dies at the host's door and the room never
// starts — the same standoff `punch` solves by exchanging two invites. This is
// that exchange: the waiting side is given the newcomer's address and starts
// sending, which is what opens its own filter to them.
//
// It carries no secret and grants nothing. Whoever is on the other end still
// has to produce a hello under the room key to become a member.
func (r *Room) Reach(ctx context.Context, endpoint *net.UDPAddr) {
	if endpoint == nil {
		return
	}
	go r.helloLoop(ctx, endpoint)
}

func (r *Room) helloLoop(ctx context.Context, endpoint *net.UDPAddr) {
	ticker := time.NewTicker(helloInterval)
	defer ticker.Stop()

	for {
		r.sayHello(endpoint)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// greet is the fallback sink: a datagram from an address nobody owns yet. Only
// a hello can come from there, and only under the room key.
func (r *Room) greet(payload []byte, from *net.UDPAddr) bool {
	frame, err := r.roomCode.Open(payload)
	if err != nil {
		return false
	}
	if !allowedUnder(frame.Kind, layerRoom) {
		// Somebody sealed a room frame with the key everyone holds. It is not
		// answered and it is not obeyed.
		return true
	}
	if frame.Kind != kindHello {
		return true
	}

	key, err := ParsePublicKey(inviteEncoding.EncodeToString([]byte(frame.Payload[:min(len(frame.Payload), PublicKeySize)])))
	if err != nil {
		return true
	}

	if local := helloLocal(frame.Payload); local != nil {
		// Only worth keeping when it is a private address on our own network:
		// a public one from here is somebody describing themselves, which the
		// address the datagram actually came from already says better.
		if entry, known := r.member(key); known {
			entry.candidates.consider(local)
		}
	}

	entry, fresh, err := r.ensureMember(context.Background(), key, from)
	if err != nil {
		if err == ErrRoomFull {
			_ = r.mux.Send(r.roomCode.Seal(kindFull, pad()), from)
		}
		return true
	}
	if fresh {
		// Telling the room about the newcomer is the whole job of the member
		// who invited them: it makes both sides punch at once, which is what a
		// restrictive NAT needs, and then it stays out of the way.
		go r.introduce(key, from)
	}
	r.sendRoster(entry)
	return true
}

// introduce names a newcomer to everyone else, and vice versa by way of the
// roster the newcomer already got.
func (r *Room) introduce(key PublicKey, addr *net.UDPAddr) {
	local := (*net.UDPAddr)(nil)
	if member, known := r.member(key); known {
		local = member.candidates.localOf(addr)
	}
	entry := Roster{{Key: key, Addr: addr, Local: local}}.Marshal()

	for _, member := range r.each() {
		if member.key == key {
			continue
		}
		member.session.send(kindIntro, entry)
		time.Sleep(rosterSpacing)
	}
}

func (r *Room) sendRoster(to *roomMember) {
	to.session.send(kindWelcome, r.roster().Marshal())
}

// roomFrame handles what a session does not know about, which is everything the
// room says over a pair channel.
func (r *Room) roomFrame(ctx context.Context, from PublicKey, frame Opened) bool {
	if !allowedUnder(frame.Kind, layerPair) {
		return true
	}

	switch frame.Kind {
	case kindWelcome, kindRoster:
		r.merge(ctx, frame.Payload)
	case kindIntro:
		r.merge(ctx, frame.Payload)
	default:
		return false
	}
	return true
}

// merge takes what somebody else says about the room and punches at anyone new.
// Nothing here is believed: an entry is an address to try, and only a frame
// that opens under the pair key makes anybody a member.
func (r *Room) merge(ctx context.Context, payload string) {
	roster, err := ParseRoster(payload, r.max)
	if err != nil {
		return
	}

	for _, entry := range roster {
		if entry.Key == r.identity.Public() {
			continue
		}
		member, _, err := r.ensureMember(ctx, entry.Key, entry.Addr)
		if err != nil {
			continue
		}
		// Every candidate the roster named is worth trying: which one works is
		// a property of the two networks, not something either side can know.
		for _, candidate := range entry.Candidates() {
			member.candidates.consider(candidate)
		}
	}
}

// Bytes is the wire form of a key, which the hello carries raw rather than
// encoded because a datagram has no reason to pay for base32.
// helloLocal reads the address a hello carried after the key, if any.
func helloLocal(payload string) *net.UDPAddr {
	if len(payload) < PublicKeySize+candidateBytes {
		return nil
	}
	return readCandidate([]byte(payload[PublicKeySize : PublicKeySize+candidateBytes]))
}

// Bytes is the raw key, for putting on the wire.
func (k PublicKey) Bytes() []byte {
	out := make([]byte, PublicKeySize)
	copy(out, k[:])
	return out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
