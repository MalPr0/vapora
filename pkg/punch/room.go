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

var ErrRoomFull = errors.New("punch: the room is full")

// Member is one participant as the room sees them.
type Member struct {
	Key    PublicKey
	Name   string
	Addr   *net.UDPAddr
	Health Health
}

// RoomObserver receives what happens in a room. Unlike a session's, every
// callback names who it came from: a conversation with more than two people in
// it has no default speaker.
type RoomObserver interface {
	Message(from Member, payload string)
	Typing(from Member, active bool)
}

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

func (r *Room) Observe(observer RoomObserver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.observer = observer
}

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
	keys := make([]PublicKey, 0, len(r.members)+1)
	keys = append(keys, r.identity.Public())
	for key := range r.members {
		keys = append(keys, key)
	}
	names := ShortNames(keys)

	members := make([]Member, 0, len(r.members))
	for key, entry := range r.members {
		members = append(members, Member{
			Key:    key,
			Name:   names[key],
			Addr:   entry.addr(),
			Health: entry.session.Health(),
		})
	}
	return members
}

// Me is this participant, named the same way everyone else names them.
func (r *Room) Me() Member {
	r.mu.RLock()
	defer r.mu.RUnlock()

	keys := make([]PublicKey, 0, len(r.members)+1)
	keys = append(keys, r.identity.Public())
	for key := range r.members {
		keys = append(keys, key)
	}
	return Member{Key: r.identity.Public(), Name: ShortNames(keys)[r.identity.Public()]}
}

// Broadcast says one line to everyone. There is no relaying: it goes out once
// per pair channel, which is what keeps any one member from being able to
// rewrite what another said.
func (r *Room) Broadcast(line string) {
	for _, entry := range r.each() {
		entry.session.SendMessage(line)
	}
}

func (r *Room) SetTyping(active bool) {
	for _, entry := range r.each() {
		entry.session.SetTyping(active)
	}
}

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
	return Member{Key: key, Name: key.Nickname()}
}

// memberObserver puts the sender back on what a session reports, which a
// session cannot do on its own: it only ever has one peer.
type memberObserver struct {
	room *Room
	key  PublicKey
}

func (o memberObserver) Message(payload string) {
	if observer := o.room.events(); observer != nil {
		observer.Message(o.room.describe(o.key), payload)
	}
}

func (o memberObserver) Typing(active bool) {
	if observer := o.room.events(); observer != nil {
		observer.Typing(o.room.describe(o.key), active)
	}
}

const roomInfo = "vapora room v1"

const (
	joinTimeout = 3 * time.Minute
	// rosterSpacing staggers the introductions so a newcomer does not arrive
	// as a burst at every member at once.
	rosterSpacing = 200 * time.Millisecond
)
