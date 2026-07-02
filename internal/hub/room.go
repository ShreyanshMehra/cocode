// Package hub is the real-time transport: it groups WebSocket clients into
// rooms (one per document) and relays CRDT operations between them.
//
// The server keeps a canonical replica and an ordered operation log per room.
// A client that joins receives the full log ("init") and replays it to rebuild
// the current document; subsequent operations are broadcast as they arrive.
// Because the payloads are CRDT ops, ordering across clients does not need to be
// globally consistent for correctness — the CRDT converges regardless — but a
// per-room goroutine serialises mutations so the server's own replica and log
// stay race-free.
package hub

import (
	"github.com/ShreyanshMehra/cocode/internal/crdt"
)

// MsgType enumerates the wire message kinds.
type MsgType string

const (
	// MsgOp carries a single CRDT operation (client<->server).
	MsgOp MsgType = "op"
	// MsgInit is sent to a joining client with the full op log to replay.
	MsgInit MsgType = "init"
	// MsgPresence announces the current participant count in the room.
	MsgPresence MsgType = "presence"
)

// Message is the JSON envelope exchanged over the WebSocket.
type Message struct {
	Type  MsgType   `json:"type"`
	Op    *crdt.Op  `json:"op,omitempty"`
	Ops   []crdt.Op `json:"ops,omitempty"`
	Count int       `json:"count,omitempty"`
}

// inbound is an operation received from a specific client.
type inbound struct {
	from *Client
	op   crdt.Op
}

// Room is a single collaborative document and the clients editing it.
type Room struct {
	id    string
	doc   *crdt.Doc // server-side canonical replica (for late-joiner snapshots)
	oplog []crdt.Op // ordered log of every applied op

	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	incoming   chan inbound
}

func newRoom(id string) *Room {
	return &Room{
		id:         id,
		doc:        crdt.New(0), // server replica never authors ops
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		incoming:   make(chan inbound, 64),
	}
}

// run owns all room state; it is the only goroutine that mutates the room.
func (r *Room) run() {
	for {
		select {
		case c := <-r.register:
			r.clients[c] = true
			// Send the joining client the full op log to rebuild state.
			snapshot := make([]crdt.Op, len(r.oplog))
			copy(snapshot, r.oplog)
			c.trySend(Message{Type: MsgInit, Ops: snapshot})
			r.broadcastPresence()

		case c := <-r.unregister:
			if _, ok := r.clients[c]; ok {
				delete(r.clients, c)
				close(c.send)
				r.broadcastPresence()
			}

		case in := <-r.incoming:
			r.doc.Apply(in.op)
			r.oplog = append(r.oplog, in.op)
			// Relay to every other client in the room.
			for c := range r.clients {
				if c != in.from {
					c.trySend(Message{Type: MsgOp, Op: &in.op})
				}
			}
		}
	}
}

func (r *Room) broadcastPresence() {
	msg := Message{Type: MsgPresence, Count: len(r.clients)}
	for c := range r.clients {
		c.trySend(msg)
	}
}

// Value returns the room's current canonical document text (for tests/snapshots).
func (r *Room) Value() string { return r.doc.Value() }
