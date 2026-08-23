package network

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// Client represents one connected WebSocket connection.
//
// The tick loop (main.go) calls Server.Broadcast every 50ms; Send must never
// block it on a slow client, so writes go through a buffered channel drained
// by a dedicated writePump goroutine per client.
//
// Send and close can now race: Phase 3's DrainNewClients calls Send on a
// client pulled off a separate channel, with no guarantee the connection is
// still alive by the time it gets there (a client that connects and
// disconnects within the same tick is enough to trigger it). mu makes both
// safe regardless of call order — Send after close silently drops instead
// of sending on (and panicking on) a closed channel, and close itself is
// idempotent.
type Client struct {
	ID   int
	conn *websocket.Conn
	out  chan []byte

	mu     sync.Mutex
	closed bool
}

func newClient(id int, conn *websocket.Conn) *Client {
	return &Client{
		ID:   id,
		conn: conn,
		// 8 slots was only 0.4s of runway at 20 snapshots a second, which
		// a scene load or a GC pause blows through easily. Dropping is
		// harmless (the next snapshot supersedes it), but a wider buffer
		// means a brief stall costs nothing at all.
		out: make(chan []byte, 64),
	}
}

// Send queues a message for this client. If the client's buffer is full
// (too slow to keep up with 20 tick/s), the snapshot is dropped rather than
// blocking the caller — the next tick's snapshot supersedes it anyway. A
// message sent after the client has disconnected is silently dropped too.
func (c *Client) Send(data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return
	}

	select {
	case c.out <- data:
	default:
		log.Printf("client %d send buffer full, dropping snapshot", c.ID)
	}
}

// writeTimeout only exists to reap a connection that has genuinely gone
// away — a peer that stopped acknowledging entirely. It is deliberately
// generous, because the library closes the connection on any error
// including a context expiry, so a timeout here is fatal to the match.
//
// Protecting the tick loop from a merely slow client is Send's job, not
// this one's: Send drops snapshots when the buffer fills and never blocks.
// Conflating the two cost a real bug — a client stalled for a few seconds
// loading a scene backed up enough writes to blow a 5s timeout, and got
// disconnected mid-match for being briefly busy.
const writeTimeout = 30 * time.Second

func (c *Client) writePump() {
	for data := range c.out {
		ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
		err := c.conn.Write(ctx, websocket.MessageText, data)
		cancel()
		if err != nil {
			return
		}
	}
}

func (c *Client) close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return
	}
	c.closed = true

	close(c.out)
	c.conn.Close(websocket.StatusNormalClosure, "server closing connection")
}
