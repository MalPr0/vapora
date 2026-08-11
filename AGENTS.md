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
| `pkg/dht` | Bencode and a mainline DHT client: announce and lookup, no serving |
| `internal/chat` | Demo TCP chat for the UPnP path |

`scripts/mesh-check.exp` drives three real rooms through pseudo-terminals with
`expect`: keyboard in, screen out, chained invites. It is the only check that
covers the keyboard and the full screen together, and it needs built binaries,
so it is not part of `go test`.

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
- **Liveness is a poll, not an event.** Nothing arrives to announce that
  nothing is arriving, so `Session.Health` computes from a timestamp rather
  than firing a callback. Any authenticated frame refreshes it.
- **`punch.Mux` is the only reader of a socket.** A session never reads: it is
  handed datagrams through `Deliver`. Writes are safe from any goroutine, reads
  are not. This is what dissolved the old trap of starving a lookup between
  `Open` and `Run`; there is no longer a gap where nothing dispatches.
- **A sink must not block, and must not keep the payload.** The buffer is
  reused as soon as `Deliver` returns, and every sink shares one reader.
- **Never dispatch with the route lock held.** A sink calls `Route`/`Unroute`
  from inside `Deliver`, so the mux copies what it needs and releases first.
- **Holding the secret does not make you the peer.** Everyone handed the same
  invite seals under the same key, so authentication alone cannot tell the peer
  from a third party. `Opened.Sender` is the nonce prefix, drawn per codec, so
  it names the process: a move is only followed when it matches the sender
  already established. You cannot be replaced once you have spoken.
- **An authenticated frame from a stranger is louder than a scanner.** Junk
  never authenticates, so `Probes.Impostors` means the invite is in more hands
  than one. Report the two differently.
- **Anything reachable from the internet authenticates.** There is no
  unauthenticated mode and `plainCodec` is unexported for that reason; it
  exists so tests can exercise framing on its own.
- **Control frames are padded.** AEAD hides content, not length. A frame whose
  only content is its kind would have a length nobody else produces, on a
  cadence that names it a heartbeat.
- **Never print network bytes raw.** Route them through `text.Safe` first, and
  note the invariant it carries: everything `Safe` produces satisfies `Valid`.
  Break that and a line is sanitised on the way out and rejected on the way in.
- **Only text crosses the channel.** Sanitise outgoing, validate incoming, drop
  what fails. A peer sending non-text is not this program.
- **Never answer what did not authenticate.** Silence is what makes probing
  this port indistinguishable from probing a closed one. Count it instead.
- **Cancelling a context does not interrupt a blocked read.** A loop that only
  checks `ctx.Err()` after a read error never stops while a peer keeps talking.
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
- **The header is not a fixed height.** A room lists everyone present, so the
  chat area has to ask `headerRows(state, height)`; anything that measures
  against a constant drops a line or paints over one as soon as a third person
  joins. `minHeight` exists to keep that arithmetic solvable.
- **Never truncate an invite on screen.** It is 95 characters, wider than a
  terminal, and a cut invite looks copyable and decodes to nothing. The view
  wraps it and `joinToken` puts it back together on paste — a wrapped invite
  arriving by chat is the normal case, not the edge case.
- **Nothing the DHT returns is trusted.** That network has nodes answering every
  key with addresses nobody announced, including ones that mirror the port just
  announced — both were seen while testing this. An address from there is a
  place to try, bounded and deduplicated, never a participant.
- **The DHT client answers no queries.** Serving strangers from the socket the
  conversation runs on is a much larger surface for nothing this needs, so a
  query addressed to it falls through as somebody else's problem.
- **An announcement nobody accepted is not an announcement.** Nodes answering is
  not the same as nodes taking the announcement, and reporting the first as
  success is how a rendezvous appears to work and never meets anyone.
- **A room only ever answers a hello.** Nothing is sent to an address the room
  has not heard from, so between two networks that both refuse a first packet
  from a stranger the newcomer's hello dies at the host's door and waiting
  longer never helps. `Room.Reach` is the way out and it is the room's version
  of the second invite `punch` sends: it carries no secret and grants nothing,
  it only makes this side start sending so its own router opens.
- **The gutter is a promise.** A nickname reaches thirty characters and the
  gutter is capped at twenty and at a third of the screen, so `fitName` cuts
  rather than pads: `%-*s` widens for a long name, which shifts that speaker's
  text right and pushes its tail off the edge, where `Screen.Text` drops it
  without a word.
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
