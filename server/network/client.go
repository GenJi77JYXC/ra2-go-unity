package network

// Client represents one connected WebSocket connection.
//
// TODO(Phase 1 step 2): once nhooyr.io/websocket is wired up in server.go,
// this should hold the *websocket.Conn plus a buffered outbound channel so
// Broadcast() never blocks the tick loop on a slow client.
type Client struct {
	ID int
}
