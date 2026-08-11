package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MalPr0/vapora/internal/tui"
	"github.com/MalPr0/vapora/pkg/chat"
	"github.com/MalPr0/vapora/pkg/punch"
	"github.com/MalPr0/vapora/pkg/stun"
)

const inviteCommand = "vapora punch"

// pasteHint is how long the one way invite is given before suggesting the
// paste back, which is what a restricted NAT ends up needing.
const pasteHint = 12 * time.Second

// endpointTimeout is how long to keep asking STUN before giving up on knowing
// this side's public address. It is generous because the watcher rotates
// servers several times a second until one answers, so reaching it means the
// network is blocking STUN rather than one server being slow.
const endpointTimeout = 15 * time.Second

// channel is everything a session needs, assembled before either front end
// takes over.
type channel struct {
	conn      *net.UDPConn
	mux       *punch.Mux
	session   *punch.Session
	talk      *chat.Conversation
	watcher   *stun.Watcher
	secret    punch.Secret
	role      punch.Role
	nicknames chat.Pair
	peer      *net.UDPAddr
	timeout   time.Duration
	keepalive time.Duration

	// established marks that the path is up, so an address arriving later has
	// nothing left to offer a joiner: its only use was the fallback line.
	established atomic.Bool

	mu     sync.Mutex
	shared string
	failed string
}

func runPunch(args []string) error {
	flags := flag.NewFlagSet("punch", flag.ContinueOnError)
	localPort := flags.Int("port", 0, "local UDP port, 0 lets the OS choose")
	timeout := flags.Duration("timeout", 3*time.Minute, "how long to keep punching before giving up")
	keepalive := flags.Duration("keepalive", stun.DefaultKeepalive, "how often to refresh the NAT binding while waiting")
	plain := flags.Bool("plain", false, "skip the full screen UI and use plain lines")
	if err := flags.Parse(args); err != nil {
		return err
	}

	ctx, cancel := signalContext()
	defer cancel()

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{Port: *localPort})
	if err != nil {
		return fmt.Errorf("cannot open local UDP socket: %w", err)
	}
	defer conn.Close()

	secret, peer, role, err := resolveRole(flags.Args())
	if err != nil {
		return err
	}

	codec, err := punch.NewSecretCodec(secret, role)
	if err != nil {
		return err
	}

	mux := punch.NewMux(conn)
	open := &channel{
		conn:      conn,
		mux:       mux,
		session:   punch.NewSession(mux, codec, os.Stdout),
		watcher:   stun.NewWatcher(stun.DefaultServers, *keepalive),
		secret:    secret,
		role:      role,
		nicknames: chat.NamePair(secret),
		peer:      peer,
		timeout:   *timeout,
		keepalive: *keepalive,
	}
	// The conversation is the layer that knows what a line is; the session
	// underneath only knows it is moving bytes.
	open.talk = chat.Over(open.session)

	if role == punch.RoleJoiner {
		open.session.SetPeer(peer)
	}

	// The mux is the only reader of this socket. The watcher goes first because
	// it is the more selective claimant: a STUN answer is recognisable by its
	// header, while the session has to try to decrypt whatever is left.
	mux.Fallback(punch.SinkFunc(open.watcher.Handle))
	mux.Fallback(open.session)
	go mux.Run(ctx)

	if !*plain && tui.IsTerminal(os.Stdin) {
		if err := runPunchUI(ctx, open); err == nil || !errors.Is(err, errNoTerminal) {
			return err
		}
		// The terminal refused raw mode, so fall through to plain lines
		// rather than leaving the user with nothing.
	}
	return runPunchPlain(ctx, open)
}

// resolveRole decides the role from the command line: an invite as argument
// means joining, its absence means minting one and waiting.
func resolveRole(args []string) (punch.Secret, *net.UDPAddr, punch.Role, error) {
	if len(args) > 0 {
		invite, err := punch.ParseInvite(args[0])
		if err != nil {
			return nil, nil, "", err
		}
		if len(invite.Secret) == 0 {
			return nil, nil, "", errors.New("that invite carries no secret, so there is no session key and nothing to authenticate with")
		}
		return invite.Secret, invite.Endpoint, punch.RoleJoiner, nil
	}

	secret, err := punch.NewSecret()
	if err != nil {
		return nil, nil, "", err
	}
	return secret, nil, punch.RoleInviter, nil
}

// waitForPath starts the handshake and reports the public endpoint as soon as
// the watcher learns it, so the invite can be shown while the punch runs.
func (c *channel) waitForPath(ctx context.Context, invite func(*net.UDPAddr)) error {
	go c.watcher.Run(ctx, c.conn)

	// The address is handed over whenever it turns up rather than waited on
	// here. The session owns the only reader, and between Open returning and
	// Run starting nothing dispatches to the watcher: waiting here would starve
	// the very answer being waited for, which is what a peer joining an already
	// waiting invite does every time.
	go func() {
		endpoint, err := c.watcher.Wait(ctx, endpointTimeout)
		if err != nil {
			endpoint = nil
		}
		invite(endpoint)
	}()

	if err := c.session.Open(ctx, c.timeout); err != nil {
		if errors.Is(err, punch.ErrPunchTimeout) {
			return fmt.Errorf("%w: both sides have to run at once, and the secret has to match", err)
		}
		return err
	}
	c.established.Store(true)
	return nil
}

func (c *channel) inviteFor(endpoint *net.UDPAddr) string {
	invite := punch.Invite{Endpoint: endpoint, Secret: c.secret}.Command(inviteCommand)

	c.mu.Lock()
	c.shared = invite
	c.mu.Unlock()
	return invite
}

// note records why a session ended, so the summary can say it after the screen
// is handed back.
func (c *channel) note(reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failed = reason
}

// report writes what happened to the real screen. Anything the full screen UI
// said is gone the moment it restores, and a session that did not work is
// exactly the one somebody needs a record of.
func (c *channel) report() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.established.Load() {
		fmt.Printf("\nthe path with %s is closed.\n", c.session.Peer())
		return
	}

	fmt.Println("\nno path was ever opened.")
	if c.failed != "" {
		fmt.Printf("  %s\n", c.failed)
	}
	if c.shared != "" {
		fmt.Printf("  the invite this side was offering:\n    %s\n", c.shared)
	}
	if c.role == punch.RoleInviter {
		fmt.Println("  this side was waiting. Somebody has to run that line, or paste theirs here.")
	}
	if probes := c.session.Probes(); probes.Count > 0 {
		fmt.Printf("  %d packets arrived from %d address(es) and were not from a peer.\n",
			probes.Count, probes.Sources)
	}
}
