package punch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// ErrRoomFull means the cap has been reached. The newcomer is told rather
// than ignored, so it can say so instead of waiting out a timeout.
var ErrRoomFull = errors.New("punch: the room is full")

// Member is one participant as the room sees them.
//
// There is no name here on purpose. What to call somebody is a question for
// whoever is showing them to a person, and an application with its own idea of
// identity should not have to work around this one.
type Member struct {
	Key    PublicKey
	Addr   *net.UDPAddr
	Health Health
}

// RoomObserver receives what happens in a room. Unlike a session's, it names
// who it came from: with more than two people there is no default sender.
type RoomObserver interface {
	// Data is a payload one member sent, exactly as they sent it.
	Data(from Member, payload []byte)
}

// RoomObserverFunc adapts a function.
type RoomObserverFunc func(from Member, payload []byte)

// Data calls the function.
func (f RoomObserverFunc) Data(from Member, payload []byte) { f(from, payload) }

type roomMember struct {
	key        PublicKey
	session    *Session
	candidates *candidates
}

// addr is where this member is currently believed to be. Once a path is open
// the session knows better than the roster does, because it has proof.
func (m *roomMember) addr() *net.UDPAddr {
	if peer := m.session.Peer(); peer != nil {
		return peer
	}
	return m.candidates.at()
}

// Room is a mesh of pair channels. Every member talks to every other one
// directly: whoever introduces two people never carries a word between them,
// and could not read it if it tried.
type Room struct {
	identity *Identity
	secret   Secret
	roomCode Codec
	local    *net.UDPAddr
	mux      *Mux
	output   io.Writer
	max      int

	mu       sync.RWMutex
	members  map[PublicKey]*roomMember
	observer RoomObserver
}

// RoomOptions is what a room needs to exist. Identity and Mux are required;
// the rest have workable defaults.
type RoomOptions struct {
	Identity *Identity
	Secret   Secret
	Mux      *Mux
	Output   io.Writer
	// Local is where this side sits on its own network. It travels with every
	// announcement so that somebody behind the same router has an address that
	// does not need the router to turn a packet around.
	Local *net.UDPAddr
	// Max caps the room. Zero uses MaxMembers.
	Max int
}

// NewRoom builds a mesh of pair channels.
//
// It registers its own sinks on the mux, so it must be built before anything
// that should see datagrams after it. Nothing happens on the network until
// Join is called or somebody says hello.
func NewRoom(opts RoomOptions) (*Room, error) {
	if opts.Identity == nil || opts.Mux == nil {
		return nil, errors.New("punch: a room needs an identity and a mux")
	}
	if len(opts.Secret) == 0 {
		return nil, errors.New("punch: a room needs a secret")
	}
	if opts.Output == nil {
		opts.Output = io.Discard
	}
	if opts.Max <= 0 {
		opts.Max = MaxMembers
	}

	// The room key seals exactly one thing: the hello of somebody arriving.
	// It is symmetric on purpose, unlike a pair channel: a hello is meant for
	// whoever is listening, so there is no "other direction" to key. Everyone
	// ever handed the invite can open one, which is precisely why nothing else
	// travels under it.
	roomKey, err := deriveKey(opts.Secret, nil, roomInfo)
	if err != nil {
		return nil, err
	}
	roomCode, err := newCodec(roomKey, roomKey)
	if err != nil {
		return nil, err
	}

	room := &Room{
		identity: opts.Identity,
		secret:   opts.Secret,
		roomCode: roomCode,
		mux:      opts.Mux,
		output:   opts.Output,
		max:      opts.Max,
		local:    opts.Local,
		members:  map[PublicKey]*roomMember{},
	}
	opts.Mux.Fallback(SinkFunc(room.greet))
	opts.Mux.Fallback(SinkFunc(room.adopt))
	return room, nil
}

// Observe sets who receives what members send. One at a time; the last call
// wins.
func (r *Room) Observe(observer RoomObserver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.observer = observer
}

// Identity is this participant's keypair.
func (r *Room) Identity() *Identity { return r.identity }

// Invite is what this member hands out. Any member can issue one: it carries
// the room secret it already holds and its own key, so the newcomer knows who
// it is talking to.
func (r *Room) Invite(endpoint *net.UDPAddr) RoomInvite {
	return RoomInvite{Endpoint: endpoint, Secret: r.secret, Host: r.identity.Public()}
}

// Members lists who is present, with the short names everyone agrees on.
func (r *Room) Members() []Member {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.snapshot()
}

func (r *Room) snapshot() []Member {
	members := make([]Member, 0, len(r.members))
	for key, entry := range r.members {
		members = append(members, Member{
			Key:    key,
			Addr:   entry.addr(),
			Health: entry.session.Health(),
		})
	}
	return members
}

// Me is this participant.
func (r *Room) Me() Member {
	return Member{Key: r.identity.Public()}
}

// Broadcast sends the same payload to everyone. There is no relaying: it goes
// out once per pair channel, sealed separately for each, which is what keeps
// any one member from being able to rewrite what another said.
func (r *Room) Broadcast(payload []byte) {
	for _, entry := range r.each() {
		entry.session.Send(payload)
	}
}

// SendTo says something to one member and to nobody else. The other members
// cannot read it: they do not have that pair's key.
func (r *Room) SendTo(key PublicKey, payload []byte) bool {
	member, known := r.member(key)
	if !known {
		return false
	}
	member.session.Send(payload)
	return true
}

// Goodbye tells everyone this side is leaving, so they can drop it at once
// instead of waiting out the whole silence budget to find out.
func (r *Room) Goodbye() {
	for _, entry := range r.each() {
		entry.session.Goodbye()
	}
}

func (r *Room) each() []*roomMember {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entries := make([]*roomMember, 0, len(r.members))
	for _, entry := range r.members {
		entries = append(entries, entry)
	}
	return entries
}

// roster is what this member tells others about who is present.
func (r *Room) roster() Roster {
	r.mu.RLock()
	defer r.mu.RUnlock()

	roster := make(Roster, 0, len(r.members))
	for key, entry := range r.members {
		roster = append(roster, Entry{
			Key:   key,
			Addr:  entry.addr(),
			Local: entry.candidates.localOf(entry.addr()),
		})
	}
	return roster
}

// adopt offers a datagram from an unclaimed address to every member session.
//
// Without this a member that moves — a new address from the router, or a second
// candidate that turns out to be the one that works — goes silent: its frames
// arrive from an address no route claims, greet cannot open them because they
// are sealed under a pair key rather than the room key, and they are dropped
// without a word.
//
// Offering them around is safe because only the session holding the right pair
// key can open one. The address is learned from proof, not from anybody's
// say-so, and a frame that opens nowhere costs at most one failed decryption
// per member, which the room's size caps.
func (r *Room) adopt(payload []byte, from *net.UDPAddr) bool {
	r.mu.RLock()
	members := make([]*roomMember, 0, len(r.members))
	for _, member := range r.members {
		members = append(members, member)
	}
	r.mu.RUnlock()

	for _, member := range members {
		if member.session.Deliver(payload, from) {
			// The session followed the move, so the address is worth a route:
			// the next frame from it skips this whole search.
			_ = r.mux.Route(from, member.session)
			return true
		}
	}
	return false
}

// ensureMember is the only way a member comes into being, so joining twice,
// being introduced by two people at once and meeting in the middle all end up
// at the same single session.
func (r *Room) ensureMember(ctx context.Context, key PublicKey, addr *net.UDPAddr) (*roomMember, bool, error) {
	if key == r.identity.Public() {
		return nil, false, fmt.Errorf("%w: that is this member", ErrBadPublicKey)
	}

	r.mu.Lock()
	if existing, known := r.members[key]; known {
		r.mu.Unlock()
		// A known member reached from somewhere new is another candidate, not a
		// move: the roster says where to try and only the pair key settles it.
		existing.candidates.consider(addr)
		return existing, false, nil
	}
	if len(r.members) >= r.max {
		r.mu.Unlock()
		return nil, false, ErrRoomFull
	}
	r.mu.Unlock()

	codec, err := r.identity.PairCodec(key, r.secret)
	if err != nil {
		return nil, false, err
	}

	session := NewSession(r.mux, codec, r.output)
	session.SetPeer(addr)
	session.Observe(memberObserver{room: r, key: key})
	session.Extra(func(frame Opened) bool { return r.roomFrame(ctx, key, frame) })

	entry := &roomMember{key: key, session: session, candidates: newCandidates(addr)}

	r.mu.Lock()
	if existing, known := r.members[key]; known {
		r.mu.Unlock()
		return existing, false, nil
	}
	r.members[key] = entry
	r.mu.Unlock()

	// A route is a claim on an address, and losing the race is not an error:
	// the other claimant authenticated first and this session still reaches the
	// peer through the fallback chain.
	_ = r.mux.Route(addr, session)

	go session.Open(ctx, joinTimeout)
	go session.Run(ctx)
	go r.rotate(ctx, entry)
	return entry, true, nil
}

// member looks one up without creating it.
func (r *Room) member(key PublicKey) (*roomMember, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, known := r.members[key]
	return entry, known
}

func (r *Room) events() RoomObserver {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.observer
}

func (r *Room) describe(key PublicKey) Member {
	for _, member := range r.Members() {
		if member.Key == key {
			return member
		}
	}
	return Member{Key: key}
}

// memberObserver puts the sender back on what a session reports, which a
// session cannot do on its own: it only ever has one peer.
type memberObserver struct {
	room *Room
	key  PublicKey
}

func (o memberObserver) Data(payload []byte) {
	if observer := o.room.events(); observer != nil {
		observer.Data(o.room.describe(o.key), payload)
	}
}

const roomInfo = "vapora room v1"

const (
	joinTimeout = 3 * time.Minute
	// rosterSpacing staggers the introductions so a newcomer does not arrive
	// as a burst at every member at once.
	rosterSpacing = 200 * time.Millisecond
)
