package chat

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/MalPr0/vapora/pkg/punch"
)

// A group of real rooms over loopback, assembled the way anybody importing
// this would: exported API only, no reaching inside.
func group(t *testing.T, count int) []*Group {
	t.Helper()

	secret, err := punch.NewSecret()
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	host, hostAddr := room(t, ctx, secret)
	groups := []*Group{In(host)}

	invite := host.Invite(hostAddr)
	for i := 1; i < count; i++ {
		joiner, _ := room(t, ctx, secret)
		joining, stop := context.WithTimeout(ctx, 20*time.Second)
		if err := joiner.Join(joining, invite, 20*time.Second); err != nil {
			stop()
			t.Fatalf("member %d never got in: %v", i, err)
		}
		stop()
		groups = append(groups, In(joiner))
	}

	// Everyone has to know about everyone before a test can mean anything.
	waitFor(t, "the room to converge", func() bool {
		for _, one := range groups {
			if len(one.Speakers()) != count-1 {
				return false
			}
		}
		return true
	})
	return groups
}

func room(t *testing.T, ctx context.Context, secret punch.Secret) (*punch.Room, *net.UDPAddr) {
	t.Helper()

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })

	identity, err := punch.NewIdentity()
	if err != nil {
		t.Fatal(err)
	}

	mux := punch.NewMux(conn)
	built, err := punch.NewRoom(punch.RoomOptions{Identity: identity, Secret: secret, Mux: mux})
	if err != nil {
		t.Fatal(err)
	}

	go mux.Run(ctx)
	return built, conn.LocalAddr().(*net.UDPAddr)
}

func waitFor(t *testing.T, what string, check func() bool) {
	t.Helper()

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("timed out waiting for " + what)
}

func TestALineReachesEveryoneInTheRoom(t *testing.T) {
	groups := group(t, 3)

	heard := make(chan Speaker, 8)
	for _, listener := range groups[1:] {
		listener.OnLine(func(from Speaker, line string) {
			if line == "hola a todos" {
				heard <- from
			}
		})
	}

	groups[0].Say("hola a todos")

	names := map[string]bool{}
	for i := 0; i < 2; i++ {
		select {
		case from := <-heard:
			if from.Name == "" {
				t.Fatal("a line arrived from somebody with no name")
			}
			names[from.Name] = true
		case <-time.After(10 * time.Second):
			t.Fatalf("only %d of 2 members heard it", i)
		}
	}
	if len(names) != 1 {
		t.Fatalf("the same speaker was named differently by each listener: %v", names)
	}
}

// Names are derived from keys and computed against everybody present, so the
// same person is called the same thing on every screen — without any of it
// being sent.
func TestEverybodyAgreesOnEverybodysName(t *testing.T) {
	groups := group(t, 3)

	// What each side calls itself, against what the others call it.
	for _, side := range groups {
		mine := side.Me()
		if mine.Name == "" {
			t.Fatal("a member has no name for itself")
		}

		for _, other := range groups {
			if other.Me().Key == mine.Key {
				continue
			}
			var found bool
			for _, speaker := range other.Speakers() {
				if speaker.Key == mine.Key {
					found = true
					if speaker.Name != mine.Name {
						t.Fatalf("%s calls itself %q and is called %q elsewhere",
							mine.Key, mine.Name, speaker.Name)
					}
				}
			}
			if !found {
				t.Fatalf("%s is missing from somebody's roster", mine.Key)
			}
		}
	}
}

func TestTypingIsAttributedToWhoIsTyping(t *testing.T) {
	groups := group(t, 2)

	typing := make(chan Speaker, 4)
	groups[1].OnTyping(func(from Speaker, active bool) {
		if active {
			typing <- from
		}
	})

	groups[0].SetTyping(true)

	select {
	case from := <-typing:
		if from.Key != groups[0].Me().Key {
			t.Fatal("typing was attributed to the wrong member")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("no typing update arrived")
	}
}

// Only text crosses, and it is sanitised on the way out so this program never
// emits a sequence a terminal would act on.
func TestWhatLeavesAGroupIsSafeText(t *testing.T) {
	groups := group(t, 2)

	lines := make(chan string, 1)
	groups[1].OnLine(func(_ Speaker, line string) { lines <- line })

	groups[0].Say("antes\x1b[31m\x07 despues")

	select {
	case got := <-lines:
		if strings.ContainsAny(got, "\x1b\x07") {
			t.Fatalf("an escape sequence crossed: %q", got)
		}
		if !strings.Contains(got, "antes") || !strings.Contains(got, "despues") {
			t.Fatalf("the readable part did not survive: %q", got)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the line never arrived")
	}
}

// The room carries anything; a group has to be sure what it has before showing
// it to anybody.
func TestAGroupIgnoresPayloadsThatAreNotItsOwn(t *testing.T) {
	groups := group(t, 2)

	lines := make(chan string, 1)
	groups[1].OnLine(func(_ Speaker, line string) { lines <- line })

	// Straight down the transport, with a tag this package never uses.
	groups[0].room.Broadcast([]byte{0x7f, 'n', 'o'})
	groups[0].room.Broadcast(nil)

	select {
	case leaked := <-lines:
		t.Fatalf("a payload that is not a chat line surfaced as one: %q", leaked)
	case <-time.After(time.Second):
	}
}

// A handler that was never set must not be a crash, because a caller that only
// cares about lines is a perfectly ordinary caller.
func TestAGroupWithNoHandlersIsHarmless(t *testing.T) {
	groups := group(t, 2)

	groups[0].Say("nobody is listening")
	groups[0].SetTyping(true)
	time.Sleep(500 * time.Millisecond)

	if len(groups[1].Speakers()) != 1 {
		t.Fatal("the room fell apart")
	}
}
