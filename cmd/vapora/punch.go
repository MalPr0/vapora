package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"sync/atomic"
	"time"

	"github.com/MalPr0/vapora/internal/tui"
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
	session   *punch.Session
	watcher   *stun.Watcher
	secret    punch.Secret
	role      punch.Role
	nicknames punch.Nicknames
	peer      *net.UDPAddr
	timeout   time.Duration
	keepalive time.Duration

	// established marks that the path is up, so an address arriving later has
	// nothing left to offer a joiner: its only use was the fallback line.
	established atomic.Bool
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

	open := &channel{
		conn:      conn,
		session:   punch.NewSession(conn, codec, os.Stdout),
		watcher:   stun.NewWatcher(stun.DefaultServers, *keepalive),
		secret:    secret,
		role:      role,
		nicknames: secret.Nicknames(),
		peer:      peer,
		timeout:   *timeout,
		keepalive: *keepalive,
	}
	if role == punch.RoleJoiner {
		open.session.SetPeer(peer)
	}

	// The session owns the only reader of this socket from here on, and the
	// endpoint watcher gets its answers through it. Querying STUN directly
	// would mean a second reader, which on a UDP socket is a lottery over
	// which one receives each datagram.
	open.session.Sniff(open.watcher.Handle)

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
	return punch.Invite{Endpoint: endpoint, Secret: c.secret}.Command(inviteCommand)
}
