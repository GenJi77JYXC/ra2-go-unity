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

// defaultOwner is hardcoded until Phase 6 gives connections real player
// identity — every command is treated as coming from player 1.
const defaultOwner = 1

// Server owns the HTTP endpoint and all connected clients.
//
// Client bookkeeping (registering/removing connections) is guarded by mu.
// This is separate from the World-mutation rule in main.go: World is only
// ever touched by the tick-loop goroutine, but the connection map is pure
// network state, so an ordinary mutex is fine here. Commands parsed off a
// connection go through the commands channel instead, so main.go's tick
// loop — not this per-connection goroutine — is what actually calls into
// World.
type Server struct {
	world *game.World

	mu      sync.Mutex
	clients map[*Client]struct{}
	nextID  int

	commands chan game.Command
}

func NewServer(world *game.World) *Server {
	return &Server{
		world:    world,
		clients:  make(map[*Client]struct{}),
		commands: make(chan game.Command, 32),
	}
}

// Commands is drained by main.go's tick loop once per tick.
func (s *Server) Commands() <-chan game.Command {
	return s.commands
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

	s.readCommands(conn)
}

// readCommands blocks reading messages from conn until it closes or errors,
// forwarding valid move commands to s.commands. This replaces Phase 1's
// CloseRead placeholder — actively reading is also what makes the
// underlying library handle ping/pong control frames, regardless of
// whether the payload itself is interesting.
func (s *Server) readCommands(conn *websocket.Conn) {
	ctx := context.Background()
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}

		var cmd ClientCommand
		if err := json.Unmarshal(data, &cmd); err != nil {
			log.Printf("bad client command: %v", err)
			continue
		}

		if cmd.Type != "move" {
			continue // "attack" (Phase 4) / "build" (Phase 5) aren't handled yet
		}

		select {
		case s.commands <- toGameCommand(cmd):
		default:
			log.Printf("command buffer full, dropping command")
		}
	}
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

func toGameCommand(cmd ClientCommand) game.Command {
	return game.Command{
		UnitIDs: cmd.UnitIDs,
		TargetX: cmd.TargetX,
		TargetY: cmd.TargetY,
		Owner:   defaultOwner,
	}
}
