# vapora

A base tool to open a direct communication channel between two peers across the
internet, from one shared link, with no account, no registration and no server
of your own. The chat that ships with it is the test harness, not the product:
it exists to prove the channel carries traffic.

Everything is standard library. SSDP, SOAP, STUN, PCP, NAT-PMP and the
authenticated wire format are implemented here, with no external dependencies.

## Quick start

```bash
go run ./cmd/vapora nat      # classify this network
go run ./cmd/vapora diag     # find out which of your routers filters, and how
go run ./cmd/vapora punch    # print an invite and wait for a peer
```

## Install

Every merge to `main` publishes a release with static binaries for macOS, Linux
and Windows. Both peers need one, so a link to the release is usually easier
than asking the other side to install Go.

**Download with `curl`, not with a browser.** On macOS a browser marks whatever
it downloads with `com.apple.quarantine`, and Gatekeeper then refuses to run a
binary that Apple has not notarized. `curl` sets no such mark, so the binary
just runs.

```bash
# macOS, Apple Silicon
curl -fsSL https://github.com/MalPr0/vapora/releases/latest/download/vapora_darwin_arm64.tar.gz | tar -xz

# macOS, Intel
curl -fsSL https://github.com/MalPr0/vapora/releases/latest/download/vapora_darwin_amd64.tar.gz | tar -xz

# Linux, x86-64
curl -fsSL https://github.com/MalPr0/vapora/releases/latest/download/vapora_linux_amd64.tar.gz | tar -xz

./vapora version
```

On Windows, download `vapora_windows_amd64.zip` from the release page and
extract it.

### Verify what you downloaded

This tool opens a port and talks to strangers, so check the binary before
running one you did not build. `SHA256SUMS` ships with every release.

```bash
curl -fsSLO https://github.com/MalPr0/vapora/releases/latest/download/vapora_darwin_arm64.tar.gz
curl -fsSLO https://github.com/MalPr0/vapora/releases/latest/download/SHA256SUMS

shasum -a 256 -c SHA256SUMS --ignore-missing   # macOS
sha256sum -c SHA256SUMS --ignore-missing       # Linux
```

### If you did download through a browser

macOS will refuse to run it, with a dialog saying Apple could not verify the
binary is free of malware. That is accurate: these releases are signed only
ad-hoc, not with an Apple Developer ID, so Apple has verified nothing. Strip the
quarantine mark yourself, but only after checking the checksum above:

```bash
xattr -d com.apple.quarantine ./vapora
```

Re-downloading with `curl` avoids the whole thing.

### Or build it

Needs nothing but a Go toolchain, and answers the trust question by not asking
it:

```bash
go build ./cmd/vapora
```

## How the channel opens

One side prints an invite that is itself a runnable command:

```
$ go run ./cmd/vapora punch

send this to your friend, it is a runnable command:

    vapora punch 203.0.113.7:41001/BXFWOBXKGS547XF2WOKVG6JYDI

waiting for them...
```

The other side runs exactly that line and both ends punch towards each other.
There is no server and no host/client role: whoever starts first retries until
the other shows up. While waiting, the socket keeps its NAT binding warm, since
an idle mapping expires long before a person reads a message and reacts.

### The chat

Once the path opens, `punch` takes over the terminal with a full screen chat:
a pixel art console drawn with half block characters and a fixed palette, the
conversation in the middle, and an input line that renders itself.

Each side is named after an animal, and both names are **derived from the shared
secret** rather than chosen or sent. Every peer computes the same pair, so
nobody can decide how they are labelled on the other's screen and no name has to
be trusted over the wire.

While the other side is writing, a courier runs in the footer and the line reads
`BADGER is typing`. That state travels as its own encrypted frame, sent on the
first keystroke and withdrawn when the line is sent or two seconds pass with no
typing.

### Knowing the path is still there

UDP has no connection, so there is nothing to close and no close to be told
about. A peer that walks away, loses wifi or gets killed looks exactly like a
peer with nothing to say.

So the session sends a ping every five seconds and the other side answers with a
pong. Neither ever reaches the chat: a healthy path stays invisible, and the
probe exists only to make silence measurable. Any frame that authenticates
counts as proof of life, so a talkative peer is never probed for nothing.

After twelve seconds of silence the header shows `no reply 14s`; after
forty-five it blinks `LINK LOST`. Probing continues either way, because a path
that healed on its own should come back without anyone restarting anything. When
it is healthy the header carries the round trip instead, which is the only thing
the probe is otherwise good for.

The five second interval is not a detection tuning: it is below the shortest NAT
binding timeouts seen in the wild, so the same packet that measures the path
also keeps it open.

### Coming back after a break

Three things can break an open path, and they are not equally recoverable.

**The routers forgot.** A laptop lid, an idle stretch, a NAT dropping state:
both endpoints are unchanged, only the pinholes are gone. The moment the path
goes quiet the ping cadence tightens and punches start again, so this heals with
nobody doing anything.

**Your friend moved.** New wifi, cellular handover, a restart on another port.
Their address is new and nothing announced it. If a frame arrives from somewhere
else and it authenticates, the path follows it. That is safe for the same reason
the invite is: only the holder of the secret can produce such a frame, so
following one trusts exactly what was already trusted. A healthy conversation
never follows, which keeps two live paths from chasing each other.

**You moved.** Now the address on the invite you shared has stopped existing and
your friend is talking to nobody. A STUN watcher shares the socket with the
session and notices, and the chat hands you a fresh invite to send. This one
cannot be automatic: telling them the new address needs a channel that still
works, which is the same gap a rendezvous would fill.

Typing `!exit` ends the session at once. It is checked before a line becomes a
message, so it never reaches the peer as text, and it says goodbye on the way
out: quitting on purpose is otherwise indistinguishable from a network that went
quiet, and the other side would wait out the whole silence budget to find out.

The conversation stacks upward from the input line and keeps its whole history:
`pgup` and `pgdn` walk it, a marker shows how much is hidden above and below,
and sending a line snaps back to the newest one.

The UI needs a terminal. Piped input, a CI job or a terminal that refuses raw
mode all fall back to plain lines automatically, and `-plain` forces it.

### The secret in the invite

The waiting socket accepts packets from any source, which is what lets an
unannounced peer in. The random secret appended to the endpoint is what keeps
that from being an open door: it is the session key, and a frame that does not
authenticate under it is dropped without ever becoming the peer.

It is not a password compared on arrival. Two AES-256-GCM keys are derived from
it with HKDF-SHA256, one per direction, so every frame is encrypted and
authenticated, no nonce is ever reused under a key, and replays are rejected by
an IPsec style sliding window. The secret is 128 random bits from `crypto/rand`;
being full entropy already, it needs a KDF, not password stretching.

**The invite is a credential.** It belongs on a channel you trust, and both
sides must carry the same one or they never see each other. There is no
unauthenticated mode: a socket reachable from the internet has no business
running without the AEAD, and the secret costs nothing.

Control frames carry random filler. AEAD hides what a frame says but not how
long it is, and a ping with nothing to carry would be a fixed size arriving on a
fixed cadence, which is a heartbeat visible to anyone counting bytes. Padding
puts it inside the same size spread as the messages it travels among.

### Only text crosses the channel

Outgoing lines are sanitised, so what leaves is always text. Incoming ones are
**validated**, and a frame carrying anything else is dropped rather than cleaned
up and shown: a peer sending non-text is not this program, and treating it as a
rendering problem would be papering over that.

### What an attacker can and cannot learn

Traffic that fails to authenticate is **never answered**. A scanner sweeping this
port gets silence, which is the same thing it gets from a closed port, so
probing confirms nothing.

It is counted, though. An address that only ever appeared on one invite should
not be hearing from anybody else, so the session reports it: `6 packets from 1
address(es) reached this port and could not authenticate`. That is not an attack
in progress, it is a signal that the invite is more public than it was meant to
be, and starting a new session gives a new secret and usually a new port.

What no amount of code hides: **your peer sees your IP**, because the packets go
from your address to theirs; **so do the STUN servers**, because asking what your
address is means asking somebody; and **so does whoever sees the invite**, since
it carries the address. Those are properties of a direct channel, not defects to
fix, and the only way around them is relaying every packet through a third
party.

### What anonymous means here

No accounts, no registration, and no server that learns who talks to whom.

It does **not** mean your peer cannot see you: on a direct channel the packets
travel from your address to theirs, so each end learns the other's IP. That is
what direct means. Hiding that would require relaying every packet through a
third party, which is the opposite of what this tool does.

## The UPnP path

`serve` maps a port and hosts a chat on it; `connect` joins. That port is
reachable from the internet, so the chat is authenticated and encrypted with the
same session key mechanism the punch channel uses, and `serve` prints an invite
exactly like `punch` does.

It hosts one conversation at a time. That is not a limitation to work around:
every peer would otherwise share one direction key, and two of them could
collide on a nonce under it.

## Whether one link is enough

That depends on the **filtering** behaviour of the waiting side, which `nat`
reports and `diag` attributes to a specific router.

- `endpoint-independent (full cone)` — the peer's first packet gets in and the
  invite alone connects the two.
- anything stricter — that packet is dropped, because the waiting side never
  contacted that endpoint. The joining side also prints its own invite; pasting
  it back into the waiting terminal completes the handshake. Pasting works at
  any moment while it waits, and the whole line can be pasted as is.

`diag` exists because a STUN report only describes the whole chain end to end.
Behind two cascaded NATs it cannot say which one is restrictive, so `diag` runs
an experiment instead of a measurement: it installs a UPnP mapping for one
socket, re-measures it against an unmapped control socket, and checks five
confounders before reaching a verdict.

The mapping matters because the UPnP-IGD spec (WANIPConnection v2, §2.3.17)
gives a wildcard `RemoteHost` mapping endpoint-independent filtering. If the
filtering opens for the mapped socket and not for the control, the router that
speaks UPnP was the one dropping packets, and it can be told to stop. If nothing
changes, the restriction lives upstream, out of reach.

## Port control protocols

`diag` probes each gateway for all three mechanisms, because a router that
refuses one often answers another:

- **UPnP-IGD** over SSDP and SOAP, including cascaded double NAT traversal.
- **PCP** (RFC 6887). A `MAP` without a FILTER option is endpoint-independent by
  the spec's own wording, which is stronger than UPnP's.
- **NAT-PMP** (RFC 6886), detected from the version 0 rejection a legacy gateway
  gives to a PCP request.

## Layout

```
cmd/vapora      CLI: nat, diag, punch, probe, serve, connect
pkg/punch       hole punching, invite secrets, the authenticated wire format
pkg/stun        STUN (RFC 5389) and NAT behaviour discovery (RFC 5780)
pkg/pcp         PCP (RFC 6887) and NAT-PMP (RFC 6886)
pkg/upnp        SSDP, device description, SOAP, port mappings, NAT chain
pkg/diag        the differential experiment that attributes filtering
pkg/text        sanitises what arrives from the network before a terminal sees it
internal/tui    the pixel art chat: renderer, sprites, key decoding, raw mode
internal/chat   the TCP chat used as the traffic probe for the UPnP path
```

Everything under `pkg/` is importable and is where the contract lives.
`internal/chat` is demo scaffolding and carries no promises.

## Development

```bash
go test ./... -race
go vet ./...
```
