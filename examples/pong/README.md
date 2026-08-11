```
        █▀▀▀▄ ▄▀▀▀▄ █▄  █ ▄▀▀▀▄
        █▄▄▄▀ █   █ █▀▄ █ █  ▄▄
        █     █   █ █  ██ █   █
        ▀      ▀▀▀  ▀   ▀  ▀▀▀

     ▀▀▀ p o w e r e d   b y   v a p o r a ▀▀▀
```

**Two players. Two houses. No server.** A 200-line tutorial that builds a real
game on `pkg/punch`, and in doing so shows the transport being used for
something that is nothing like a chat.

← [back to the README](../../README.md) · [the transport tour](../../ARCHITECTURE.md)

---

## Run it

```bash
go run ./examples/pong host          # prints an invite, waits
go run ./examples/pong join <invite> # the other machine
```

`w`/`s` moves. `q` quits. That is the whole game.

```
  CRIMSON OTTER 3   —   2 JADE HERON
  ───────────────────────────────────────
                                          
    █                  ██                 
    █                  ██              █  
    █                                  █  
    █                                  █  
                                          
  ───────────────────────────────────────
  w/s moves  ·  47ms  ·  q quits
```

---

## The idea that makes it work

A chat and a game want opposite things from the same channel.

```
   CHAT                            GAME
   ────                            ────
   sends EVENTS                    sends STATE
   every one matters               only the last one matters
   a lost line is lost             a lost packet is fixed 33ms later
   needs delivery                  needs freshness
```

That is the whole design. The host sends **the entire world** thirty times a
second — ball, paddles, score, eleven bytes — and the guest draws whatever
arrived most recently. Nothing is acknowledged, nothing is retransmitted,
nothing is ordered, and none of it is missed.

**A packet lost on the internet costs exactly one frame.** The next one carries
the full truth anyway.

At twelve bytes a tick and thirty ticks a second, the whole game is **under half
a kilobyte per second** in each direction.

---

## Step 1 · Open a channel

The network half is short enough to read in one go. A socket, a mux to read it,
a codec from the shared secret, a session on top.

```go
conn, _ := net.ListenUDP("udp4", &net.UDPAddr{})

codec, _ := punch.NewSecretCodec(secret, punch.RoleInviter)

mux := punch.NewMux(conn)
watcher := stun.NewWatcher(stun.DefaultServers, stun.DefaultKeepalive)
mux.Fallback(punch.SinkFunc(watcher.Handle))     // STUN replies

session := punch.NewSession(mux, codec, nil)
mux.Fallback(session)                            // anything that authenticates

go mux.Run(ctx)          // the only thing that reads the socket
go watcher.Run(ctx, conn)
go session.Run(ctx)
```

The host needs its own address before it can invite anybody:

```go
endpoint, _ := watcher.Wait(ctx, 15*time.Second)
fmt.Printf("pong join %s/%s\n", endpoint, secret)
```

Then both sides punch until the path opens:

```go
session.Open(ctx, 3*time.Minute)
```

<sup>That is <a href="main.go">main.go</a>, and it is the only part of this example that is about the network.</sup>

---

## Step 2 · Decide who is right

Somebody has to own the ball. If both machines simulated it they would drift
apart within seconds — two computers cannot agree on physics over a lossy link
without a great deal of machinery.

```
   HOST                                   GUEST
   ────                                   ─────
   owns the ball                          owns its own paddle
   owns both scores                       owns nothing else
   simulates every tick        ────▶      draws what it is told
                               ◀────      sends one number
```

The oldest answer in networked games, and still the right one at this size.

---

## Step 3 · Define your own wire

The transport gives you **one** frame kind for your bytes. If you need more than
one message, tag them *inside* the payload — then your numbering and the
transport's can never collide.

```go
const (
    tagPaddle byte = 1   // guest → host
    tagState  byte = 2   // host  → guest
)
```

The whole world is eleven bytes, twelve with the tag in front:

```
   tag  │            the entire game, every tick
  ──────┼──────────────────────────────────────────────────────
   1 B  │ ballX  ballY  leftY  rightY  scores  serving
        │  2 B    2 B    2 B    2 B     2 B      1 B     = 11
```

Positions travel as a fraction of a fixed field, not as terminal cells, so the
two players can have different window sizes and still agree about the game.

```go
const fieldWidth, fieldHeight = 1000, 1000
```

<sup><a href="wire.go">wire.go</a></sup>

---

## Step 4 · Refuse to trust what arrives

Everything from the network is a claim. The transport guarantees it came from
somebody holding the secret — **nothing more than that**.

```go
func decodeState(payload []byte) (State, bool) {
    if len(payload) != 1+stateBytes || payload[0] != tagState {
        return State{}, false          // not this program on the other end
    }
    return State{
        BallX: clamp16(binary.BigEndian.Uint16(payload[1:]), fieldWidth),
        ...
    }, true
}
```

Two rules, and they are the same two everywhere:

- **A length that does not match is not your program.** Drop it; do not guess.
- **Clamp anything that will index something.** A peer claiming the ball is at
  65535 would otherwise be writing past the end of your screen buffer.

---

## Step 5 · The loop

Identical on both machines. Only the ownership differs.

```go
select {
case key := <-pressed:
    switch key {
    case 'w', 'k': world.move(me, -paddleSpeed)   // my paddle, always mine
    case 's', 'j': world.move(me, paddleSpeed)
    }

case payload := <-t.incoming:
    if hosting {
        if y, ok := decodePaddle(payload); ok {
            world.paddle[1] = y             // the one thing I believe
        }
    } else if received, ok := decodeState(payload); ok {
        state = received                    // everything, replaced whole
    }

case <-ticker.C:                            // 33ms
    if hosting {
        world.tick()
        t.session.Send(encodeState(world.state))
    } else {
        t.session.Send(encodePaddle(world.paddle[1]))
    }
    display.draw(state)
    fmt.Print(display.render(state, t.me, t.them, t.status(world, hosting)))
}
```

**Never block the transport.** It delivers on its own goroutine, so the observer
hands off to a channel and drops when the game falls behind — a stale frame is
worth nothing, and a backlog of them is worth less:

```go
session.Observe(punch.ObserverFunc(func(payload []byte) {
    select {
    case built.incoming <- payload:
    default:                    // behind? the newest one is along in 33ms
    }
}))
```

---

## Step 6 · Say what the connection is doing

A game between two houses will have a bad minute. If it just freezes, the player
blames your game.

```go
health := t.session.Health()

switch health.Link {
case punch.LinkLost:  return "connection lost"
case punch.LinkStale: return fmt.Sprintf("no reply for %ds", int(health.Silence.Seconds()))
default:              return fmt.Sprintf("%dms", health.RTT.Milliseconds())
}
```

The transport measures the path for you, with a padded ping whose reply is
padded independently — so neither is recognisable by size to somebody watching.

---

## What this proved

The tests beside this file are the point of the exercise:

| | |
|---|---|
| `TestTheBallReachesTheOtherSide` | 60 ticks, and the ball is seen in a dozen places — the state really crosses |
| `TestThePaddleReachesTheHost` | the guest's one authority arrives intact |
| `TestALostPacketCostsNothing` | a state survives its own encoding whole, which is why nothing needs retransmitting |
| `TestNonsenseFromTheNetworkIsRefused` | wrong lengths dropped, impossible positions clamped |
| `TestATickIsSmall` | eleven bytes, which is why 30/second is nothing |

They build their sessions through the exported API only, exactly as the game
does. If a game could not be assembled from outside the transport, the layering
would be a claim rather than a fact.

```bash
go test ./examples/pong/ -race
```

---

## What is missing, honestly

This is a tutorial, not a shipped game.

- **No interpolation.** At 30 ticks the ball steps rather than glides. Real
  games interpolate between the last two states.
- **No prediction.** The guest's paddle waits a round trip. Over 200ms that is
  felt; real games move locally and reconcile.
- **The host decides everything**, so the host cannot lose to lag and the guest
  cannot win because of it. Fine among friends, not fine for anything ranked.
- **Terminal handling is unix only.** The `pkg/` half is portable; termios is
  not.

None of that is about the channel. All of it is the part you would write next.

---

<sup>Not shipped in releases — <code>cmd/vapora</code> is what gets built. This lives here to be read.</sup>
