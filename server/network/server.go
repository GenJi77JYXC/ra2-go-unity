package network

import (
	"log"
	"net/http"

	"server/game"
)

// Server owns the HTTP endpoint and all connected clients.
//
// TODO(Phase 1 step 2 — see 学习计划 Phase 1 "Go 服务端要点"):
//  1. go get nhooyr.io/websocket
//  2. In handleWS, upgrade the connection with websocket.Accept(w, r, nil)
//     and register a *Client in a connections map guarded by a mutex (or
//     owned by the tick-loop goroutine, per the plan's concurrency note).
//  3. Implement Broadcast to marshal the snapshot to JSON once and write it
//     to every connected client.
//
// Until then this only proves the HTTP server starts and logs each tick,
// which is enough to validate the tick loop in main.go before touching
// WebSocket code at all.
type Server struct {
	world *game.World
}

func NewServer(world *game.World) *Server {
	return &Server{world: world}
}

func (s *Server) ListenAndServe(addr string) {
	http.HandleFunc("/ws", s.handleWS)
	log.Printf("server listening on %s (ws upgrade not implemented yet)", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	// TODO(Phase 1 step 2): replace with a real WebSocket upgrade + Client
	// registration once nhooyr.io/websocket is added.
	http.Error(w, "websocket not implemented yet", http.StatusNotImplemented)
}

// Broadcast sends the latest snapshot to every connected client.
func (s *Server) Broadcast(snapshot game.Snapshot) {
	// TODO(Phase 1 step 2): marshal to JSON and write to each *Client.
	log.Printf("tick=%d units=%d (broadcast not implemented yet)", snapshot.Tick, len(snapshot.Units))
}
