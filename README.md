```
██      ██      ██      ████████      ██████    ████████        ██
██      ██    ██  ██    ██      ██  ██      ██  ██      ██    ██  ██
██      ██  ██      ██  ██      ██  ██      ██  ██      ██  ██      ██
██      ██  ██      ██  ████████    ██      ██  ████████    ██      ██
██      ██  ██████████  ██          ██      ██  ██    ██    ██████████
  ██  ██    ██      ██  ██          ██      ██  ██      ██  ██      ██
    ██      ██      ██  ██            ██████    ██      ██  ██      ██
```

### Chat straight from your computer to theirs. No server. No account. No trace.

You share one line of text. They paste it. You are talking — encrypted,
directly, with nothing in between.

[![release](https://img.shields.io/github/v/release/MalPr0/vapora?style=flat-square&color=e8a33d)](https://github.com/MalPr0/vapora/releases/latest)
![go](https://img.shields.io/badge/go-1.25-00ADD8?style=flat-square)
![dependencies](https://img.shields.io/badge/dependencies-zero-2ea043?style=flat-square)
![license](https://img.shields.io/badge/license-MIT-blue?style=flat-square)

---

## Try it in 30 seconds

```bash
curl -fsSL https://github.com/MalPr0/vapora/releases/latest/download/vapora_darwin_arm64.tar.gz | tar -xz
./vapora punch
```

It prints one line. Send it to a friend. They paste it into their terminal.

<sup>Other builds: `darwin_amd64` · `linux_amd64` · `linux_arm64` · `windows_amd64.zip` — swap the name in the URL. Use `curl`, not your browser: a browser marks downloads as untrusted and macOS then refuses to run them.</sup>

---

## What it looks like

```
 █   █  ▄▀▄  █▀▀▀▄ ▄▀▀▀▄ █▀▀▀▄  ▄▀▄                    ● JADE HERON     31ms
 █   █ █   █ █▄▄▄▀ █   █ █▄▄▄▀ █   █                   ● SWIFT OTTER    47ms
 ▀▄ ▄▀ █▀▀▀█ █     █   █ █  ▀▄ █▀▀▀█                   ◐ GREY MARTEN  no reply 9s
   ▀   ▀   ▀ ▀      ▀▀▀  ▀   ▀ ▀   ▀
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ you are CRIMSON QUAIL ━━━━━━━━━━━━━━━━━━━━━━━━━

  --             JADE HERON joined
  JADE HERON     anyone there?
  SWIFT OTTER    @QUAIL check this out
▸ CRIMSON QUAIL  on it
  GREY MARTEN    ...

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
> hola_
                        enter sends · pgup/pgdn scrolls · !exit quits
```

A retro pixel-art terminal chat. Everyone gets an animal name nobody can claim,
`@mentions` pull a line out of the scroll, and a little runner sprints across a
loading screen while the connection punches through.

---

## Why you might want this

**Nobody is in the middle.** Your words go from your machine to theirs. Not
through a company's servers, not through mine. There is no middle to subpoena,
sell, or breach.

**Nothing to sign up for.** No email, no phone number, no username, no profile.
The program does not know who you are, and neither does anybody else.

**Nothing is stored.** Close it and the conversation is gone from both ends.
There is no history to leak, because there is no history.

**One file, zero dependencies.** Download a binary and run it. No Docker, no
runtime, no install. Built from the Go standard library and nothing else — you
can read every line that ships.

**Encrypted by default, with no way to turn it off.** AES-256-GCM, a different
key each direction. The invite you share *is* the key.

**Groups are a real mesh.** Everyone talks to everyone directly. Two people in a
room of five have a channel the other three cannot read — not as a promise about
behaviour, as arithmetic. They do not have the keys.

---

## What people use it for

- **Sending something sensitive** to a colleague without it living in a company
  chat log forever.
- **Talking across a firewall** where you cannot open ports or install anything.
- **A quick channel with a friend** that leaves no account, no history and no
  trace on either machine.
- **Understanding your own connection** — the diagnostics tell you more about
  your network than your provider will.

---

## Two people

```bash
./vapora punch                 # you: prints an invite
./vapora punch "<the invite>"  # them: pastes and runs it
```

**If it does not connect, both of you send an invite.** Home routers usually
refuse packets from strangers, so when both do that, each first packet dies at
the other's door. Their screen prints a line under *"if it does not connect,
send this back"* — send it, paste it into your terminal, and now both of you are
knocking at the same moment. That is exactly what those routers need to see.

You can find out in advance whether you will need that step — see
[diagnostics](#know-your-network-before-you-blame-it).

## A group

```bash
./vapora room                  # opens a room, prints an invite
./vapora room "<the invite>"   # anyone joins with it
```

**Anyone can invite.** Joined five minutes ago? `!invite` gives you a line to
bring in the next person. Everybody ends up knowing everybody without going back
to whoever started it.

**Whoever invited you is not a server.** They introduce two people and step out
of the way. They carry nothing between them and could not read it if they tried.
Close the machine that opened the room and the conversation carries on without
it.

**Rooms hold eight**, and they **close once they empty out** — a room nobody is
in is a port with nobody behind it. Add `-standalone` if you want one to sit and
wait.

**Two of you on the same wifi?** That works too. Each participant announces both
its public and its local address, because two machines behind one router cannot
reach each other through the public one. It settles itself in a few seconds.

### While you are in one

| | |
|---|---|
| `@name` | pulls your line out of their scroll, with a mark in the margin |
| `!who` | who is here, and how healthy each connection is |
| `!invite` | a fresh invite to bring somebody in |
| `!exit` | leave, and tell everyone at once |
| `PgUp` / `PgDn` | scroll back through what was said |
| `-plain` | plain lines instead of the full screen, for when something is wrong |

---

## How it works

Your computer has no address of its own on the internet. Your router has one and
everything in your home shares it. That is **NAT**, and it is why nobody can
simply call your laptop. The usual answer is a server in the middle that both
sides connect *out* to — which works, and means somebody else's computer sees
every word.

vapora does the other thing. Both sides send packets *out* at the same moment,
each punching a hole through its own router, and the two holes line up. After
that the path is direct and nobody else is on it.

| What | Why it is there |
|---|---|
| **UDP hole punching** | The direct path itself. Both sides punch at once and meet in the middle. |
| **STUN** ([5389](https://www.rfc-editor.org/rfc/rfc5389), [5780](https://www.rfc-editor.org/rfc/rfc5780)) | Learns what the outside world sees as your address, and classifies how your router behaves. |
| **UPnP-IGD, PCP, NAT-PMP** | Three languages for asking a router to open a door. It tries all three, because routers rarely agree on which they speak. |
| **X25519 + HKDF + AES-256-GCM** | A separate key per pair, per direction. In a room, no member can read another pair's traffic. |
| **Anti-replay window** | IPsec-style sliding window, per sender, so a captured packet cannot be played back at you. |
| **BitTorrent DHT** *(opt-in)* | Find each other with no address at all. Off by default — see [security](#security). |

All of it from the Go standard library. No third-party code, anywhere.

<sup><a href="ARCHITECTURE.md">ARCHITECTURE.md</a> has the step by step, with diagrams.</sup>

---

## Know your network before you blame it

```bash
./vapora nat                   # what kind of router you are behind
./vapora diag                  # every router between you and the internet
```

`nat` prints a short profile like `CONE-PORT-18`. Send it to whoever you are
connecting to, put theirs in, and it tells you what to expect **before** you
waste an evening:

```bash
./vapora nat -pair CONE-OPEN-64                    # for two people
./vapora nat -room "CONE-PORT-18,SYM-PORT-F3"      # for a whole room
```

Whether a connection works is a property of the *pair*, not of either end — no
measurement of your own network can answer it alone. That is why the profile is
built to be pasted to somebody else. For a room it goes further: it says whether
the mesh closes, who should host, and exactly which pair will never reach each
other.

<sup>If a firewall opens one specific port, measure that one: <code>vapora nat -port 41000</code>. Filtering is a property of a port, not of a machine.</sup>

---

## Security

**The invite is the key.** That string is not an address, it is the secret that
encrypts everything. Treat it like a password: anyone who sees it — in a
screenshot, a group chat, over a shoulder — can use it.

**Silence to strangers.** Packets without the right key get no reply at all. A
port scanner learns exactly what it would learn from a closed port. But they are
counted, and **you are told**, because it means somebody found an address that
should only ever have been on one invite.

**Nobody can take over your conversation.** Hand your invite to a third person
and they still cannot push your friend out. The program tells them apart,
ignores the newcomer, and warns you.

**Only text crosses.** Anything else is dropped rather than shown. And text from
the network is stripped of the escape sequences that would let someone move your
cursor, repaint your screen, or reach your clipboard.

**An invite stays valid until you close the program.** There is no expiry and no
way to revoke one. Closing and reopening *is* the revocation — it gives you a
new key and usually a new address.

**In a room, a member can lie about who else is present.** They can announce
somebody who does not exist. What they cannot do is read or forge what two other
people say to each other. A made-up member never answers and drops off on its
own.

**"No account" is not the same as invisible.** The person you talk to sees your
IP address. They must — the packets go from your home to theirs. That is what
*direct* means, and it is the honest trade for having no server.

**`-discover` publishes your address on a public network**, which is why it is
off by default. With it, both sides find each other through the BitTorrent DHT
under a name derived from your secret. Nobody can look you up without that
secret, but you do become one more address in a table anyone can crawl.

---

## What will break, and when

Honest limitations, not fine print.

- **The STUN servers belong to somebody else** — Google, Cloudflare and two
  others, free services run for other purposes. If they go, this cannot learn
  its own address, and there is no fallback today.
- **Some networks block it outright**: companies, universities, hotels, some
  mobile carriers. Nothing on your end fixes that.
- **Some connections cannot do it at all.** A *symmetric* or carrier-grade NAT
  makes your address unpredictable from moment to moment, so there is nothing to
  aim at. `vapora nat` tells you. The only fix is a relay, which this
  deliberately does not have.
- **Your address changes and the invite dies.** Switch wifi, move to mobile
  data, sit idle long enough. It notices and prints a new one, but you have to
  send it again.
- **Versions must match.** The wire format has changed several times and will
  change again. Old and new do not interoperate, and the symptom is *silence*.
  Both of you run `./vapora version` first.
- **Nothing is protected after the fact.** Someone who records your traffic
  today and obtains your invite later can read that recording. Serious tools
  solve this with keys thrown away as you go. This one does not.
- **The binaries are not signed.** Your operating system will warn you, and it
  is right to. Verify the checksum against `SHA256SUMS`, or build it yourself.
- **`vapora serve` changes your router's configuration.** It is the original
  UPnP demo, and the one command here that asks your router to open a port to
  the internet. It closes it again on exit — but if it crashes, that door may
  stay open until the router restarts. Everything else in this README leaves
  your router alone.
- **Nobody who breaks software for a living has reviewed this.** Being built
  carefully is not the same as being audited. Do not stake anything that matters
  on it.

---

## How you can use this

The chat is one thing built on the channel. Here is another, in forty lines:
two copies of this program, on two machines anywhere on the internet, sending
each other bytes with nothing in between.

```go
conn, _ := net.ListenUDP("udp4", &net.UDPAddr{})

codec, _ := punch.NewSecretCodec(secret, punch.RoleInviter)
mux := punch.NewMux(conn)
session := punch.NewSession(mux, codec, nil)
mux.Fallback(session)

session.Observe(punch.ObserverFunc(func(payload []byte) {
    fmt.Println("←", string(payload))       // whatever they sent, exactly
}))

go mux.Run(ctx)
go session.Run(ctx)

session.Open(ctx, 3*time.Minute)             // punch through both routers
session.Send([]byte("hola"))
```

### 🏓 Start here: [**build a Pong game**](examples/pong)

A step-by-step tutorial that goes from that skeleton to a real two-player game
across the internet — its own wire format, who is allowed to be right about
what, and why a game survives packet loss that would ruin a conversation.

```
  QUAIL 7   —   6 WAPITI
  ───────────────────────────────────────
    █                    ▄
    █                    █             █
                                       █
  ───────────────────────────────────────
  w/s moves · r resets · 47ms · q quits        powered by vapora
```

### The three, side by side

| | Sends | Cares about |
|---|---|---|
| **[Pong](examples/pong)** — tutorial | **state**, 30×/second | only the newest. A lost packet costs one frame |
| **[filedrop](examples/filedrop)** | **blocks** of a file | all of them, in the right places |
| **`vapora punch` / `room`** | **events** — lines of text | every single one |

A game and a conversation want opposite things from the same transport —
freshness against delivery — and neither needed the transport to change. That is
the clearest evidence the layering is real, and it is why building on it does
not mean inheriting anybody else's decisions.

---

## Use it for something else

The chat is one thing built on the channel, not the point of it. The transport
is a separate layer with no idea what a conversation is: it opens an encrypted
path through two routers, keeps a mesh alive, and moves **bytes**.

```go
session := punch.NewSession(mux, codec, nil)
session.Observe(punch.ObserverFunc(func(payload []byte) {
    // whatever you put in is what you get out
}))
session.Send([]byte{...})
```

| Package | What it gives you |
|---|---|
| `pkg/punch` | The path, the encryption, the mesh. Bytes in, bytes out. |
| `pkg/stun` | Your public address, and a classification of your NAT. |
| `pkg/upnp`, `pkg/pcp` | Asking a router to open a door, in three protocols. |
| `pkg/dht` | Announce and find an address on the BitTorrent DHT. |
| `pkg/diag` | Whether two networks can reach each other, and what to do. |
| `pkg/names` | A key turned into a name a person can say out loud. |
| `pkg/chat` | Lines, typing and speakers — the layer this program's UI uses. |

**→ [ARCHITECTURE.md](ARCHITECTURE.md) walks the whole thing**: how a path is
opened step by step, what the wire looks like, how the mesh keys itself, and a
recipe for building on it. Diagrams, not prose.

[`examples/filedrop`](examples/filedrop) is the proof it is a real separation:
it moves a file between two machines with no chat, no nicknames and no terminal,
and nothing in the transport had to change to allow it.

---

## Build it yourself

The shortest answer to "should I trust this binary":

```bash
git clone https://github.com/MalPr0/vapora && cd vapora
go build ./cmd/vapora
go test ./... -race
```

Go 1.25. Nothing to fetch, nothing to configure.

Every exported declaration in `pkg/` is documented, and the check is in the
repo: `go run ./internal/doclint pkg`.

**Layout, if you want to read it.** `pkg/punch` is the handshake, sessions and
rooms. `pkg/stun` learns your address and classifies your NAT. `pkg/upnp` and
`pkg/pcp` ask routers to open doors. `pkg/dht` is the BitTorrent client.
`pkg/diag` is the reasoning behind the advice. `internal/tui` is the pixel-art
chat.

[`ARCHITECTURE.md`](ARCHITECTURE.md) is the guided tour of all of it.
[`AGENTS.md`](AGENTS.md) documents the invariants — the things that look like
details and turn out to be load-bearing.

---

<sup>MIT licensed. Built in the open, one commit at a time.</sup>
