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

🚧 In development.

- ✅ CRDT engine (RGA / causal tree) — 9 tests
- ⏳ WebSocket hub: rooms, op relay, late-joiner sync, presence
- ⏳ Git-style snapshots / history
- ⏳ Web frontend
- ⏳ Deploy

## Build & test

```bash
go test ./...
```

## Tech

- Go (standard library; WebSocket transport TBD)
- Causal-tree / RGA sequence CRDT (hand-implemented)

## License

[MIT](LICENSE)
