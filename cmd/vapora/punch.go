package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/MalPr0/vapora/pkg/punch"
	"github.com/MalPr0/vapora/pkg/stun"
)

const inviteCommand = "vapora punch"

// pasteHint is how long the one way invite is given before suggesting the
// paste back, which is what a restricted NAT ends up needing.
const pasteHint = 12 * time.Second

func runPunch(args []string) error {
	flags := flag.NewFlagSet("punch", flag.ContinueOnError)
	localPort := flags.Int("port", 0, "local UDP port, 0 lets the OS choose")
	timeout := flags.Duration("timeout", 3*time.Minute, "how long to keep punching before giving up")
	insecure := flags.Bool("insecure", false, "drop the invite secret and run the session unauthenticated")
	keepalive := flags.Duration("keepalive", stun.DefaultKeepalive, "how often to refresh the NAT binding while waiting")
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

	endpoint, _, err := stun.FirstEndpoint(ctx, conn, stun.DefaultServers, stunTimeout)
	if err != nil {
		return err
	}

	invite, role, err := resolveInvite(flags.Args(), endpoint, *insecure)
	if err != nil {
		return err
	}

	codec, err := buildCodec(invite.Secret, role)
	if err != nil {
		return err
	}

	session := punch.NewSession(conn, codec, os.Stdout)
	if role == punch.RoleJoiner {
		session.SetPeer(invite.Endpoint)
	}

	printInvite(session, punch.Invite{Endpoint: endpoint, Secret: invite.Secret}, role, len(invite.Secret) > 0)
	go readStdin(ctx, session, invite.Secret)
	go stun.Keepalive(ctx, conn, stun.DefaultServers, *keepalive)

	hint := time.AfterFunc(pasteHint, func() {
		if session.Peer() == nil {
			fmt.Println("\nstill nothing. Your NAT may be dropping unannounced packets:")
			fmt.Println("ask your friend for the invite their side printed and paste it here.")
		}
	})
	defer hint.Stop()

	if err := session.Open(ctx, *timeout); err != nil {
		if errors.Is(err, punch.ErrPunchTimeout) {
			return fmt.Errorf("%w: both sides have to run at once, and the secret has to match", err)
		}
		return err
	}
	hint.Stop()

	fmt.Printf("\ndirect UDP path open with %s, type to chat, ctrl+c to quit\n", session.Peer())
	return session.Run(ctx)
}

// resolveInvite decides the role from the command line: an invite as argument
// means joining, its absence means minting one and waiting.
func resolveInvite(args []string, endpoint *net.UDPAddr, insecure bool) (punch.Invite, punch.Role, error) {
	if len(args) > 0 {
		invite, err := punch.ParseInvite(args[0])
		if err != nil {
			return punch.Invite{}, "", err
		}
		if len(invite.Secret) == 0 && !insecure {
			return punch.Invite{}, "", errors.New("that invite carries no secret, re-run with -insecure if that is on purpose")
		}
		return invite, punch.RoleJoiner, nil
	}

	invite := punch.Invite{Endpoint: endpoint}
	if !insecure {
		secret, err := punch.NewSecret()
		if err != nil {
			return punch.Invite{}, "", err
		}
		invite.Secret = secret
	}
	return invite, punch.RoleInviter, nil
}

func buildCodec(secret punch.Secret, role punch.Role) (punch.Codec, error) {
	if len(secret) == 0 {
		return punch.PlainCodec{}, nil
	}
	return punch.NewSecretCodec(secret, role)
}

func printInvite(session *punch.Session, invite punch.Invite, role punch.Role, authenticated bool) {
	fmt.Println()
	if role == punch.RoleJoiner {
		fmt.Printf("punching towards %s\n", session.Peer())
		fmt.Printf("if it does not connect, send this back so they can paste it:\n\n    %s\n\n", invite.Command(inviteCommand))
		return
	}

	fmt.Printf("send this to your friend, it is a runnable command:\n\n    %s\n\n", invite.Command(inviteCommand))
	if authenticated {
		fmt.Println("the secret in that line is the session key: only whoever holds it can join,")
		fmt.Println("and every packet is encrypted with it. Send it over a channel you trust.")
	} else {
		fmt.Println("warning: running unauthenticated, any host that finds this port can join")
	}
	fmt.Println("\nwaiting for them...")
}

// readStdin doubles as the invite prompt before the path is open and as the
// chat input once it is, so a single reader owns the terminal.
func readStdin(ctx context.Context, session *punch.Session, secret punch.Secret) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		if ctx.Err() != nil {
			return
		}
		line := scanner.Text()

		if session.Peer() != nil {
			session.SendMessage(line)
			continue
		}

		invite, err := punch.ParseInvite(line)
		if err != nil {
			fmt.Println("that is not an endpoint. Paste your friend's invite here; chat opens once the path does.")
			continue
		}
		if len(secret) > 0 && !secret.Equal(invite.Secret) {
			fmt.Println("that invite carries a different secret, so the two sides would never talk. Ignored.")
			continue
		}
		session.SetPeer(invite.Endpoint)
		fmt.Printf("punching towards %s...\n", invite.Endpoint)
	}
}
