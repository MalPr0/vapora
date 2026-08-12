package stun

import (
	"context"
	"encoding/binary"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

// countingServer only counts what arrives: the keepalive never reads answers,
// so the packet leaving is the whole contract.
type countingServer struct {
	conn     *net.UDPConn
	received atomic.Int64
}

func newCountingServer(t *testing.T) *countingServer {
	t.Helper()

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("cannot listen: %v", err)
	}

	server := &countingServer{conn: conn}
	go server.serve()
	t.Cleanup(func() { conn.Close() })
	return server
}

func (s *countingServer) address() string {
	return s.conn.LocalAddr().String()
}

func (s *countingServer) serve() {
	buffer := make([]byte, 1500)
	for {
		n, _, err := s.conn.ReadFromUDP(buffer)
		if err != nil {
			return
		}
		if n >= headerSize && binary.BigEndian.Uint32(buffer[4:8]) == magicCookie {
			s.received.Add(1)
		}
	}
}

func TestKeepaliveKeepsTheBindingWarm(t *testing.T) {
	server := newCountingServer(t)

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("cannot listen: %v", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 350*time.Millisecond)
	defer cancel()

	_ = Keepalive(ctx, conn, []string{server.address()}, 50*time.Millisecond)

	if got := server.received.Load(); got < 3 {
		t.Fatalf("only %d keepalives left the socket, the binding would have expired", got)
	}
}

// The socket owner has to keep receiving its own traffic: a keepalive that read
// would steal datagrams from it.
func TestKeepaliveLeavesTheSocketReadable(t *testing.T) {
	server := newCountingServer(t)

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("cannot listen: %v", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go Keepalive(ctx, conn, []string{server.address()}, 20*time.Millisecond)

	other, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("cannot listen: %v", err)
	}
	defer other.Close()

	local := conn.LocalAddr().(*net.UDPAddr)
	for attempt := 0; attempt < 20; attempt++ {
		if _, err := other.WriteToUDP([]byte("mine"), local); err != nil {
			t.Fatalf("cannot send: %v", err)
		}

		_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		buffer := make([]byte, 64)
		n, _, err := conn.ReadFromUDP(buffer)
		if err == nil && string(buffer[:n]) == "mine" {
			return
		}
	}
	t.Fatal("the owner never read its own datagram")
}

func TestKeepaliveNeedsAServer(t *testing.T) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("cannot listen: %v", err)
	}
	defer conn.Close()

	if err := Keepalive(context.Background(), conn, nil, time.Second); err == nil {
		t.Fatal("expected an error with no servers")
	}
}
