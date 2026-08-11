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

**Contents** · [Run it](#run-it) · [The smallest version](#step-0--the-smallest-thing-that-runs) ·
[The idea](#the-idea-that-makes-it-work) · [Open a channel](#step-1--open-a-channel) ·
[Who is right](#step-2--decide-who-is-right) · [Your own wire](#step-3--define-your-own-wire) ·
[Trust nothing](#step-4--refuse-to-trust-what-arrives) · [The loop](#step-5--the-loop) ·
[Say what is happening](#step-6--say-what-the-connection-is-doing) ·
[What it proved](#what-this-proved) · [Take it further](#take-it-to-your-own-game)

---

## Run it

```bash
go run ./examples/pong host          # prints an invite, waits
go run ./examples/pong join <invite> # the other machine
```

`w`/`s` moves, `r` starts again, `q` quits. First to eleven.

```
      █▀▀▀▄ ▄▀▀▀▄ █▄  █ ▄▀▀▀▄
      █▄▄▄▀ █   █ █▀▄ █ █  ▄▄
      █     █   █ █  ██ █   █
      ▀      ▀▀▀  ▀   ▀  ▀▀▀

      powered by

      █   █  ▄▀▄  █▀▀▀▄ ▄▀▀▀▄ █▀▀▀▄  ▄▀▄
      █   █ █   █ █▄▄▄▀ █   █ █▄▄▄▀ █   █
      ▀▄ ▄▀ █▀▀▀█ █     █   █ █  ▀▄ █▀▀▀█
        ▀   ▀   ▀ ▀      ▀▀▀  ▀   ▀ ▀   ▀

      direct, encrypted, no server in the middle

      run this on the other machine:

        pong join 203.0.113.7:41001/BXFWOBXKGS547XF2WOKVG6JYDI

      waiting for a challenger...
```

Then the court, which is the whole game:

```
  QUAIL 7   —   6 WAPITI
  ───────────────────────────────────────
                                          
    █                    ▄                
    █                    █             █  
                                       █  
                                          
  ───────────────────────────────────────
  w/s moves  ·  r resets  ·  47ms  ·  q quits          powered by vapora
```

---

## Step 0 · The smallest thing that runs

Before the game, the skeleton. This is a complete program: two copies of it on
two machines, anywhere on the internet, sending each other bytes.

```go
package main

import (
    "context"
    "fmt"
    "net"
    "os"
    "time"

    "github.com/MalPr0/vapora/pkg/punch"
    "github.com/MalPr0/vapora/pkg/stun"
)

func main() {
    ctx := context.Background()
    conn, _ := net.ListenUDP("udp4", &net.UDPAddr{})

    // Host mints a secret; guest gets it from the invite.
    secret, role := punch.Secret(nil), punch.RoleInviter
    var peer *net.UDPAddr
    if len(os.Args) > 1 {
        invite, _ := punch.ParseInvite(os.Args[1])
        secret, role, peer = invite.Secret, punch.RoleJoiner, invite.Endpoint
    } else {
        secret, _ = punch.NewSecret()
    }

    codec, _ := punch.NewSecretCodec(secret, role)

    mux := punch.NewMux(conn)
    watcher := stun.NewWatcher(stun.DefaultServers, stun.DefaultKeepalive)
    mux.Fallback(punch.SinkFunc(watcher.Handle))

    session := punch.NewSession(mux, codec, nil)
    mux.Fallback(session)
    if peer != nil {
        session.SetPeer(peer)
    }

    session.Observe(punch.ObserverFunc(func(payload []byte) {
        fmt.Println("←", string(payload))
    }))

    go mux.Run(ctx)
    go watcher.Run(ctx, conn)
    go session.Run(ctx)

    if peer == nil {
        endpoint, _ := watcher.Wait(ctx, 15*time.Second)
        fmt.Printf("run: go run . %s/%s\n", endpoint, secret)
    }

    if err := session.Open(ctx, 3*time.Minute); err != nil {
        fmt.Println("no path:", err)
        return
    }

    for range time.Tick(time.Second) {
        session.Send([]byte("hola"))
    }
}
```

Forty lines, no dependencies, and the hard part — two routers that both refuse
strangers — is already handled. Everything after this is your program, not the
network.

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
    tagReset  byte = 3   // guest → host: start again
)
```

A reset shows the design paying off. The guest cannot reset anything — only the
host simulates — so it sends one byte asking, and then learns the new score from
the next state, like it learns everything else. No acknowledgement, no special
case, no way for the two sides to disagree about whether it happened.

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
| `TestEitherSideCanAskToStartAgain` | the guest asks, the host decides, the new score arrives as ordinary state |

They build their sessions through the exported API only, exactly as the game
does. If a game could not be assembled from outside the transport, the layering
would be a claim rather than a fact.

```bash
go test ./examples/pong/ -race
```

---

## What playing it taught us

Three full matches between two people, and every lesson was about the game
rather than the channel.

**A still paddle beat a person 7-6.** The first build had a paddle covering 16%
of the court and a ball that barely changed angle. Nothing was broken; there was
simply no difficulty in it. Slower ball, smaller ball, and eleven points to win.

**Chasing the ball loses to anticipating it.** One side was played by a script
reading the screen every 120ms and moving towards wherever the ball currently
was. At 12 units a tick and 30 ticks a second the ball travels **43 units
between one look and the next**, so the paddle was permanently aiming at where
the ball had been. It went 4-1 up and lost 4-11 the moment the other player
started using angles.

That is the same problem every networked game has, in miniature. It is why real
ones **predict** rather than follow, and it is what the missing pieces below are
for.

---

## What is missing, honestly

This is a tutorial, not a shipped game. None of it is about the channel.

| Missing | What it would take |
|---|---|
| **Interpolation** | At 30 ticks the ball steps rather than glides. Draw between the last two states instead of on top of the newest. |
| **Prediction** | The guest's paddle waits a round trip; over 200ms that is felt. Move locally at once, reconcile when the host's version arrives. |
| **A fair authority** | The host cannot lose to lag and the guest cannot win because of it. Fine among friends, not fine for anything ranked. |
| **Windows** | The `pkg/` half is portable. termios is not. |

---

## Take it to your own game

The parts of this that are not about Pong:

- **Send state, not events**, whenever the newest message makes the older ones
  irrelevant. It buys you immunity to packet loss for free.
- **Put your tags inside the payload.** The transport gives you one frame kind;
  your numbering lives under it and cannot collide with anything.
- **Pick one owner per fact.** The ball has exactly one, the guest's paddle has
  exactly one. Anything with two owners will disagree, and then you are writing
  a consensus algorithm instead of a game.
- **Bound and clamp everything that arrives.** The transport proves who sent it,
  never that it makes sense.
- **Never block in the observer.** Hand off to a channel and drop when full: for
  state, the newest is along in a moment and the backlog is worth nothing.
- **Show the connection.** `session.Health()` gives you round trip and silence.
  A game that freezes without saying why gets blamed for the network.

---

## The files

| | |
|---|---|
| [`main.go`](main.go) | The network setup, and the only part about the internet |
| [`wire.go`](wire.go) | The protocol: three tags and eleven bytes |
| [`game.go`](game.go) | The rules, which run on the host only |
| [`play.go`](play.go) | The loop, identical on both sides |
| [`screen.go`](screen.go) | Half-block drawing |
| [`splash.go`](splash.go) | The wordmarks |
| [`keys.go`](keys.go) | Raw mode, about thirty lines of termios |
| [`pong_test.go`](pong_test.go) | The checks in the table above |

---

<sup>Not shipped in releases — <code>cmd/vapora</code> is what gets built. This lives here to be read.</sup>
