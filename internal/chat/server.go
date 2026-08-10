package chat

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/MalPr0/vapora/pkg/punch"
	"github.com/MalPr0/vapora/pkg/text"
)

// Server hosts one authenticated conversation at a time. One at a time is not a
// limitation to work around: every peer would otherwise share the same
// direction key, and two of them could collide on a nonce under it.
type Server struct {
	output io.Writer
	secret punch.Secret

	mu   sync.Mutex
	busy bool
	peer *stream
}

func NewServer(output io.Writer, secret punch.Secret) *Server {
	return &Server{output: output, secret: secret}
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

// Serve accepts one peer at a time, and goes back to waiting when it leaves.
func (s *Server) Serve(ctx context.Context, listener net.Listener, input io.Reader) error {
	go s.pump(ctx, input)

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("chat: accept failed: %w", err)
		}

		// Accepting has to keep running while a conversation is open, or a
		// second peer sits in the listen backlog with no answer at all
		// instead of being told the seat is taken.
		if !s.claim() {
			fmt.Fprintf(s.output, "* refused %s, a conversation is already open\n", conn.RemoteAddr())
			_ = conn.Close()
			continue
		}
		go func() {
			defer s.release()
			s.handle(ctx, conn)
		}()
	}
}

func (s *Server) handle(ctx context.Context, conn net.Conn) {
	codec, err := punch.NewSecretCodec(s.secret, punch.RoleInviter)
	if err != nil {
		fmt.Fprintf(s.output, "* cannot build the session key: %v\n", err)
		_ = conn.Close()
		return
	}

	peer := newStream(conn, codec)
	s.setPeer(peer)
	defer s.setPeer(nil)

	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
		case <-stop:
		}
		_ = peer.Close()
	}()

	name := conn.RemoteAddr().String()
	fmt.Fprintf(s.output, "* %s connected\n", name)
	defer fmt.Fprintf(s.output, "* %s disconnected\n", name)

	for {
		line, err := peer.ReadLine()
		if err != nil {
			// A frame that does not authenticate is the interesting failure: it
			// means somebody found the port without the key.
			if ctx.Err() == nil && err != io.EOF {
				fmt.Fprintf(s.output, "* %s dropped: %v\n", name, err)
			}
			return
		}
		fmt.Fprintf(s.output, "<%s> %s\n", name, text.Safe(line))
	}
}

// pump sends what the host types to whoever is connected.
func (s *Server) pump(ctx context.Context, input io.Reader) {
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		if ctx.Err() != nil {
			return
		}

		s.mu.Lock()
		peer := s.peer
		s.mu.Unlock()

		if peer == nil {
			fmt.Fprintln(s.output, "* nobody is connected yet")
			continue
		}
		if err := peer.WriteLine(scanner.Text()); err != nil {
			fmt.Fprintf(s.output, "* cannot send: %v\n", err)
		}
	}
}

// claim takes the single conversation slot, or reports that it is taken.
func (s *Server) claim() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.busy {
		return false
	}
	s.busy = true
	return true
}

func (s *Server) release() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.busy = false
	s.peer = nil
}

func (s *Server) setPeer(peer *stream) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.peer = peer
}
