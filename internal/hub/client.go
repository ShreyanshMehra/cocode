package hub

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Demo app: allow any origin. Tighten for production.
	CheckOrigin: func(*http.Request) bool { return true },
}

// Hub owns all rooms and routes new connections to the right one.
type Hub struct {
	mu    sync.Mutex
	rooms map[string]*Room
}

// New creates an empty Hub.
func New() *Hub {
	return &Hub{rooms: make(map[string]*Room)}
}

// room returns the room with the given id, creating and starting it if needed.
func (h *Hub) room(id string) *Room {
	h.mu.Lock()
	defer h.mu.Unlock()
	r, ok := h.rooms[id]
	if !ok {
		r = newRoom(id)
		h.rooms[id] = r
		go r.run()
	}
	return r
}

// ServeWS upgrades an HTTP request to a WebSocket and joins it to a room. The
// room is taken from the "room" query parameter (default "default").
func (h *Hub) ServeWS(w http.ResponseWriter, req *http.Request) {
	roomID := req.URL.Query().Get("room")
	if roomID == "" {
		roomID = "default"
	}
	conn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Printf("ws upgrade: %v", err)
		return
	}
	room := h.room(roomID)
	c := &Client{conn: conn, room: room, send: make(chan Message, 64)}
	room.register <- c

	go c.writePump()
	go c.readPump()
}

// Client is a single WebSocket connection participating in a room.
type Client struct {
	conn *websocket.Conn
	room *Room
	send chan Message
}

// trySend queues a message, dropping it if the client's buffer is full (a slow
// consumer must not block the room goroutine).
func (c *Client) trySend(m Message) {
	select {
	case c.send <- m:
	default:
		// Buffer full: drop. The CRDT will still converge from later ops/init.
	}
}

// readPump reads operations from the client and forwards them to the room.
func (c *Client) readPump() {
	defer func() {
		c.room.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		var msg Message
		if err := c.conn.ReadJSON(&msg); err != nil {
			return
		}
		if msg.Type == MsgOp && msg.Op != nil {
			c.room.incoming <- inbound{from: c, op: *msg.Op}
		}
	}
}

// writePump writes queued messages (and periodic pings) to the client.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteJSON(msg); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
