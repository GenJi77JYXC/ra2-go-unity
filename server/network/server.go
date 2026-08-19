package network

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"server/game"

	"github.com/coder/websocket"
)

// Server owns the HTTP endpoint and all connected clients.
//
// Client bookkeeping (registering/removing connections) is guarded by mu.
// This is separate from the World-mutation rule in main.go: World is only
// ever touched by the tick-loop goroutine, but the connection map is pure
// network state, so an ordinary mutex is fine here.
type Server struct {
	world *game.World

	mu      sync.Mutex
	clients map[*Client]struct{}
	nextID  int
}

func NewServer(world *game.World) *Server {
	return &Server{
		world:   world,
		clients: make(map[*Client]struct{}),
	}
}

func (s *Server) ListenAndServe(addr string) {
	http.HandleFunc("/ws", s.handleWS)
	log.Printf("server listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		log.Printf("websocket accept failed: %v", err)
		return
	}

	client := s.addClient(conn)
	defer s.removeClient(client)

	go client.writePump()

	// Phase 1 only broadcasts state; it doesn't process client -> server
	// messages yet (that's Phase 2's move command). CloseRead discards
	// anything the client sends and cancels ctx once the connection closes,
	// which is what we use to detect disconnects.
	ctx := conn.CloseRead(context.Background())
	<-ctx.Done()
}

func (s *Server) addClient(conn *websocket.Conn) *Client {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextID++
	client := newClient(s.nextID, conn)
	s.clients[client] = struct{}{}
	log.Printf("client %d connected (total=%d)", client.ID, len(s.clients))
	return client
}

func (s *Server) removeClient(client *Client) {
	s.mu.Lock()
	delete(s.clients, client)
	total := len(s.clients)
	s.mu.Unlock()

	client.close()
	log.Printf("client %d disconnected (total=%d)", client.ID, total)
}

// Broadcast sends the latest snapshot to every connected client.
func (s *Server) Broadcast(snapshot game.Snapshot) {
	data, err := json.Marshal(toGameState(snapshot))
	if err != nil {
		log.Printf("marshal snapshot failed: %v", err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for client := range s.clients {
		client.Send(data)
	}
}

func toGameState(snapshot game.Snapshot) GameState {
	units := make([]UnitSnapshot, len(snapshot.Units))
	for i, u := range snapshot.Units {
		units[i] = UnitSnapshot{ID: u.ID, X: u.X, Y: u.Y}
	}
	return GameState{Tick: snapshot.Tick, Units: units}
}
