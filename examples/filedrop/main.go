// Command filedrop moves a file between two machines over a punched channel.
//
// It exists to prove something about the layering: there is no chat here, no
// nicknames, no terminal UI, and nothing in pkg/punch had to change to allow
// it. The transport opens an encrypted path through two routers and moves
// bytes; what those bytes mean is entirely this program's business.
//
//	filedrop send <file>          prints an invite and waits
//	filedrop receive <invite>     joins and writes what arrives
//
// Deliberately minimal: one file, no resume, no integrity check beyond what
// AES-GCM already gives each datagram. It is an example, not a tool.
package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/MalPr0/vapora/pkg/punch"
	"github.com/MalPr0/vapora/pkg/stun"
)

// Our own tags, in our own numbering, carried inside the payload the transport
// moves. The transport's frame kinds are not visible here and cannot collide
// with these.
const (
	tagName byte = 1
	tagPart byte = 2
	tagDone byte = 3
)

// chunk is what fits in a datagram with room to spare for the seal and for a
// router that is stricter than most about size.
const chunk = 900

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: filedrop send <file> | filedrop receive <invite>")
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var err error
	switch os.Args[1] {
	case "send":
		err = send(ctx, os.Args[2])
	case "receive":
		err = receive(ctx, os.Args[2])
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func send(ctx context.Context, path string) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	secret, err := punch.NewSecret()
	if err != nil {
		return err
	}

	session, endpoint, err := open(ctx, secret, punch.RoleInviter, nil)
	if err != nil {
		return err
	}

	fmt.Printf("\nrun this on the other machine:\n\n    filedrop receive %s/%s\n\n", endpoint, secret)
	fmt.Println("waiting...")

	if err := session.Open(ctx, 3*time.Minute); err != nil {
		return err
	}
	fmt.Println("connected, sending", len(body), "bytes")

	session.Send(frame(tagName, []byte(filepath.Base(path))))
	for offset := 0; offset < len(body); offset += chunk {
		end := min(offset+chunk, len(body))

		part := make([]byte, 4+end-offset)
		binary.BigEndian.PutUint32(part, uint32(offset))
		copy(part[4:], body[offset:end])
		session.Send(frame(tagPart, part))

		// A pace, not a protocol: this example has no acknowledgements, so it
		// leaves room rather than filling a buffer somewhere and losing the
		// difference. A real one would wait for the other side.
		time.Sleep(2 * time.Millisecond)
	}
	session.Send(frame(tagDone, nil))

	time.Sleep(time.Second)
	session.Goodbye()
	fmt.Println("sent")
	return nil
}

func receive(ctx context.Context, invite string) error {
	parsed, err := punch.ParseInvite(invite)
	if err != nil {
		return err
	}

	session, _, err := open(ctx, parsed.Secret, punch.RoleJoiner, parsed.Endpoint)
	if err != nil {
		return err
	}

	var (
		name  = "received.bin"
		parts = map[uint32][]byte{}
		done  = make(chan struct{})
	)

	session.Observe(punch.ObserverFunc(func(payload []byte) {
		if len(payload) == 0 {
			return
		}
		switch payload[0] {
		case tagName:
			// Whatever the other side calls it, this decides where it lands.
			// A name from the network is not a path.
			name = "received-" + filepath.Base(string(payload[1:]))
		case tagPart:
			if len(payload) < 5 {
				return
			}
			offset := binary.BigEndian.Uint32(payload[1:5])
			parts[offset] = append([]byte(nil), payload[5:]...)
		case tagDone:
			select {
			case <-done:
			default:
				close(done)
			}
		}
	}))

	if err := session.Open(ctx, 3*time.Minute); err != nil {
		return err
	}
	fmt.Println("connected, receiving...")

	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}

	body, missing := assemble(parts)
	if missing > 0 {
		fmt.Printf("warning: %d gap(s) — this example has no retransmission\n", missing)
	}
	if err := os.WriteFile(name, body, 0o600); err != nil {
		return err
	}
	fmt.Printf("wrote %s, %d bytes\n", name, len(body))
	return nil
}

// assemble puts the parts back in order and reports how many holes there are.
// UDP does not promise delivery and this example does not add the promise, so
// saying so out loud beats writing a file that is quietly wrong.
func assemble(parts map[uint32][]byte) ([]byte, int) {
	var highest uint32
	for offset, part := range parts {
		if end := offset + uint32(len(part)); end > highest {
			highest = end
		}
	}

	body := make([]byte, highest)
	filled := make([]bool, highest)
	for offset, part := range parts {
		copy(body[offset:], part)
		for i := range part {
			filled[int(offset)+i] = true
		}
	}

	gaps, inGap := 0, false
	for _, ok := range filled {
		if !ok && !inGap {
			gaps++
		}
		inGap = !ok
	}
	return body, gaps
}

func frame(tag byte, payload []byte) []byte {
	return append([]byte{tag}, payload...)
}

// open is the whole of the transport setup: a socket, a mux reading it, a codec
// from the shared secret, and a session. Under a hundred lines of this file are
// about the network at all.
func open(ctx context.Context, secret punch.Secret, role punch.Role, peer *net.UDPAddr) (*punch.Session, *net.UDPAddr, error) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{})
	if err != nil {
		return nil, nil, err
	}

	codec, err := punch.NewSecretCodec(secret, role)
	if err != nil {
		return nil, nil, err
	}

	mux := punch.NewMux(conn)
	watcher := stun.NewWatcher(stun.DefaultServers, stun.DefaultKeepalive)
	mux.Fallback(punch.SinkFunc(watcher.Handle))

	session := punch.NewSession(mux, codec, nil)
	mux.Fallback(session)
	if peer != nil {
		session.SetPeer(peer)
	}

	go mux.Run(ctx)
	go watcher.Run(ctx, conn)
	go session.Run(ctx)

	if peer != nil {
		return session, nil, nil
	}

	// The waiting side needs to know its own address before it can invite.
	endpoint, err := watcher.Wait(ctx, 10*time.Second)
	if err != nil {
		return nil, nil, fmt.Errorf("no STUN server answered: %w", err)
	}
	return session, endpoint, nil
}
