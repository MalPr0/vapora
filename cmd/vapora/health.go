package main

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/MalPr0/vapora/pkg/punch"
	"github.com/MalPr0/vapora/pkg/stun"
)

const healthPoll = time.Second

// Recovery is what the path has done since it was opened. All of it is polled
// rather than pushed: nothing arrives to announce that nothing is arriving, and
// a migration is noticed by the session itself.
type Recovery struct {
	Health punch.Health
	// Moves counts how many times the peer was followed to a new address.
	Moves int
	// Moved marks the report that is about a migration rather than about the
	// link changing verdict. Without it a single recovery reads as two
	// separate events, because it is both.
	Moved bool
}

// watchPath reports only when the verdict changes. A live link is silent by
// design: the probe and its answer never surface, so the only thing worth
// saying is that they stopped, that the peer moved, or that this side did.
func watchPath(ctx context.Context, open *channel, report func(Recovery)) {
	ticker := time.NewTicker(healthPoll)
	defer ticker.Stop()

	link := punch.LinkAlive
	moves := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			health := open.session.Health()
			current := open.session.Moves()

			if current != moves {
				moves = current
				report(Recovery{Health: health, Moves: current, Moved: true})
			}
			if health.Link != link {
				link = health.Link
				report(Recovery{Health: health, Moves: current})
			}
		}
	}
}

// watchEndpoint keeps the socket's public address under observation and hands
// the answers to the watcher through the session, which owns the only reader.
func watchEndpoint(ctx context.Context, open *channel, moved func(previous, current *net.UDPAddr)) {
	watcher := stun.NewWatcher(stun.DefaultServers, open.keepalive)
	watcher.OnChange(moved)
	open.session.Sniff(watcher.Handle)

	_ = watcher.Run(ctx, open.conn)
}

func recoveryMessage(recovery Recovery) string {
	if recovery.Moved {
		return "your friend moved to a new address and the path followed them"
	}

	switch recovery.Health.Link {
	case punch.LinkStale:
		return "no answer for a few seconds, punching again in case the routers forgot us"
	case punch.LinkLost:
		return "link lost. Probing continues, so it will come back on its own if they return to the same address"
	default:
		return "link back"
	}
}

func movedMessage(open *channel, current *net.UDPAddr) string {
	return fmt.Sprintf("your address changed to %s, so the invite you shared is dead. Send this one:\n    %s",
		current, open.inviteFor(current))
}
