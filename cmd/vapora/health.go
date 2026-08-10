package main

import (
	"context"
	"time"

	"github.com/MalPr0/vapora/pkg/punch"
)

const healthPoll = time.Second

// watchHealth polls the path and reports only when the verdict changes. A live
// link is silent by design: the ping and its answer never surface, so the only
// thing worth saying is that they stopped.
func watchHealth(ctx context.Context, session *punch.Session, onChange func(punch.Health)) {
	ticker := time.NewTicker(healthPoll)
	defer ticker.Stop()

	last := punch.LinkAlive
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			health := session.Health()
			if health.Link != last {
				last = health.Link
				onChange(health)
			}
		}
	}
}

func linkMessage(health punch.Health) string {
	switch health.Link {
	case punch.LinkStale:
		return "no answer for a few seconds, still probing"
	case punch.LinkLost:
		return "link lost. Nothing has arrived in a while; probes keep running in case it comes back"
	default:
		return "link back"
	}
}
