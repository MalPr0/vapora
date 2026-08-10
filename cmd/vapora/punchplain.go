package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/MalPr0/vapora/pkg/punch"
)

// runPunchPlain is the line based front end. It is what a pipe, a CI job or a
// terminal that refuses raw mode gets, and it stays the reference behaviour.
func runPunchPlain(ctx context.Context, open *channel) error {
	endpoint, err := open.endpoint(ctx)
	if err != nil {
		return err
	}

	printInvite(open, endpoint)
	go readStdin(ctx, open)
	go watchEndpoint(ctx, open, func(_, current *net.UDPAddr) {
		fmt.Printf("\n-- %s\n", movedMessage(open, current))
	})

	hint := time.AfterFunc(pasteHint, func() {
		if open.session.Peer() == nil {
			fmt.Println("\nstill nothing. Your NAT may be dropping unannounced packets:")
			fmt.Println("ask your friend for the invite their side printed and paste it here.")
		}
	})
	defer hint.Stop()

	if err := open.session.Open(ctx, open.timeout); err != nil {
		if errors.Is(err, punch.ErrPunchTimeout) {
			return fmt.Errorf("%w: both sides have to run at once, and the secret has to match", err)
		}
		return err
	}
	hint.Stop()

	fmt.Printf("\ndirect UDP path open with %s. You are %s, they are %s. Type to chat, ctrl+c to quit\n",
		open.session.Peer(), open.nicknames.For(open.role), open.nicknames.Other(open.role))

	go watchPath(ctx, open, func(recovery Recovery) {
		fmt.Printf("-- %s\n", recoveryMessage(recovery))
	})
	return open.session.Run(ctx)
}

func printInvite(open *channel, endpoint *net.UDPAddr) {
	fmt.Println()
	if open.role == punch.RoleJoiner {
		fmt.Printf("punching towards %s\n", open.session.Peer())
		fmt.Printf("if it does not connect, send this back so they can paste it:\n\n    %s\n\n", open.inviteFor(endpoint))
		return
	}

	fmt.Printf("send this to your friend, it is a runnable command:\n\n    %s\n\n", open.inviteFor(endpoint))
	if len(open.secret) > 0 {
		fmt.Println("the secret in that line is the session key: only whoever holds it can join,")
		fmt.Println("and every packet is encrypted with it. Send it over a channel you trust.")
	} else {
		fmt.Println("warning: running unauthenticated, any host that finds this port can join")
	}
	fmt.Println("\nwaiting for them...")
}

// readStdin doubles as the invite prompt before the path is open and as the
// chat input once it is, so a single reader owns the terminal.
func readStdin(ctx context.Context, open *channel) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		if ctx.Err() != nil {
			return
		}
		line := scanner.Text()

		if open.session.Peer() != nil {
			open.session.SendMessage(line)
			continue
		}

		invite, err := punch.ParseInvite(line)
		if err != nil {
			fmt.Println("that is not an endpoint. Paste your friend's invite here; chat opens once the path does.")
			continue
		}
		if len(open.secret) > 0 && !open.secret.Equal(invite.Secret) {
			fmt.Println("that invite carries a different secret, so the two sides would never talk. Ignored.")
			continue
		}
		open.session.SetPeer(invite.Endpoint)
		fmt.Printf("punching towards %s...\n", invite.Endpoint)
	}
}
