package main

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/MalPr0/vapora/pkg/punch"
)

// runPunchPlain is the line based front end. It is what a pipe, a CI job or a
// terminal that refuses raw mode gets, and it stays the reference behaviour.
func runPunchPlain(ctx context.Context, open *channel) error {
	ctx, quit := context.WithCancel(ctx)
	defer quit()

	go readStdin(ctx, open, quit)

	hint := time.AfterFunc(pasteHint, func() {
		if open.session.Peer() == nil {
			fmt.Println("\nstill nothing. Your NAT may be dropping unannounced packets:")
			fmt.Println("ask your friend for the invite their side printed and paste it here.")
		}
	})
	defer hint.Stop()

	open.watcher.OnChange(func(_, current *net.UDPAddr) {
		fmt.Printf("\n-- %s\n", movedMessage(open, current))
	})

	if open.role == punch.RoleJoiner {
		fmt.Printf("\npunching towards %s\n", open.peer)
	}

	if err := open.waitForPath(ctx, func(endpoint *net.UDPAddr) {
		printInvite(open, endpoint)
	}); err != nil {
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
		if open.established.Load() {
			return // the fallback line has nothing to offer an open path
		}
		if endpoint == nil {
			fmt.Println("no STUN server answered, so there is no invite to send back if this stalls")
			return
		}
		fmt.Printf("if it does not connect, send this back so they can paste it:\n\n    %s\n\n", open.inviteFor(endpoint))
		return
	}

	if endpoint == nil {
		fmt.Println("no STUN server answered, so there is no address to put on an invite.")
		fmt.Println("something on this network is blocking STUN; run `vapora nat` to see how far it gets.")
		return
	}

	fmt.Printf("send this to your friend, it is a runnable command:\n\n    %s\n\n", open.inviteFor(endpoint))
	fmt.Println("the secret in that line is the session key: only whoever holds it can join,")
	fmt.Println("and every packet is encrypted with it. Send it over a channel you trust.")
	fmt.Println("\nwaiting for them...")
}

// readStdin doubles as the invite prompt before the path is open and as the
// chat input once it is, so a single reader owns the terminal.
func readStdin(ctx context.Context, open *channel, quit context.CancelFunc) {
	lines := readLines(ctx)

	for {
		var line string
		select {
		case <-ctx.Done():
			return
		case typed, more := <-lines:
			if !more {
				<-ctx.Done()
				return
			}
			line = typed
		}

		if isExit(line) {
			leave(open.session)
			quit()
			return
		}

		if open.session.Peer() != nil {
			open.session.SendMessage(line)
			continue
		}

		invite, err := punch.ParseInvite(line)
		if err != nil {
			fmt.Println("that is not an endpoint. Paste your friend's invite here; chat opens once the path does.")
			continue
		}
		if !open.secret.Equal(invite.Secret) {
			fmt.Println("that invite carries a different secret, so the two sides would never talk. Ignored.")
			continue
		}
		open.session.SetPeer(invite.Endpoint)
		fmt.Printf("punching towards %s...\n", invite.Endpoint)
	}
}
