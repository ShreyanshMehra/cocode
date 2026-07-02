# cocode

> A collaborative real-time code editor, built in Go with a hand-implemented
> CRDT for conflict-free concurrent editing.

Multiple people edit the same document simultaneously; edits converge without
conflicts thanks to a **causal-tree / RGA sequence CRDT** implemented from
scratch (no CRDT libraries). Planned: WebSocket rooms with presence, and
git-style content-addressed snapshots for versioning (reusing ideas from
[gitfromscratch](https://github.com/ShreyanshMehra/gitfromscratch)).

## Why a CRDT?

When two people type at the same position at the same time, a naive editor
corrupts or drops text. A CRDT (Conflict-free Replicated Data Type) guarantees
every replica converges to the same document regardless of the order operations
arrive — no central lock, no operational-transform server round-trips.

This project implements the **causal tree** model (equivalent to RGA):

- Every character is an atom with a unique ID and a parent (the atom it was
  inserted after). Atoms form a tree rooted at a virtual root.
- The visible document is a pre-order DFS of the tree, with siblings ordered by
  descending ID (so concurrent inserts at the same spot order deterministically).
- Deletes are tombstones. Operations are idempotent and commutative, and
  out-of-order delivery is handled with a pending buffer.

See `internal/crdt/` — the convergence properties are proven by tests
(concurrent inserts, out-of-order delivery, and 50 random-shuffle commutativity
trials).

## Status

🚧 Working MVP — collaborative editing over WebSockets, end-to-end.

- ✅ CRDT engine (RGA / causal tree) in Go — 11 tests
- ✅ WebSocket hub: rooms, op relay, late-joiner sync, presence — integration tests
- ✅ Web frontend (textarea) with a JS port of the CRDT; live multi-client sync
- ✅ Verified end-to-end: two clients type concurrently and converge
- ⏳ Git-style snapshots / history (reuse content-addressed objects)
- ⏳ Richer editor (CodeMirror/Monaco), live cursors
- ⏳ Deploy

## Run it

```bash
go run ./cmd/server          # serves http://localhost:8090
```

Open `http://localhost:8090` in two browser tabs and type — edits sync live.
Use a URL hash to pick a room, e.g. `http://localhost:8090/#myroom`.

## Build & test

```bash
go test ./...                # Go: CRDT + hub
```

## Architecture

```
Browser (textarea + web/crdt.js)  ⇄  WebSocket  ⇄  Go hub (rooms, presence)
        client-side CRDT replica                     server canonical replica
                                                      + ordered op log
```

Both client and server run the **same** CRDT, so the server only relays ops
(and snapshots the log for late joiners). The wire format is pinned by tests so
the Go and JavaScript implementations stay interoperable.

## Tech

- Go (standard library + gorilla/websocket)
- Causal-tree / RGA sequence CRDT (hand-implemented in both Go and JS)
- Dependency-free web frontend (no build step)

## License

[MIT](LICENSE)
