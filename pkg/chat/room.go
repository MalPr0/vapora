package chat

import (
	"sync"

	"github.com/MalPr0/vapora/pkg/names"
	"github.com/MalPr0/vapora/pkg/punch"
)

// Speaker is a participant as a conversation sees them: the transport's member
// plus the one thing the transport has no business deciding, what to call them.
type Speaker struct {
	punch.Member
	Name string
}

// Group is a chat among everyone in a room.
type Group struct {
	room *punch.Room

	mu       sync.RWMutex
	onLine   func(from Speaker, line string)
	onTyping func(from Speaker, active bool)
}

// In wraps a room the same way Over wraps a session.
func In(room *punch.Room) *Group {
	group := &Group{room: room}
	room.Observe(punch.RoomObserverFunc(group.receive))
	return group
}

// OnLine is called for each line somebody says, with who said it. Unlike a
// two-way conversation there is no default speaker, so every line is named.
func (g *Group) OnLine(handler func(from Speaker, line string)) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.onLine = handler
}

// OnTyping reports who is currently writing. Several people can be at once,
// so this is a fact about one member rather than about the room.
func (g *Group) OnTyping(handler func(from Speaker, active bool)) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.onTyping = handler
}

// Say sends a line to everyone. It goes out once per pair channel, sealed
// separately for each, so no member can rewrite what another said.
func (g *Group) Say(text string) {
	g.room.Broadcast(encode(tagLine, safeLine(text)))
}

// SetTyping tells everyone whether a line is in progress here. It goes to each
// member down their own pair channel, like everything else.
func (g *Group) SetTyping(active bool) {
	g.room.Broadcast(typingPayload(active))
}

// Speakers is everyone present, named.
//
// The names are worked out from the whole set rather than one key at a time:
// somebody is "OTTER" until a second otter arrives, and then both become
// "CRIMSON OTTER" and "JADE OTTER". Everyone computes it from the same roster,
// so the names agree without being negotiated.
func (g *Group) Speakers() []Speaker {
	members := g.room.Members()

	keys := make([]names.Key, 0, len(members)+1)
	keys = append(keys, g.room.Me().Key)
	for _, member := range members {
		keys = append(keys, member.Key)
	}
	short := names.Short(keys)

	speakers := make([]Speaker, 0, len(members))
	for _, member := range members {
		speakers = append(speakers, Speaker{Member: member, Name: short[member.Key]})
	}
	return speakers
}

// Me is this side, named the same way.
func (g *Group) Me() Speaker {
	return Speaker{Member: g.room.Me(), Name: g.nameOf(g.room.Me().Key)}
}

// nameOf names one member against everyone currently present, so a name means
// the same thing everywhere it appears.
func (g *Group) nameOf(key punch.PublicKey) string {
	keys := []names.Key{g.room.Me().Key}
	for _, member := range g.room.Members() {
		keys = append(keys, member.Key)
	}
	if name, ok := names.Short(keys)[key]; ok {
		return name
	}
	return names.Of(key, 1)
}

func (g *Group) receive(from punch.Member, payload []byte) {
	tag, body, ok := decode(payload)
	if !ok {
		return
	}

	speaker := Speaker{Member: from, Name: g.nameOf(from.Key)}

	g.mu.RLock()
	onLine, onTyping := g.onLine, g.onTyping
	g.mu.RUnlock()

	switch tag {
	case tagLine:
		shown, ok := line(body)
		if !ok || onLine == nil {
			return
		}
		onLine(speaker, shown)
	case tagTyping:
		if onTyping != nil {
			onTyping(speaker, typing(body))
		}
	}
}
