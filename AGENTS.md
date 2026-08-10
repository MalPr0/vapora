# AGENTS.md

Entry point for agents working on vapora. Read this, then the `README.md`, then
the package you are about to touch.

## What this project is

A base tool that opens a direct, authenticated UDP channel between two peers
from a single shared link, with no account and no server. The chat under
`internal/chat` is a test harness that proves the channel carries traffic; it is
not the product and carries no compatibility promises.

## Ground rules

- **Standard library only.** No external dependencies, ever. Every protocol here
  (SSDP, SOAP, STUN, PCP, NAT-PMP, the AEAD wire format) is implemented against
  `crypto/*` and `net/*`. A change that adds a dependency needs a decision from
  the owner first.
- **Code and documentation in English.** Conversation with the owner is in
  Spanish; nothing in the repository is.
- Files stay at 200–300 lines, 500 absolute maximum.
- Dependencies enter through interfaces declared on the consumer side. `pkg/diag`
  is the reference: it defines `PortMapper` and `FilterProbe` itself so the
  experiment runs against fakes with no network.
- Errors wrap with `fmt.Errorf("...: %w", err)`, sentinel errors for the cases a
  caller branches on, no panics in normal flow.
- No narrative logging. A message is worth printing when it changes what the
  user does next.

## Where things live

| Path | Role |
|---|---|
| `cmd/vapora` | CLI. Assembles packages, owns all user-facing output |
| `pkg/punch` | Hole punching handshake, session, invites, `Codec`, replay window |
| `pkg/stun` | STUN client, RFC 5780 mapping and filtering discovery, keepalive |
| `pkg/pcp` | PCP and NAT-PMP client with version detection |
| `pkg/upnp` | SSDP discovery, device description, SOAP, mappings, NAT chain |
| `pkg/diag` | The differential experiment attributing filtering to a router |
| `pkg/text` | Sanitiser for anything from the network headed to a terminal |
| `internal/tui` | The pixel art chat: screen, sprites, key decoding, raw mode |
| `internal/chat` | Demo TCP chat for the UPnP path |

`pkg/` is the shared contract: high coverage is expected there, and a change to
a signature is a change to the contract.

## Invariants worth knowing before editing

- **One reader per UDP socket.** `punch.Session` owns the read loop of its
  socket. `stun.Keepalive` deliberately only writes, because a second reader on
  the same `net.UDPConn` steals its datagrams. Anything new that needs to read
  that socket has to go through a demultiplexer, not a second `ReadFrom`.
- **The codec is the only arbiter of who the peer is.** `Session.Open` accepts
  datagrams from any source and adopts the sender only when the frame
  authenticates. Endpoint discovery may propose candidates; it never decides.
- **Never print network bytes raw.** Route them through `text.Safe` first.
- **Publish the endpoint STUN observed, never the one requested.** Behind double
  NAT the external port of a UPnP mapping lives on a private WAN address and is
  not what a peer on the internet dials.
- **A mapping lease is not a pinhole.** The UPnP lease governs the inner router;
  the outermost NAT expires its binding on inactivity, which is why the waiting
  side has to keep sending.
- **The UI renders to a buffer, never straight to a terminal.** `tui.Draw`
  takes a `State` and fills a `Screen`; only `Screen.Flush` touches the tty.
  That split is what lets frames be asserted on in tests, and it is the reason
  every layout bug so far was caught by a test rather than by eye.
- **Pixel art is drawn with half blocks, never whole ones.** A terminal cell is
  about twice as tall as it is wide, so `█` is a rectangle; `▀` with a
  foreground and a background is a square pixel. The first wordmark was
  unreadable for exactly this reason.
- **Nothing but the UI writes to the terminal while it runs.** A `fmt.Print`
  buried in a package lands in the middle of whatever was drawn. Report through
  the observer instead.
- **Ranging a string steps by bytes.** Every block glyph the sprites use is
  three of them, so pixel art must range `[]rune(line)`.
- **No real addresses in the repository.** Use RFC 5737 documentation ranges
  (`203.0.113.0/24`, `192.0.2.0/24`) for public addresses and generic RFC 1918
  ranges for private ones.

## Testing

`go test ./... -race` and `go vet ./...` have to pass before anything ships.

Tests do not touch the real network. The patterns already in the tree: fake UDP
servers on loopback that answer canned bytes (`pkg/pcp`, `pkg/stun`), scripted
fakes behind consumer interfaces (`pkg/diag`), and two real sockets talking to
each other over loopback (`pkg/punch`).

Verification against real hardware happens through the CLI (`vapora diag`), not
through tests.
