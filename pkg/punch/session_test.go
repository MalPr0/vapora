package punch

import (
	"bytes"
	"context"
	"io"
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

	session := wired(t, conn, plainCodec{}, &syncBuffer{})
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
	leftSession, rightSession := wired(t, left, plainCodec{}, leftOutput), wired(t, right, plainCodec{}, rightOutput)
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

	waitingSession := wired(t, waiting, plainCodec{}, &syncBuffer{})
	joiningSession := wired(t, joining, plainCodec{}, &syncBuffer{})
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

	leftSession, rightSession := wired(t, left, plainCodec{}, &syncBuffer{}), wired(t, right, plainCodec{}, &syncBuffer{})
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

// wired puts a session behind a mux, which is the only arrangement it supports:
// a session never reads its own socket. Extra sinks are registered ahead of it,
// the way the STUN watcher is in the real thing.
func wired(t *testing.T, conn *net.UDPConn, codec Codec, output io.Writer, ahead ...Sink) *Session {
	t.Helper()

	mux := NewMux(conn)
	session := NewSession(mux, codec, output)
	for _, sink := range ahead {
		mux.Fallback(sink)
	}
	mux.Fallback(session)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go mux.Run(ctx)
	return session
}
