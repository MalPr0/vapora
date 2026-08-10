package punch

import (
	"bytes"
	"context"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	kind, payload, err := decode(encode(kindMessage, "hola & chau"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kind != kindMessage || payload != "hola & chau" {
		t.Fatalf("got kind %d payload %q", kind, payload)
	}

	if _, _, err := decode(nil); err == nil {
		t.Fatal("expected an error for an empty frame")
	}
}

func TestParseInviteAcceptsCommandOrEndpoint(t *testing.T) {
	for _, line := range []string{
		"vapora punch 203.0.113.7:41001",
		"go run ./cmd/vapora punch 203.0.113.7:41001",
		"  203.0.113.7:41001  ",
	} {
		invite, err := ParseInvite(line)
		if err != nil {
			t.Fatalf("%q: %v", line, err)
		}
		if invite.Endpoint.String() != "203.0.113.7:41001" {
			t.Fatalf("%q gave %s", line, invite.Endpoint)
		}
		if len(invite.Secret) != 0 {
			t.Fatalf("%q should carry no secret", line)
		}
	}

	for _, line := range []string{"", "not an endpoint", "203.0.113.7"} {
		if _, err := ParseInvite(line); err == nil {
			t.Fatalf("%q should have been rejected", line)
		}
	}
}

func TestInviteRoundTripsWithSecret(t *testing.T) {
	secret, err := NewSecret()
	if err != nil {
		t.Fatalf("cannot generate a secret: %v", err)
	}

	invite := Invite{Endpoint: &net.UDPAddr{IP: net.IPv4(203, 0, 113, 7), Port: 41001}, Secret: secret}
	command := invite.Command("vapora punch")
	if !strings.HasPrefix(command, "vapora punch 203.0.113.7:41001/") {
		t.Fatalf("got %q", command)
	}

	parsed, err := ParseInvite(command)
	if err != nil {
		t.Fatalf("an invite must parse back: %v", err)
	}
	if parsed.Endpoint.String() != invite.Endpoint.String() {
		t.Fatalf("endpoint became %s", parsed.Endpoint)
	}
	if !parsed.Secret.Equal(secret) {
		t.Fatal("the secret did not survive the round trip")
	}
}

func TestParseInviteRejectsBrokenSecret(t *testing.T) {
	for _, line := range []string{
		"vapora punch 203.0.113.7:41001/not-base32",
		"vapora punch 203.0.113.7:41001/AAAA",
	} {
		if _, err := ParseInvite(line); err == nil {
			t.Fatalf("%q should have been rejected", line)
		}
	}
}

func TestOpenTimesOutWithoutPeer(t *testing.T) {
	conn := listen(t)
	defer conn.Close()

	session := NewSession(conn, PlainCodec{}, &syncBuffer{})
	session.SetPeer(&net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: freePort(t)})

	if err := session.Open(context.Background(), 400*time.Millisecond); err != ErrPunchTimeout {
		t.Fatalf("got %v", err)
	}
}

func TestOpenAndExchangeMessages(t *testing.T) {
	left, right := listen(t), listen(t)
	defer left.Close()
	defer right.Close()

	leftOutput, rightOutput := &syncBuffer{}, &syncBuffer{}
	leftSession, rightSession := NewSession(left, PlainCodec{}, leftOutput), NewSession(right, PlainCodec{}, rightOutput)
	leftSession.SetPeer(localAddr(t, right))
	rightSession.SetPeer(localAddr(t, left))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	openBoth(t, ctx, leftSession, rightSession)

	go leftSession.Run(ctx)
	go rightSession.Run(ctx)

	leftSession.SendMessage("ping")
	rightSession.SendMessage("pong")

	waitFor(t, rightOutput, "<peer> ping")
	waitFor(t, leftOutput, "<peer> pong")
}

// The waiting side knows nothing about the peer until its first packet lands,
// which is the one way invite flow.
func TestOpenLearnsPeerFromIncomingPacket(t *testing.T) {
	waiting, joining := listen(t), listen(t)
	defer waiting.Close()
	defer joining.Close()

	waitingSession := NewSession(waiting, PlainCodec{}, &syncBuffer{})
	joiningSession := NewSession(joining, PlainCodec{}, &syncBuffer{})
	joiningSession.SetPeer(localAddr(t, waiting))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	openBoth(t, ctx, waitingSession, joiningSession)

	learned := waitingSession.Peer()
	if learned == nil || learned.Port != localAddr(t, joining).Port {
		t.Fatalf("the waiting side learned %v", learned)
	}
}

func TestSetPeerWhileOpenIsPickedUp(t *testing.T) {
	left, right := listen(t), listen(t)
	defer left.Close()
	defer right.Close()

	leftSession, rightSession := NewSession(left, PlainCodec{}, &syncBuffer{}), NewSession(right, PlainCodec{}, &syncBuffer{})
	rightSession.SetPeer(localAddr(t, left))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Left has no peer yet: it is pasted in while Open is already running.
	time.AfterFunc(200*time.Millisecond, func() { leftSession.SetPeer(localAddr(t, right)) })
	openBoth(t, ctx, leftSession, rightSession)
}

func openBoth(t *testing.T, ctx context.Context, sessions ...*Session) {
	t.Helper()

	var wg sync.WaitGroup
	errs := make([]error, len(sessions))
	for i, session := range sessions {
		wg.Add(1)
		go func(index int, s *Session) {
			defer wg.Done()
			errs[index] = s.Open(ctx, 5*time.Second)
		}(i, session)
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			t.Fatalf("open failed: %v", err)
		}
	}
}

func waitFor(t *testing.T, output *syncBuffer, want string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(output.String(), want) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("never received %q, got %q", want, output.String())
}

func listen(t *testing.T) *net.UDPConn {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("cannot listen: %v", err)
	}
	return conn
}

func localAddr(t *testing.T, conn *net.UDPConn) *net.UDPAddr {
	t.Helper()
	address, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("unexpected address type %T", conn.LocalAddr())
	}
	return address
}

func freePort(t *testing.T) int {
	t.Helper()
	conn := listen(t)
	port := localAddr(t, conn).Port
	conn.Close()
	return port
}

type syncBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *syncBuffer) Write(payload []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(payload)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}
