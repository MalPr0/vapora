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

	// The UI draws on the alternate screen, so quitting wipes every trace of
	// what happened. A session that failed has to leave something behind in
	// the scrollback, or the only way to find out why is to run it again in
	// plain mode and hope it fails the same way.
	defer func() {
		terminal.Restore()
		open.report()
	}()

	me := open.nicknames.For(open.role)
	peer := open.nicknames.Other(open.role)

	view := tui.NewChat(terminal, me)
	view.OnSend(open.talk.Say)
	view.OnTyping(open.talk.SetTyping)
	view.OnCommand(func(line string) bool {
		if !isExit(line) {
			return false
		}
		leave(open.session)
		view.Quit()
		return true
	})
	// A session only ever has one peer, so it reports without saying who; the
	// view names every line because a room has no default speaker. Putting the
	// name back here is what lets both use the same view.
	open.talk.OnLine(func(line string) { view.Message(peer, line) })
	open.talk.OnTyping(func(active bool) { view.Typing(peer, active) })

	uiCtx, stopUI := context.WithCancel(ctx)
	defer stopUI()

	// The UI owns the terminal from here on, so everything the connection has
	// to say goes through it and nothing writes to stdout directly.
	done := make(chan error, 1)
	go func() { done <- view.Run(uiCtx) }()

	go func() {
		if err := connect(uiCtx, open, view); err != nil {
			open.note(err.Error())
			view.Closed(err.Error())
			return
		}
		view.Connected(me)
		go pushHealth(uiCtx, open, view)
		if err := open.session.Run(uiCtx); err != nil {
			view.Closed(err.Error())
		}
	}()

	return <-done
}

// pushHealth keeps the link indicator current and posts a line when the verdict
// changes, so a path that goes quiet says so instead of just looking idle.
func pushHealth(ctx context.Context, open *channel, view *tui.Chat) {
	go watchPath(ctx, open, func(recovery Recovery) {
		view.System(recoveryMessage(recovery))
	})

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			health := open.session.Health()
			view.SetMembers([]tui.Participant{{
				Name:    open.nicknames.Other(open.role),
				Link:    linkState(health.Link),
				RTT:     health.RTT,
				Silence: health.Silence,
			}})
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
func connect(ctx context.Context, open *channel, view *tui.Chat) error {
	view.SetStatus("looking up your public endpoint", 0.05)

	open.watcher.OnChange(func(_, current *net.UDPAddr) {
		view.SetInvite(open.inviteFor(current))
		view.System(movedMessage(open, current))
	})
	go trackProgress(ctx, open, view)

	err := open.waitForPath(ctx, func(endpoint *net.UDPAddr) {
		if open.role == punch.RoleJoiner {
			if open.established.Load() {
				return
			}
			view.SetStatus("punching towards your friend", 0.2)
			if endpoint != nil {
				view.SetInvite("if it stalls, send back: " + open.inviteFor(endpoint))
			}
			return
		}
		if endpoint == nil {
			view.SetStatus("no STUN server answered, there is no address to share", 0.2)
			view.SetInvite("something here is blocking STUN. Try: vapora nat")
			return
		}
		view.SetStatus("waiting for your friend to join", 0.2)
		view.SetInvite(open.inviteFor(endpoint))
	})
	if err != nil {
		return err
	}

	view.SetStatus("connected", 1)
	return nil
}

// trackProgress advances the loading meter with elapsed time. There is nothing
// better to measure: a punch either lands or it does not, so the bar shows how
// much of the budget is gone rather than pretending to know more.
func trackProgress(ctx context.Context, open *channel, view *tui.Chat) {
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
				view.SetStatus("your friend arrived, opening the path", 0.85)
				announced = true
			}
			elapsed := time.Since(started).Seconds() / open.timeout.Seconds()
			view.SetProgress(0.2 + 0.6*elapsed)
		}
	}
}
