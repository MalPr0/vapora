package chat

import (
	"sync"

	"github.com/MalPr0/vapora/pkg/names"
	"github.com/MalPr0/vapora/pkg/punch"
)

// Conversation is a two-way chat over one punched session.
type Conversation struct {
	session *punch.Session

	mu       sync.RWMutex
	onLine   func(line string)
	onTyping func(active bool)
}

// Over wraps a session. The session keeps doing what it does — punching,
// sealing, measuring the path — and this adds the meaning.
func Over(session *punch.Session) *Conversation {
	conversation := &Conversation{session: session}
	session.Observe(punch.ObserverFunc(conversation.receive))
	return conversation
}

// OnLine and OnTyping are how a UI attaches. Injected rather than imported so
// this package knows nothing about how anything is rendered.
func (c *Conversation) OnLine(handler func(line string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onLine = handler
}

// OnTyping is called when the peer starts or stops writing a line. Purely
// advisory: an indicator that never arrives costs nothing, which is why it is
// sent as ordinary traffic with no acknowledgement.
func (c *Conversation) OnTyping(handler func(active bool)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onTyping = handler
}

// Say sends a line. It is sanitised first, so this program never emits a
// sequence a terminal would act on, whatever the caller handed over.
func (c *Conversation) Say(text string) {
	c.session.Send(encode(tagLine, safeLine(text)))
}

// SetTyping tells the peer whether a line is in progress here.
func (c *Conversation) SetTyping(active bool) {
	c.session.Send(typingPayload(active))
}

// Name is what to call the peer. It comes from their key, so both sides compute
// the same one with nothing sent and nothing to trust.
func (c *Conversation) Name(peer punch.PublicKey) string {
	return names.Of(peer, 2)
}

func (c *Conversation) receive(payload []byte) {
	tag, body, ok := decode(payload)
	if !ok {
		return
	}

	c.mu.RLock()
	onLine, onTyping := c.onLine, c.onTyping
	c.mu.RUnlock()

	switch tag {
	case tagLine:
		shown, ok := line(body)
		if !ok || onLine == nil {
			return
		}
		onLine(shown)
	case tagTyping:
		if onTyping != nil {
			onTyping(typing(body))
		}
	}
}
