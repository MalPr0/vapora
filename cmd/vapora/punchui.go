package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/MalPr0/vapora/internal/tui"
	"github.com/MalPr0/vapora/pkg/punch"
)

// errNoTerminal says the tty refused raw mode, which is the one failure the
// caller answers by falling back to plain lines instead of giving up.
var errNoTerminal = errors.New("terminal unavailable")

func runPunchUI(ctx context.Context, open *channel) error {
	terminal, err := tui.OpenTerminal(os.Stdin, os.Stdout)
	if err != nil {
		return fmt.Errorf("%w: %w", errNoTerminal, err)
	}
	defer terminal.Restore()

	me := open.nicknames.For(open.role)
	peer := open.nicknames.Other(open.role)

	chat := tui.NewChat(terminal, me, peer)
	chat.OnSend(open.session.SendMessage)
	chat.OnTyping(open.session.SetTyping)
	open.session.Observe(chat)

	uiCtx, stopUI := context.WithCancel(ctx)
	defer stopUI()

	// The UI owns the terminal from here on, so everything the connection has
	// to say goes through it and nothing writes to stdout directly.
	done := make(chan error, 1)
	go func() { done <- chat.Run(uiCtx) }()

	go func() {
		if err := connect(uiCtx, open, chat); err != nil {
			chat.Closed(err.Error())
			return
		}
		chat.Connected(me, peer)
		go pushHealth(uiCtx, open, chat)
		if err := open.session.Run(uiCtx); err != nil {
			chat.Closed(err.Error())
		}
	}()

	return <-done
}

// pushHealth keeps the link indicator current and posts a line when the verdict
// changes, so a path that goes quiet says so instead of just looking idle.
func pushHealth(ctx context.Context, open *channel, chat *tui.Chat) {
	go watchPath(ctx, open, func(recovery Recovery) {
		chat.System(recoveryMessage(recovery))
	})

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			health := open.session.Health()
			chat.SetLink(linkState(health.Link), health.RTT, health.Silence)
		}
	}
}

func linkState(link punch.Link) tui.LinkState {
	switch link {
	case punch.LinkStale:
		return tui.LinkStale
	case punch.LinkLost:
		return tui.LinkLost
	default:
		return tui.LinkAlive
	}
}

// connect drives the loading screen through the handshake and hands the invite
// to the UI as soon as there is one to show.
func connect(ctx context.Context, open *channel, chat *tui.Chat) error {
	chat.SetStatus("looking up your public endpoint", 0.05)

	endpoint, err := open.endpoint(ctx)
	if err != nil {
		return err
	}

	if open.role == punch.RoleJoiner {
		chat.SetStatus("punching towards your friend", 0.2)
		chat.SetInvite("if it stalls, send back: " + open.inviteFor(endpoint))
	} else {
		chat.SetStatus("waiting for your friend to join", 0.2)
		chat.SetInvite(open.inviteFor(endpoint))
	}

	go watchEndpoint(ctx, open, func(_, current *net.UDPAddr) {
		chat.SetInvite(open.inviteFor(current))
		chat.System(movedMessage(open, current))
	})
	go trackProgress(ctx, open, chat)

	if err := open.session.Open(ctx, open.timeout); err != nil {
		if errors.Is(err, punch.ErrPunchTimeout) {
			return fmt.Errorf("%w: both sides have to run at once, and the secret has to match", err)
		}
		return err
	}
	chat.SetStatus("connected", 1)
	return nil
}

// trackProgress advances the loading meter with elapsed time. There is nothing
// better to measure: a punch either lands or it does not, so the bar shows how
// much of the budget is gone rather than pretending to know more.
func trackProgress(ctx context.Context, open *channel, chat *tui.Chat) {
	started := time.Now()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	announced := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !announced && open.session.Peer() != nil && open.role == punch.RoleInviter {
				chat.SetStatus("your friend arrived, opening the path", 0.85)
				announced = true
			}
			elapsed := time.Since(started).Seconds() / open.timeout.Seconds()
			chat.SetProgress(0.2 + 0.6*elapsed)
		}
	}
}
