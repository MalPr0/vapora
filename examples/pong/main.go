// Command pong is two-player Pong over a direct, encrypted channel between two
// machines on the internet. No server, no account, no port forwarding.
//
//	pong host              prints an invite and waits
//	pong join <invite>     the other machine runs this
//
// It exists to show that pkg/punch carries whatever you give it. A chat sends
// events and cares about every one; a game sends state thirty times a second
// and only ever cares about the last. Same channel, opposite use, and nothing
// in the transport had to know the difference.
//
// Not shipped in releases: it is a tutorial. See README.md beside this file.
package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"time"

	"github.com/MalPr0/vapora/pkg/names"
	"github.com/MalPr0/vapora/pkg/punch"
	"github.com/MalPr0/vapora/pkg/stun"
)

// tickRate is how often the host sends the world. Thirty a second is smooth to
// the eye and is eleven bytes a packet, which is nothing.
const tickRate = 33 * time.Millisecond

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: pong host | pong join <invite>")
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var err error
	switch os.Args[1] {
	case "host":
		err = host(ctx)
	case "join":
		if len(os.Args) < 3 {
			err = fmt.Errorf("join needs the invite the host printed")
			break
		}
		err = join(ctx, os.Args[2])
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "\nerror:", err)
		os.Exit(1)
	}
}

// host opens the channel, waits, and then owns the ball.
func host(ctx context.Context) error {
	secret, err := punch.NewSecret()
	if err != nil {
		return err
	}

	fmt.Print(splash())

	table, endpoint, err := connect(ctx, secret, punch.RoleInviter, nil)
	if err != nil {
		return err
	}

	fmt.Printf("\n      run this on the other machine:\n\n        pong join %s/%s\n\n", endpoint, secret)
	fmt.Println("      waiting for a challenger...")

	if err := table.session.Open(ctx, 3*time.Minute); err != nil {
		return err
	}
	return table.play(ctx, true)
}

func join(ctx context.Context, invite string) error {
	parsed, err := punch.ParseInvite(invite)
	if err != nil {
		return err
	}

	fmt.Print(splash())

	table, _, err := connect(ctx, parsed.Secret, punch.RoleJoiner, parsed.Endpoint)
	if err != nil {
		return err
	}

	fmt.Println("\n      connecting...")
	if err := table.session.Open(ctx, 3*time.Minute); err != nil {
		return err
	}
	return table.play(ctx, false)
}

// table is the game and the channel it runs over.
type table struct {
	session *punch.Session
	me      string
	them    string

	// incoming carries what the peer sent, already decoded. The transport
	// delivers on its own goroutine, so the game loop reads from here instead
	// of being called from somewhere it does not control.
	incoming chan []byte
}

// connect is the whole of the network setup: a socket, a mux reading it, a
// codec from the shared secret, and a session on top.
func connect(ctx context.Context, secret punch.Secret, role punch.Role, peer *net.UDPAddr) (*table, *net.UDPAddr, error) {
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

	built := &table{session: session, incoming: make(chan []byte, 64)}
	session.Observe(punch.ObserverFunc(func(payload []byte) {
		// Never block the transport. A game that falls behind wants the newest
		// state, not a backlog of stale ones, so a full buffer drops.
		select {
		case built.incoming <- payload:
		default:
		}
	}))

	// Names both sides compute identically, from the secret they already share.
	pair := names.Short([]names.Key{keyFrom(secret, 0), keyFrom(secret, 1)})
	first, second := keyFrom(secret, 0), keyFrom(secret, 1)
	if role == punch.RoleInviter {
		built.me, built.them = pair[first], pair[second]
	} else {
		built.me, built.them = pair[second], pair[first]
	}

	go mux.Run(ctx)
	go watcher.Run(ctx, conn)
	go session.Run(ctx)

	if peer != nil {
		return built, nil, nil
	}

	endpoint, err := watcher.Wait(ctx, 15*time.Second)
	if err != nil {
		return nil, nil, fmt.Errorf("no STUN server answered: %w", err)
	}
	return built, endpoint, nil
}

// keyFrom turns the shared secret into two stable keys, only so pkg/names has
// something to name. A room derives names from real public keys; a two-way
// session has only a secret and a role, and both sides get the same answer.
func keyFrom(secret punch.Secret, slot byte) names.Key {
	var key names.Key
	copy(key[:], secret)
	key[len(key)-1] = slot
	return key
}
