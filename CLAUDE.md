# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
go build ./...                          # everything, including examples
go test ./... -race -count=1            # the suite CI runs
go test ./pkg/punch/ -run TestName -v   # one test
gofmt -l .                              # must print nothing; CI fails otherwise

go run ./internal/doclint pkg           # every exported declaration in pkg/ has a doc comment
go run ./internal/linklint              # translated pages: links resolve and stay in their language
```

`doclint` and `linklint` run in CI alongside the tests. Both exit non-zero on
failure, so they are worth running before pushing.

### Integration drivers

`scripts/` holds checks that drive real processes through pseudo-terminals with
`expect`. They are not part of `go test` because they need built binaries in the
working directory:

```bash
go build -o ./vapora ./cmd/vapora && ./scripts/mesh-check.exp
```

| | |
|---|---|
| `mesh-check.exp` | three rooms, chained invites, C reaching A directly |
| `punch-check.exp` | the two-way chat |
| `paste-check.exp` | the address paste-back both routers need |
| `interrupt-check.exp` | one ctrl+c ends a plain session without pressing enter |
| `quorum-check.sh` | a room closes when it empties, `-standalone` does not |
| `pong-check.exp` | needs `go build -o ./pong ./examples/pong` |

## Architecture

Two layers, and the split is load-bearing rather than tidy.

**`pkg/punch` is transport.** It opens an encrypted path through two NATs, keeps
a mesh of pair channels alive, and moves **bytes**. It has no idea what a
conversation is. Its entire application surface is one frame kind, `AppKind`
(0x40): everything below that number belongs to the transport, and everything a
caller sends rides opaquely above it. A caller needing several message types
tags them *inside* the payload, so the two numbering spaces cannot collide.

**`pkg/chat` is one caller.** Lines, typing indicators, speakers. `pkg/names`
turns a public key into something a person can say. Neither is imported by the
transport — `punch.Member` deliberately has no `Name` field.

Three programs exercise the same channel in opposite ways, which is what keeps
the split honest: the chat sends **events** (every one matters),
`examples/filedrop` sends **blocks**, and `examples/pong` sends **state** thirty
times a second (only the newest matters). None of them required a transport
change.

### Things that need several files to see

- **`punch.Mux` is the only thing that reads a socket.** Sessions are handed
  datagrams and never read. That is what lets one socket — and therefore one NAT
  binding and one keepalive — carry STUN, every peer in a room, and a DHT client
  at once. Unrouted datagrams walk a fallback chain in registration order.
- **A room is every pair at once, not a session with more peers.** Two key
  layers: the room key (from the invite secret) seals only `hello`/`full`; a
  pair key (X25519 between those two, salted with the room secret) seals
  everything else. A third member holding the invite cannot read or forge what
  two others say — arithmetic, not a promise.
- **Every member announces two addresses**, public and local, because two
  machines behind the same router cannot reach each other through the public one.
- **`cmd/vapora`** has two front ends per command: full-screen (`internal/tui`)
  and `-plain`. Plain mode is what to use when debugging, since the full-screen
  one erases itself on exit.

## Ground rules

- **Standard library only.** No external dependencies, ever — including in
  examples. Every protocol here is implemented from its RFC.
- **Code, comments and documentation in English.** Conversation with the owner
  is in Spanish.
- **No real addresses in the repository.** Use RFC 5737 documentation ranges
  (`203.0.113.0/24`, `198.51.100.0/24`, `192.0.2.0/24`).
- **`examples/apitour` compiles every snippet in `ARCHITECTURE.md`.** Change a
  signature and the build catches the stale page.
- **Translations live in `docs/<lang>/`.** The README is translated into ten
  languages; `ARCHITECTURE.md` and the Pong tutorial into Spanish only. A
  selector must offer exactly what exists — `linklint` enforces both directions.

**[`AGENTS.md`](AGENTS.md) is the important file to read before editing.** It
documents ~40 invariants that look like details and turn out to be load-bearing:
why a sink must not block, why control frames are padded, why nothing waits on
stdin directly, why an authenticated frame from a stranger is worth counting.
Most were written after something broke.

## graphify

This project has a knowledge graph at graphify-out/ with god nodes, community structure, and cross-file relationships.

Rules:
- For codebase questions, first run `graphify query "<question>"` when graphify-out/graph.json exists. Use `graphify path "<A>" "<B>"` for relationships and `graphify explain "<concept>"` for focused concepts. These return a scoped subgraph, usually much smaller than GRAPH_REPORT.md or raw grep output.
- If graphify-out/wiki/index.md exists, use it for broad navigation instead of raw source browsing.
- Read graphify-out/GRAPH_REPORT.md only for broad architecture review or when query/path/explain do not surface enough context.
- After modifying code, run `graphify update .` to keep the graph current (AST-only, no API cost).
