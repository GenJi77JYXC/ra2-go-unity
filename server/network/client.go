package network

import (
	"context"
	"log"
	"time"

	"github.com/coder/websocket"
)

// Client represents one connected WebSocket connection.
//
// The tick loop (main.go) calls Server.Broadcast every 50ms; Send must never
// block it on a slow client, so writes go through a buffered channel drained
// by a dedicated writePump goroutine per client.
type Client struct {
	ID   int
	conn *websocket.Conn
	out  chan []byte
}

func newClient(id int, conn *websocket.Conn) *Client {
	return &Client{
		ID:   id,
		conn: conn,
		out:  make(chan []byte, 8),
	}
}

// Send queues a message for this client. If the client's buffer is full
// (too slow to keep up with 20 tick/s), the snapshot is dropped rather than
// blocking the caller — the next tick's snapshot supersedes it anyway.
func (c *Client) Send(data []byte) {
	select {
	case c.out <- data:
	default:
		log.Printf("client %d send buffer full, dropping snapshot", c.ID)
	}
}

func (c *Client) writePump() {
	for data := range c.out {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := c.conn.Write(ctx, websocket.MessageText, data)
		cancel()
		if err != nil {
			return
		}
	}
}

func (c *Client) close() {
	close(c.out)
	c.conn.Close(websocket.StatusNormalClosure, "server closing connection")
}
