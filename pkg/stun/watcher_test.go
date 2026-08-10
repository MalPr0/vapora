package stun

import (
	"context"
	"net"
	"testing"
	"time"
)

func watcherSockets(t *testing.T) (*net.UDPConn, *movingServer) {
	t.Helper()

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("cannot listen: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn, newMovingServer(t, 41001)
}

// The socket has one owner, so the watcher is handed its answers rather than
// reading alongside. Handle is that handoff.
func TestWatcherLearnsTheEndpoint(t *testing.T) {
	conn, server := watcherSockets(t)

	watcher := NewWatcher([]string{server.address()}, 30*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go watcher.Run(ctx, conn)

	feed(t, conn, watcher, "203.0.113.5:41001")
	if endpoint := watcher.Endpoint(); endpoint == nil || endpoint.String() != "203.0.113.5:41001" {
		t.Fatalf("got %v", endpoint)
	}
}

// An address that moves is what silently kills an invite already shared, so it
// has to be reported exactly once, and not on the first observation.
func TestWatcherReportsOnlyRealMoves(t *testing.T) {
	conn, server := watcherSockets(t)

	changes := make(chan string, 4)
	watcher := NewWatcher([]string{server.address()}, 30*time.Millisecond)
	watcher.OnChange(func(_, current *net.UDPAddr) { changes <- current.String() })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go watcher.Run(ctx, conn)

	feed(t, conn, watcher, "203.0.113.5:41001")
	select {
	case got := <-changes:
		t.Fatalf("the first observation was reported as a move to %s", got)
	case <-time.After(200 * time.Millisecond):
	}

	feed(t, conn, watcher, "203.0.113.5:41001")
	select {
	case got := <-changes:
		t.Fatalf("an unchanged address was reported as a move to %s", got)
	case <-time.After(200 * time.Millisecond):
	}

	feed(t, conn, watcher, "203.0.113.9:52000")
	select {
	case got := <-changes:
		if got != "203.0.113.9:52000" {
			t.Fatalf("got %s", got)
		}
	case <-time.After(time.Second):
		t.Fatal("a moved address was never reported")
	}
}

// A datagram that is not an answer to this watcher must be left for whoever
// else shares the socket.
func TestWatcherLeavesForeignDatagrams(t *testing.T) {
	conn, server := watcherSockets(t)
	watcher := NewWatcher([]string{server.address()}, time.Hour)

	for _, payload := range [][]byte{
		{},
		[]byte("not stun at all"),
		buildResponse(t, [12]byte{9, 9, 9}, attribute{attrXORMappedAddress, xorAddress("203.0.113.5", 41001)}),
	} {
		if watcher.Handle(payload, nil) {
			t.Fatalf("the watcher claimed a datagram that was not its own: %q", payload)
		}
	}
	_ = conn
}

// feed sends one request and hands the watcher an answer to it, standing in for
// the session that owns the reader.
func feed(t *testing.T, conn *net.UDPConn, watcher *Watcher, mapped string) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		watcher.mu.Lock()
		var transactionID [12]byte
		found := false
		for id := range watcher.pending {
			transactionID, found = id, true
			break
		}
		watcher.mu.Unlock()

		if found {
			address, err := net.ResolveUDPAddr("udp4", mapped)
			if err != nil {
				t.Fatalf("bad address %q: %v", mapped, err)
			}
			response := buildResponse(t, transactionID,
				attribute{attrXORMappedAddress, xorAddress(address.IP.String(), uint16(address.Port))})
			if !watcher.Handle(response, nil) {
				t.Fatal("the watcher did not claim its own answer")
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the watcher never sent a request")
}

// movingServer only has to exist: the watcher writes to it and the test feeds
// the answers back by hand.
type movingServer struct {
	conn *net.UDPConn
}

func newMovingServer(t *testing.T, _ uint16) *movingServer {
	t.Helper()

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("cannot listen: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return &movingServer{conn: conn}
}

func (s *movingServer) address() string { return s.conn.LocalAddr().String() }
