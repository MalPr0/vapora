package chat

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/MalPr0/vapora/pkg/text"
)

// Server is a minimal line based broadcast chat used to prove that the port
// mapping actually carries traffic from outside the local network.
type Server struct {
	output io.Writer

	mu    sync.Mutex
	peers map[net.Conn]string
}

func NewServer(output io.Writer) *Server {
	return &Server{output: output, peers: map[net.Conn]string{}}
}

func (s *Server) Listen(ctx context.Context, address string) (net.Listener, error) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("chat: cannot listen on %s: %w", address, err)
	}

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()
	return listener, nil
}

func (s *Server) Serve(ctx context.Context, listener net.Listener) error {
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("chat: accept failed: %w", err)
		}
		go s.handle(ctx, conn)
	}
}

func (s *Server) handle(ctx context.Context, conn net.Conn) {
	name := conn.RemoteAddr().String()

	s.mu.Lock()
	s.peers[conn] = name
	s.mu.Unlock()

	s.report("* %s connected", name)
	fmt.Fprintf(conn, "welcome, you reached the host through UPnP as %s\n", name)

	defer func() {
		s.mu.Lock()
		delete(s.peers, conn)
		s.mu.Unlock()
		_ = conn.Close()
		s.report("* %s disconnected", name)
	}()

	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		// Nothing authenticates a peer here, so anyone who reaches the mapped
		// port could otherwise drive the host terminal with escape sequences.
		line := text.Safe(scanner.Text())
		s.report("<%s> %s", name, line)
		s.Broadcast(fmt.Sprintf("<%s> %s", name, line), conn)
	}
}

// Broadcast writes a line to every peer except the one that originated it.
func (s *Server) Broadcast(line string, origin net.Conn) {
	s.mu.Lock()
	targets := make([]net.Conn, 0, len(s.peers))
	for conn := range s.peers {
		if conn != origin {
			targets = append(targets, conn)
		}
	}
	s.mu.Unlock()

	for _, conn := range targets {
		fmt.Fprintf(conn, "%s\n", line)
	}
}

func (s *Server) PeerCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.peers)
}

func (s *Server) report(format string, args ...any) {
	fmt.Fprintf(s.output, format+"\n", args...)
}
