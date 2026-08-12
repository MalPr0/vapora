package main

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/MalPr0/vapora/pkg/punch"
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
	// Probes is traffic that reached the socket and could not authenticate.
	Probes punch.Probes
	// Probed marks the report that is about that traffic.
	Probed bool
}

// watchPath reports only when the verdict changes. A live link is silent by
// design: the probe and its answer never surface, so the only thing worth
// saying is that they stopped, that the peer moved, or that this side did.
func watchPath(ctx context.Context, open *channel, report func(Recovery)) {
	ticker := time.NewTicker(healthPoll)
	defer ticker.Stop()

	link := punch.LinkAlive
	moves := 0
	warned := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			health := open.session.Health()
			current := open.session.Moves()
			probes := open.session.Probes()

			if current != moves {
				moves = current
				report(Recovery{Health: health, Moves: current, Moved: true})
			}
			if health.Link != link {
				link = health.Link
				report(Recovery{Health: health, Moves: current})
			}
			// Doubling keeps a sustained scan from filling the conversation
			// with its own noise while still saying it is getting worse.
			if probes.Count >= nextProbeWarning(warned) {
				warned = probes.Count
				report(Recovery{Health: health, Probes: probes, Probed: true})
			}
		}
	}
}

// nextProbeWarning is the count at which the next warning is worth printing.
func nextProbeWarning(warned int) int {
	if warned == 0 {
		return 1
	}
	return warned * 2
}

func recoveryMessage(recovery Recovery) string {
	if recovery.Probed {
		if recovery.Probes.Impostors > 0 {
			return fmt.Sprintf(
				"%d packets from %d address(es) carried your session key but did not come from your friend. "+
					"Somebody else is holding this invite and using it. They were ignored and cannot take "+
					"over the conversation, but the invite is no longer private: quit and start a new session",
				recovery.Probes.Impostors, recovery.Probes.Sources)
		}
		return fmt.Sprintf(
			"%d packets from %d address(es) reached this port and could not authenticate. "+
				"Nothing was answered, so they learned nothing, but somebody knows this address: "+
				"treat the invite as public and start a new session if that is a surprise",
			recovery.Probes.Count, recovery.Probes.Sources)
	}
	if recovery.Moved {
		return "your friend moved to a new address and the path followed them"
	}

	if recovery.Health.Departed {
		return "your friend left"
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
