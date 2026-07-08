---
title: cocode
emoji: 👥
colorFrom: indigo
colorTo: purple
sdk: docker
app_port: 8000
pinned: false
license: mit
---

# cocode

> A collaborative real-time code editor, built in Go with a hand-implemented
> CRDT for conflict-free concurrent editing.

**▶ Live demo: https://shreyansh112-cocode.hf.space** — open it in two tabs (add
`#myroom` to both URLs to share a room) and type; edits and colored cursors sync
live. It's a free, sleeping Space, so the first load may take ~30s to wake.

Multiple people edit the same document simultaneously; edits converge without
conflicts thanks to a **causal-tree / RGA sequence CRDT** implemented from
scratch (no CRDT libraries). It ships with WebSocket rooms and presence, plus
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
- ✅ Web frontend with a JS port of the CRDT; live multi-client sync
- ✅ Verified end-to-end: two clients type concurrently and converge
- ✅ Git-style snapshots / history (content-addressed, deduplicated blobs)
- ✅ Presence registry: each collaborator gets a name/color + cursor tracking
- ✅ Version diff (LCS) and language detection, exposed over an HTTP API
- ✅ CodeMirror 6 editor: **rendered remote cursors** (colored, named) + syntax
  highlighting for 9+ languages (auto-detected, with a manual selector)
- ✅ Deployed live on Hugging Face Spaces (Docker); WebSockets verified through
  the proxy (two-client presence relay + `wss://` handshake)
- ⏳ Sandboxed code execution ("Run")

## Editor bundle (build once, committed)

The CodeMirror editor is bundled with esbuild into `web/vendor/cocode-editor.js`,
which **is committed**, so the app runs with no build step. To change the editor,
rebuild the bundle:

```bash
cd web-src
npm install
npm run build      # -> web/vendor/cocode-editor.js
```

## HTTP API

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/ws?room=<id>` | WebSocket for real-time collaboration |
| `POST` | `/api/snapshot?room=<id>&message=<m>` | Save the room's current text as a version |
| `GET` | `/api/versions?room=<id>` | List a room's saved versions |
| `GET` | `/api/version?room=<id>&id=<vid>` | Fetch one version's content |
| `GET` | `/api/diff?room=<id>&a=<vid>&b=<vid>` | Unified line diff between two versions |
| `GET` | `/api/lang?filename=<f>&content=<c>` | Detect the programming language |

## Run it

```bash
go run ./cmd/server          # serves http://localhost:8090
```

Open `http://localhost:8090` in two browser tabs and type — edits sync live.
Use a URL hash to pick a room, e.g. `http://localhost:8090/#myroom`.

## Build & test

```bash
go test ./...                # all packages
```

## Deploy

Deployed as a Docker container (`Dockerfile` builds a static, CGO-free binary on
`alpine`). The server reads `PORT` and `DATA_DIR` from the environment and the
frontend picks `ws://`/`wss://` automatically, so it runs unchanged behind a
TLS-terminating proxy.

It's live on Hugging Face Spaces (Docker SDK, `app_port: 8000` in the README
front matter). To redeploy: `git push hf main`. The same image runs on any
Docker host (Render, Fly.io, Cloud Run, …).

## Architecture

```
Browser (CodeMirror 6 + web/crdt.js)  ⇄  WebSocket  ⇄  Go hub (rooms, presence)
        client-side CRDT replica                        server canonical replica
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
