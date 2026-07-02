package hub

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ShreyanshMehra/cocode/internal/crdt"
	"github.com/gorilla/websocket"
)

// dialConn opens a websocket client to the test server's /ws for a room.
func dialConn(t *testing.T, srv *httptest.Server, room string) *websocket.Conn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws?room=" + room
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return conn
}

// readUntil reads messages until one of type t is seen or the deadline passes.
func readUntil(t *testing.T, c *websocket.Conn, want MsgType) Message {
	t.Helper()
	c.SetReadDeadline(time.Now().Add(2 * time.Second))
	for {
		var m Message
		if err := c.ReadJSON(&m); err != nil {
			t.Fatalf("read (waiting for %s): %v", want, err)
		}
		if m.Type == want {
			return m
		}
	}
}

func TestTwoClientsConvergeOverWebSocket(t *testing.T) {
	h := New()
	srv := httptest.NewServer(http.HandlerFunc(h.ServeWS))
	defer srv.Close()

	a := dialConn(t, srv, "r1")
	defer a.Close()
	readUntil(t, a, MsgInit) // a's initial (empty) snapshot

	b := dialConn(t, srv, "r1")
	defer b.Close()
	readUntil(t, b, MsgInit)

	// Client A builds "hi" locally and sends the ops.
	da := crdt.New(1)
	for i, ch := range "hi" {
		op := da.LocalInsert(i, ch)
		if err := a.WriteJSON(Message{Type: MsgOp, Op: &op}); err != nil {
			t.Fatal(err)
		}
	}

	// Client B receives the ops and applies them to its own replica.
	db := crdt.New(2)
	for i := 0; i < 2; i++ {
		m := readUntil(t, b, MsgOp)
		db.Apply(*m.Op)
	}

	if got := db.Value(); got != "hi" {
		t.Fatalf("client B converged to %q, want %q", got, "hi")
	}
}

func TestLateJoinerReceivesSnapshot(t *testing.T) {
	h := New()
	srv := httptest.NewServer(http.HandlerFunc(h.ServeWS))
	defer srv.Close()

	// First client types "abc".
	a := dialConn(t, srv, "r2")
	defer a.Close()
	readUntil(t, a, MsgInit)

	da := crdt.New(1)
	for i, ch := range "abc" {
		op := da.LocalInsert(i, ch)
		a.WriteJSON(Message{Type: MsgOp, Op: &op})
	}
	// Give the server a moment to apply + log the ops.
	time.Sleep(100 * time.Millisecond)

	// A late joiner should receive the full op log in its init snapshot.
	b := dialConn(t, srv, "r2")
	defer b.Close()
	init := readUntil(t, b, MsgInit)
	if len(init.Ops) != 3 {
		t.Fatalf("late joiner got %d ops, want 3", len(init.Ops))
	}
	db := crdt.New(2)
	for _, op := range init.Ops {
		db.Apply(op)
	}
	if got := db.Value(); got != "abc" {
		t.Fatalf("late joiner rebuilt %q, want %q", got, "abc")
	}
}

func TestPresenceCountsParticipants(t *testing.T) {
	h := New()
	srv := httptest.NewServer(http.HandlerFunc(h.ServeWS))
	defer srv.Close()

	a := dialConn(t, srv, "r3")
	defer a.Close()
	readUntil(t, a, MsgInit)

	// When B joins, A should receive a presence update showing 2 participants.
	// (A first sees its own join presence of 1, then 2 when B joins.)
	b := dialConn(t, srv, "r3")
	defer b.Close()

	deadline := time.Now().Add(2 * time.Second)
	for {
		pres := readUntil(t, a, MsgPresence)
		if pres.Count == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("never saw presence count 2 (last was %d)", pres.Count)
		}
	}
}
