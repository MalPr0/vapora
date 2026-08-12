package chat

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/MalPr0/vapora/pkg/punch"
)

// Everything here is built through the transport's exported API and nothing
// else. That is deliberate: if a conversation cannot be assembled from outside
// the package, neither can anything else somebody wants to build on it, and the
// separation would be a claim rather than a fact.
func pair(t *testing.T) (*Conversation, *Conversation) {
	t.Helper()

	secret, err := punch.NewSecret()
	if err != nil {
		t.Fatal(err)
	}

	left, right := socket(t), socket(t)
	leftSession := session(t, left, secret, punch.RoleInviter)
	rightSession := session(t, right, secret, punch.RoleJoiner)

	leftSession.SetPeer(right.LocalAddr().(*net.UDPAddr))
	rightSession.SetPeer(left.LocalAddr().(*net.UDPAddr))

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	go leftSession.Open(ctx, 10*time.Second)
	go rightSession.Open(ctx, 10*time.Second)
	go leftSession.Run(ctx)
	go rightSession.Run(ctx)

	if err := leftSession.Established(withTimeout(ctx, t)); err != nil {
		t.Fatalf("the path never opened: %v", err)
	}
	if err := rightSession.Established(withTimeout(ctx, t)); err != nil {
		t.Fatalf("the path never opened on the other side: %v", err)
	}

	return Over(leftSession), Over(rightSession)
}

func withTimeout(ctx context.Context, t *testing.T) context.Context {
	t.Helper()
	timed, cancel := context.WithTimeout(ctx, 10*time.Second)
	t.Cleanup(cancel)
	return timed
}

func socket(t *testing.T) *net.UDPConn {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func session(t *testing.T, conn *net.UDPConn, secret punch.Secret, role punch.Role) *punch.Session {
	t.Helper()

	codec, err := punch.NewSecretCodec(secret, role)
	if err != nil {
		t.Fatal(err)
	}

	mux := punch.NewMux(conn)
	built := punch.NewSession(mux, codec, nil)
	mux.Fallback(built)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go mux.Run(ctx)

	return built
}

func TestALineCrossesAsItself(t *testing.T) {
	here, there := pair(t)

	lines := make(chan string, 1)
	there.OnLine(func(line string) { lines <- line })

	here.Say("hola, ¿qué tal?")

	select {
	case got := <-lines:
		if got != "hola, ¿qué tal?" {
			t.Fatalf("got %q", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the line never arrived")
	}
}

func TestTypingIsReportedBothWays(t *testing.T) {
	here, there := pair(t)

	typing := make(chan bool, 4)
	there.OnTyping(func(active bool) { typing <- active })

	here.SetTyping(true)
	if !receive(t, typing) {
		t.Fatal("the peer was not told that typing started")
	}

	here.SetTyping(false)
	if receive(t, typing) {
		t.Fatal("the peer was not told that typing stopped")
	}
}

// A typing indicator must not surface as something somebody said, and a line
// must not be mistaken for one. They share a channel and are told apart by a
// tag this package owns.
func TestTagsDoNotLeakIntoEachOther(t *testing.T) {
	here, there := pair(t)

	lines := make(chan string, 4)
	typing := make(chan bool, 4)
	there.OnLine(func(line string) { lines <- line })
	there.OnTyping(func(active bool) { typing <- active })

	here.SetTyping(true)
	if !receive(t, typing) {
		t.Fatal("typing did not arrive")
	}
	select {
	case leaked := <-lines:
		t.Fatalf("a typing indicator surfaced as a line: %q", leaked)
	case <-time.After(300 * time.Millisecond):
	}

	here.Say("una linea")
	select {
	case <-lines:
	case <-time.After(5 * time.Second):
		t.Fatal("the line never arrived")
	}
	select {
	case <-typing:
		t.Fatal("a line surfaced as a typing update")
	case <-time.After(300 * time.Millisecond):
	}
}

// Only text crosses a conversation, and it is sanitised on the way out so this
// program never emits a sequence a terminal would act on.
func TestWhatCrossesIsAlwaysSafeText(t *testing.T) {
	here, there := pair(t)

	lines := make(chan string, 1)
	there.OnLine(func(line string) { lines <- line })

	here.Say("antes\x1b[31mrojo\x07 despues")

	select {
	case got := <-lines:
		if strings.ContainsAny(got, "\x1b\x07") {
			t.Fatalf("an escape sequence crossed: %q", got)
		}
		if !strings.Contains(got, "antes") || !strings.Contains(got, "despues") {
			t.Fatalf("the readable part did not survive: %q", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the line never arrived")
	}
}

// A payload that is not one of ours is ignored rather than guessed at. The
// transport will carry anything, so a conversation has to be sure what it has.
func TestForeignPayloadsAreIgnored(t *testing.T) {
	here, there := pair(t)

	lines := make(chan string, 1)
	there.OnLine(func(line string) { lines <- line })

	// Sent through the transport directly, with a tag this package never uses.
	here.session.Send([]byte{0x7f, 'n', 'o'})
	here.session.Send(nil)

	select {
	case leaked := <-lines:
		t.Fatalf("a payload that was not a chat line surfaced as one: %q", leaked)
	case <-time.After(500 * time.Millisecond):
	}
}

func receive(t *testing.T, updates chan bool) bool {
	t.Helper()
	select {
	case active := <-updates:
		return active
	case <-time.After(5 * time.Second):
		t.Fatal("no typing update arrived")
		return false
	}
}
