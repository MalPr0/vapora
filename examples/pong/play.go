package main

import (
	"context"
	"fmt"
	"time"

	"github.com/MalPr0/vapora/pkg/punch"
)

// play is the loop, and it is the same shape on both machines: read the
// keyboard, read the network, draw. What differs is who owns the ball.
func (t *table) play(ctx context.Context, hosting bool) error {
	terminal, err := rawMode()
	if err != nil {
		return err
	}
	defer func() {
		terminal.restore()
		fmt.Print(leaveAlternate)
	}()

	fmt.Print(enterAlternate)

	var (
		display screen
		world   = newGame()
		state   State
		pressed = keys()
		ticker  = time.NewTicker(tickRate)
	)
	defer ticker.Stop()

	// Which paddle this side controls: the host is the left one.
	me := 0
	if !hosting {
		me = 1
	}

	for {
		select {
		case <-ctx.Done():
			return nil

		case key, open := <-pressed:
			if !open {
				return nil
			}
			switch key {
			case 'q', 3: // q or ctrl+c
				t.session.Goodbye()
				return nil
			case 'w', 'k':
				world.move(me, -paddleSpeed)
			case 's', 'j':
				world.move(me, paddleSpeed)
			case 'r':
				if hosting {
					world.reset()
				} else {
					// The guest asks; the host is the one who decides.
					t.session.Send(encodeReset())
				}
			}

		case payload := <-t.incoming:
			if hosting {
				// The only two things the host believes from the network.
				if y, ok := decodePaddle(payload); ok {
					world.paddle[1] = y
				}
				if isReset(payload) {
					world.reset()
				}
				continue
			}
			if received, ok := decodeState(payload); ok {
				state = received
			}

		case <-ticker.C:
			if hosting {
				world.tick()
				state = world.state
				t.session.Send(encodeState(state))
			} else {
				// The joiner is authoritative about one thing only: its own
				// paddle. Everything else it draws is the host's word.
				state.RightY = world.paddle[1]
				t.session.Send(encodePaddle(world.paddle[1]))
			}

			display.draw(state)
			fmt.Print(display.render(state, t.me, t.them, t.status(world, hosting)))
		}
	}
}

// status is the line under the court. It says what the path is doing, because
// on a direct connection between two homes that is the thing most likely to go
// wrong, and a game that just freezes tells you nothing.
func (t *table) status(world *game, hosting bool) string {
	health := t.session.Health()

	switch {
	case world.finished():
		winner := t.me
		if (world.state.RightScore >= winningScore) == hosting {
			winner = t.them
		}
		return winner + " wins  ·  r plays again  ·  q quits"
	case health.Link == punch.LinkLost:
		return "\x1b[38;5;196mconnection lost\x1b[0m\x1b[38;5;250m  ·  q quits"
	case health.Link == punch.LinkStale:
		return fmt.Sprintf("no reply for %ds  ·  q quits", int(health.Silence.Seconds()))
	default:
		return fmt.Sprintf("w/s moves  ·  r resets  ·  %dms  ·  q quits", health.RTT.Milliseconds())
	}
}
