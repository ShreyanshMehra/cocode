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
	"github.com/ShreyanshMehra/cocode/internal/participant"
)

// MsgType enumerates the wire message kinds.
type MsgType string

const (
	// MsgOp carries a single CRDT operation (client<->server).
	MsgOp MsgType = "op"
	// MsgInit is sent to a joining client with the full op log to replay.
	MsgInit MsgType = "init"
	// MsgPresence announces the current participants in the room.
	MsgPresence MsgType = "presence"
	// MsgCursor updates the sender's caret position (client->server).
	MsgCursor MsgType = "cursor"
	// MsgWelcome tells a joining client its own assigned identity.
	MsgWelcome MsgType = "welcome"
)

// Message is the JSON envelope exchanged over the WebSocket.
type Message struct {
	Type         MsgType                   `json:"type"`
	Op           *crdt.Op                  `json:"op,omitempty"`
	Ops          []crdt.Op                 `json:"ops,omitempty"`
	Count        int                       `json:"count,omitempty"`
	Cursor       int                       `json:"cursor,omitempty"`
	Self         *participant.Participant  `json:"self,omitempty"`
	Participants []participant.Participant `json:"participants,omitempty"`
}

// inbound is an operation received from a specific client.
type inbound struct {
	from *Client
	op   crdt.Op
}

// cursorInbound is a caret-position update from a specific client.
type cursorInbound struct {
	from *Client
	pos  int
}

// Room is a single collaborative document and the clients editing it.
type Room struct {
	id    string
	doc   *crdt.Doc // server-side canonical replica (for late-joiner snapshots)
	oplog []crdt.Op // ordered log of every applied op
	parts *participant.Registry

	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	incoming   chan inbound
	cursor     chan cursorInbound
}

func newRoom(id string) *Room {
	return &Room{
		id:         id,
		doc:        crdt.New(0), // server replica never authors ops
		parts:      participant.NewRegistry(),
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		incoming:   make(chan inbound, 64),
		cursor:     make(chan cursorInbound, 64),
	}
}

// run owns all room state; it is the only goroutine that mutates the room.
func (r *Room) run() {
	for {
		select {
		case c := <-r.register:
			r.clients[c] = true
			self := r.parts.Join(c.id)
			// Send the joining client the full op log to rebuild state,
			// followed by its own assigned identity.
			snapshot := make([]crdt.Op, len(r.oplog))
			copy(snapshot, r.oplog)
			c.trySend(Message{Type: MsgInit, Ops: snapshot})
			c.trySend(Message{Type: MsgWelcome, Self: &self})
			r.broadcastPresence()

		case c := <-r.unregister:
			if _, ok := r.clients[c]; ok {
				delete(r.clients, c)
				r.parts.Leave(c.id)
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

		case cu := <-r.cursor:
			r.parts.SetCursor(cu.from.id, cu.pos)
			r.broadcastPresence()
		}
	}
}

func (r *Room) broadcastPresence() {
	parts := r.parts.List()
	msg := Message{Type: MsgPresence, Count: len(r.clients), Participants: parts}
	for c := range r.clients {
		c.trySend(msg)
	}
}

// Value returns the room's current canonical document text (for tests/snapshots).
func (r *Room) Value() string { return r.doc.Value() }
