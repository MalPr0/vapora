package tcpchat

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MalPr0/vapora/pkg/punch"
)

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

func secret(t *testing.T) punch.Secret {
	t.Helper()

	value, err := punch.NewSecret()
	if err != nil {
		t.Fatalf("cannot generate a secret: %v", err)
	}
	return value
}

// host starts a server on a free port and returns where to reach it.
func host(t *testing.T, key punch.Secret, output io.Writer, input io.Reader) string {
	t.Helper()

	server := NewServer(output, key)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	listener, err := server.Listen(ctx, "127.0.0.1:0")
	if err != nil {
		t.Fatalf("cannot listen: %v", err)
	}
	go server.Serve(ctx, listener, input)
	return listener.Addr().String()
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
	t.Fatalf("never saw %q, got %q", want, output.String())
}

func TestChatCarriesLinesBothWays(t *testing.T) {
	key := secret(t)
	serverOut, clientOut := &syncBuffer{}, &syncBuffer{}

	// Both sides type into pipes: a host writing before anyone connects has
	// nowhere to put it, and a client whose input ends closes the session on
	// purpose, which would end this one before the reply arrived.
	hostInput, hostTypes := io.Pipe()
	clientInput, clientTypes := io.Pipe()
	address := host(t, key, serverOut, hostInput)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go Dial(ctx, address, key, clientInput, clientOut)

	waitFor(t, serverOut, "connected")
	go clientTypes.Write([]byte("desde el cliente\n"))
	go hostTypes.Write([]byte("desde el host\n"))

	waitFor(t, serverOut, "desde el cliente")
	waitFor(t, clientOut, "desde el host")
}

// The port is reachable from the internet, so the key is the only thing between
// a stranger and the conversation.
func TestAWrongSecretIsRefused(t *testing.T) {
	output := &syncBuffer{}
	address := host(t, secret(t), output, strings.NewReader(""))

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := Dial(ctx, address, secret(t), strings.NewReader("dejame entrar\n"), io.Discard)
	if err == nil {
		waitFor(t, output, "does not authenticate")
	}
	if strings.Contains(output.String(), "dejame entrar") {
		t.Fatalf("a stranger was heard:\n%s", output.String())
	}
}

// Raw bytes on the socket must not be mistaken for a conversation.
func TestPlainTextIsRefused(t *testing.T) {
	output := &syncBuffer{}
	address := host(t, secret(t), output, strings.NewReader(""))

	conn, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatalf("cannot connect: %v", err)
	}
	defer conn.Close()

	payload := []byte("hola sin cifrar")
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, uint32(len(payload)))
	_, _ = conn.Write(append(header, payload...))

	waitFor(t, output, "does not authenticate")
	if strings.Contains(output.String(), "hola sin cifrar") {
		t.Fatalf("plain text was printed:\n%s", output.String())
	}
}

// A length prefix is chosen by the peer, so believing it is how a chat becomes
// a memory exhaustion primitive.
func TestAnAbsurdLengthIsRefused(t *testing.T) {
	output := &syncBuffer{}
	address := host(t, secret(t), output, strings.NewReader(""))

	conn, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatalf("cannot connect: %v", err)
	}
	defer conn.Close()

	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, 4_000_000_000)
	_, _ = conn.Write(header)

	waitFor(t, output, "larger than the limit")
}

// Whatever the peer sends is printed, so it must not be able to drive the
// terminal with it.
func TestControlSequencesAreStripped(t *testing.T) {
	key := secret(t)
	output := &syncBuffer{}
	address := host(t, key, output, strings.NewReader(""))

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	go Dial(ctx, address, key, strings.NewReader("hola\x1b[2Jchau\n"), io.Discard)

	waitFor(t, output, "hola[2Jchau")
	if strings.Contains(output.String(), "\x1b") {
		t.Fatal("an escape byte reached the output")
	}
}

// A second peer would share the first one's direction key, and two of them
// could collide on a nonce under it.
func TestASecondPeerIsRefused(t *testing.T) {
	key := secret(t)
	output := &syncBuffer{}
	address := host(t, key, output, strings.NewReader(""))

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	go Dial(ctx, address, key, blockingReader{}, io.Discard)
	waitFor(t, output, "connected")

	go Dial(ctx, address, key, strings.NewReader("soy el segundo\n"), io.Discard)
	waitFor(t, output, "a conversation is already open")
}

// blockingReader keeps a client alive without sending anything.
type blockingReader struct{}

func (blockingReader) Read([]byte) (int, error) {
	select {}
}
