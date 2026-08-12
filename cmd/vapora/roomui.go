package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/MalPr0/vapora/internal/tui"
	"github.com/MalPr0/vapora/pkg/chat"
	"github.com/MalPr0/vapora/pkg/punch"
)

// runRoomUI is the full screen front end for a room. It is the same view the
// two way chat uses: a room is a roster of several rather than a different
// screen, and one participant is just the small case of that.
func runRoomUI(ctx context.Context, open *room) error {
	terminal, err := tui.OpenTerminal(os.Stdin, os.Stdout)
	if err != nil {
		return fmt.Errorf("%w: %w", errNoTerminal, err)
	}

	defer func() {
		terminal.Restore()
		open.report()
	}()

	view := tui.NewChat(terminal, open.group.Me().Name)
	view.OnSend(open.group.Say)
	view.OnTyping(open.group.SetTyping)
	view.OnCommand(func(line string) bool { return open.command(line, view) })
	open.group.OnLine(func(from chat.Speaker, line string) { view.Message(from.Name, line) })
	open.group.OnTyping(func(from chat.Speaker, active bool) { view.Typing(from.Name, active) })

	uiCtx, stopUI := context.WithCancel(ctx)
	defer stopUI()

	done := make(chan error, 1)
	go func() { done <- view.Run(uiCtx) }()

	go func() {
		if err := open.connect(uiCtx, view); err != nil {
			if errors.Is(err, context.Canceled) {
				return // the user quit while waiting; report() says so already
			}
			open.note(err.Error())
			view.Closed(err.Error())
			return
		}
		view.Connected(open.group.Me().Name)
		go open.pushRoster(uiCtx, view)
	}()

	return <-done
}

// pushRoster keeps the header current. Everything about a path is polled, and
// so is who is present: nothing arrives to announce that somebody stopped
// arriving.
func (r *room) pushRoster(ctx context.Context, view *tui.Chat) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	known := map[punch.PublicKey]string{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			members := r.group.Speakers()

			participants := make([]tui.Participant, 0, len(members))
			present := map[punch.PublicKey]string{}
			for _, member := range members {
				present[member.Key] = member.Name
				if _, seen := known[member.Key]; !seen {
					view.System(member.Name + " joined")
				}
				participants = append(participants, tui.Participant{
					Name:    member.Name,
					Link:    linkState(member.Health.Link),
					RTT:     member.Health.RTT,
					Silence: member.Health.Silence,
				})
			}
			for key, name := range known {
				if _, still := present[key]; !still {
					view.System(name + " left")
				}
			}
			known = present
			view.SetMembers(participants)

			if !r.quorum(stillHereNamed(members)) {
				view.System("everybody left, so the room is closing")
				view.Quit()
				return
			}
		}
	}
}

// command handles the lines that are instructions rather than conversation.
func (r *room) command(line string, view *tui.Chat) bool {
	switch {
	case isExit(line):
		r.room.Goodbye()
		view.Quit()
		return true
	case trimmed(line) == "!who":
		view.System(r.describeMembers())
		return true
	case r.pasteWhileWaiting(line, view):
		return true
	case trimmed(line) == "!invite":
		r.mu.Lock()
		invite := r.shared
		r.mu.Unlock()

		if invite == "" {
			view.System("no address to put on an invite yet")
			return true
		}
		view.System(invite)
		return true
	}
	return false
}

// pasteWhileWaiting takes an address the other side sent back. It only applies
// before anybody is here: once the room has members, a line that looks like an
// address is far more likely to be somebody talking about one.
func (r *room) pasteWhileWaiting(line string, view *tui.Chat) bool {
	if strings.HasPrefix(strings.TrimSpace(line), "!") || len(r.room.Members()) > 0 {
		return false
	}

	where, ok := r.reach(context.Background(), line)
	if !ok {
		return false
	}
	view.System("punching towards " + where + ", they have to be running too")
	return true
}

func (r *room) describeMembers() string {
	members := r.group.Speakers()
	if len(members) == 0 {
		return "nobody else is here yet"
	}

	description := fmt.Sprintf("%d present:", len(members))
	for _, member := range members {
		description += fmt.Sprintf("  %s (%s)", member.Name, member.Health.Link)
	}
	return description
}
