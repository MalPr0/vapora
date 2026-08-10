package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/MalPr0/vapora/internal/tui"
	"github.com/MalPr0/vapora/pkg/punch"
	"github.com/MalPr0/vapora/pkg/stun"
)

const inviteCommand = "vapora punch"

// pasteHint is how long the one way invite is given before suggesting the
// paste back, which is what a restricted NAT ends up needing.
const pasteHint = 12 * time.Second

// channel is everything a session needs, assembled before either front end
// takes over.
type channel struct {
	conn      *net.UDPConn
	session   *punch.Session
	secret    punch.Secret
	role      punch.Role
	nicknames punch.Nicknames
	invite    punch.Invite
	timeout   time.Duration
	keepalive time.Duration
}

func runPunch(args []string) error {
	flags := flag.NewFlagSet("punch", flag.ContinueOnError)
	localPort := flags.Int("port", 0, "local UDP port, 0 lets the OS choose")
	timeout := flags.Duration("timeout", 3*time.Minute, "how long to keep punching before giving up")
	insecure := flags.Bool("insecure", false, "drop the invite secret and run the session unauthenticated")
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

	secret, peer, role, err := resolveRole(flags.Args(), *insecure)
	if err != nil {
		return err
	}

	codec, err := buildCodec(secret, role)
	if err != nil {
		return err
	}

	open := &channel{
		conn:      conn,
		session:   punch.NewSession(conn, codec, os.Stdout),
		secret:    secret,
		role:      role,
		nicknames: secret.Nicknames(),
		invite:    punch.Invite{Endpoint: peer, Secret: secret},
		timeout:   *timeout,
		keepalive: *keepalive,
	}
	if role == punch.RoleJoiner {
		open.session.SetPeer(peer)
	}

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
func resolveRole(args []string, insecure bool) (punch.Secret, *net.UDPAddr, punch.Role, error) {
	if len(args) > 0 {
		invite, err := punch.ParseInvite(args[0])
		if err != nil {
			return nil, nil, "", err
		}
		if len(invite.Secret) == 0 && !insecure {
			return nil, nil, "", errors.New("that invite carries no secret, re-run with -insecure if that is on purpose")
		}
		return invite.Secret, invite.Endpoint, punch.RoleJoiner, nil
	}

	if insecure {
		return nil, nil, punch.RoleInviter, nil
	}
	secret, err := punch.NewSecret()
	if err != nil {
		return nil, nil, "", err
	}
	return secret, nil, punch.RoleInviter, nil
}

func buildCodec(secret punch.Secret, role punch.Role) (punch.Codec, error) {
	if len(secret) == 0 {
		return punch.PlainCodec{}, nil
	}
	return punch.NewSecretCodec(secret, role)
}

func (c *channel) endpoint(ctx context.Context) (*net.UDPAddr, error) {
	endpoint, _, err := stun.FirstEndpoint(ctx, c.conn, stun.DefaultServers, stunTimeout)
	return endpoint, err
}

func (c *channel) inviteFor(endpoint *net.UDPAddr) string {
	return punch.Invite{Endpoint: endpoint, Secret: c.secret}.Command(inviteCommand)
}
