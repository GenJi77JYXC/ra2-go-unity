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

	commands   chan game.Command
	newClients chan *Client
}

func NewServer(world *game.World) *Server {
	return &Server{
		world:      world,
		clients:    make(map[*Client]struct{}),
		commands:   make(chan game.Command, 32),
		newClients: make(chan *Client, 8),
	}
}

// Commands is drained by main.go's tick loop once per tick.
func (s *Server) Commands() <-chan game.Command {
	return s.commands
}

// DrainNewClients sends the current full world state — including the
// static map, which regular Broadcast calls never include — to every
// client that connected since the last tick. Like HandleCommand, this must
// only ever be called from the tick-loop goroutine: it reads s.world
// directly, which is only safe there.
func (s *Server) DrainNewClients() {
	for {
		select {
		case client := <-s.newClients:
			s.sendInitialSnapshot(client)
		default:
			return
		}
	}
}

func (s *Server) sendInitialSnapshot(client *Client) {
	data, err := json.Marshal(initialGameState(s.world))
	if err != nil {
		log.Printf("marshal initial snapshot failed: %v", err)
		return
	}
	client.Send(data)
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

	select {
	case s.newClients <- client:
	default:
		log.Printf("new-client buffer full, client %d won't get initial map snapshot", client.ID)
	}

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
	return GameState{Tick: snapshot.Tick, Units: toUnitSnapshots(snapshot.Units)}
}

// initialGameState is the one-off full-state message a new connection gets
// (see DrainNewClients) — it's the only GameState that carries the map.
func initialGameState(world *game.World) GameState {
	m := world.Map
	tiles := make([]TileData, 0, m.Width*m.Height)
	for y := 0; y < m.Height; y++ {
		for x := 0; x < m.Width; x++ {
			t := m.Tiles[y][x]
			tiles = append(tiles, TileData{Type: int(t.Type), Passable: t.Passable})
		}
	}

	return GameState{
		Tick:      world.TickCount,
		IsInitial: true,
		MapWidth:  m.Width,
		MapHeight: m.Height,
		Tiles:     tiles,
		Units:     toUnitSnapshots(world.Units),
	}
}

func toUnitSnapshots(units []*game.Unit) []UnitSnapshot {
	out := make([]UnitSnapshot, len(units))
	for i, u := range units {
		out[i] = UnitSnapshot{ID: u.ID, X: u.X, Y: u.Y}
	}
	return out
}

func toGameCommand(cmd ClientCommand) game.Command {
	return game.Command{
		UnitIDs: cmd.UnitIDs,
		TargetX: cmd.TargetX,
		TargetY: cmd.TargetY,
		Owner:   defaultOwner,
	}
}
