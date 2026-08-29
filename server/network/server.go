package network

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"sync"

	"server/game"

	"github.com/coder/websocket"
)

// Server owns the HTTP endpoint and hands connections to the lobby.
//
// It no longer holds a World: each Room owns its own, and each Room's tick
// loop is the single writer to it. A connection goroutine never touches
// game state directly — it parses a message and either calls a lobby
// method (guarded by Room.mu) or drops a command on the room's channel.
type Server struct {
	rooms *RoomManager

	mu      sync.Mutex
	clients map[*Client]struct{}
	nextID  int
}

func NewServer(rooms *RoomManager) *Server {
	return &Server{
		rooms:   rooms,
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

	s.readMessages(client, conn)
}

// session is the per-connection state the read loop owns outright. Keeping
// it in local scope rather than on Client means no other goroutine can
// read a half-updated membership — the room holds its own player list for
// broadcasting, and this is only ever used to route what this connection
// sends.
type session struct {
	room   *Room
	player *RoomPlayer
}

// readMessages blocks reading from conn until it closes or errors,
// dispatching lobby traffic and forwarding in-game orders to the client's
// room. Actively reading is also what makes the underlying library handle
// ping/pong control frames.
func (s *Server) readMessages(client *Client, conn *websocket.Conn) {
	ctx := context.Background()
	var sess session

	// Leaving on disconnect has to happen however the loop exits, not just
	// on a clean "leaveRoom" — a browser closing mid-match still has to
	// free the seat.
	defer func() {
		if sess.room != nil {
			s.leaveRoom(&sess)
		}
	}()

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

		s.handleMessage(client, &sess, cmd)
	}
}

func (s *Server) handleMessage(client *Client, sess *session, cmd ClientCommand) {
	switch cmd.Type {
	case "listRooms":
		client.Send(mustMarshal(ServerMessage{Type: "rooms", Rooms: s.rooms.list()}))

	case "createRoom":
		room := s.rooms.create(roomName(cmd), cmd.Victory)
		s.joinRoom(client, sess, room, cmd.PlayerName)

	case "joinRoom":
		room := s.rooms.get(cmd.RoomID)
		if room == nil {
			sendError(client, "room not found")
			return
		}
		s.joinRoom(client, sess, room, cmd.PlayerName)

	case "leaveRoom":
		if sess.room != nil {
			s.leaveRoom(sess)
		}

	case "setReady":
		if sess.room == nil {
			return
		}
		if started := sess.room.setReady(sess.player, cmd.Ready); started {
			log.Printf("room %d starting: all players ready", sess.room.ID)
		}
		sess.room.notifyRoomState()

	default:
		s.forwardGameCommand(sess, cmd)
	}
}

// forwardGameCommand hands an in-game order to the room's tick loop,
// stamped with the sender's real seat. This is the line that replaces the
// hardcoded owner every phase since Phase 2 was written against — the
// ownership checks inside game.HandleCommand finally have a value that can
// differ between callers.
func (s *Server) forwardGameCommand(sess *session, cmd ClientCommand) {
	switch cmd.Type {
	case "move", "attack", "build", "place", "produce", "cancel", "setPrimary":
	default:
		return // unknown command type
	}

	if sess.room == nil || sess.player == nil {
		return // not in a room: nothing to command
	}

	sess.room.submit(toGameCommand(cmd, sess.player.ID))
}

func (s *Server) joinRoom(client *Client, sess *session, room *Room, name string) {
	if sess.room != nil {
		s.leaveRoom(sess)
	}

	player, err := room.join(client, playerName(name, client.ID))
	if err != nil {
		sendError(client, err.Error())
		return
	}

	sess.room = room
	sess.player = player
	log.Printf("client %d joined room %d as player %d", client.ID, room.ID, player.ID)

	room.notifyRoomState()
}

func (s *Server) leaveRoom(sess *session) {
	room, player := sess.room, sess.player
	sess.room, sess.player = nil, nil

	if room.leave(player) {
		s.rooms.remove(room.ID)
		return
	}
	room.notifyRoomState()
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

func sendError(client *Client, message string) {
	client.Send(mustMarshal(ServerMessage{Type: "error", Error: message}))
}

func roomName(cmd ClientCommand) string {
	if cmd.PlayerName == "" {
		return "Room"
	}
	return cmd.PlayerName + "'s room"
}

func playerName(name string, clientID int) string {
	if name == "" {
		return "Player " + strconv.Itoa(clientID)
	}
	return name
}

func toGameState(snapshot game.Snapshot) GameState {
	return GameState{
		Tick:            snapshot.Tick,
		Units:           toUnitSnapshots(snapshot.Units),
		Buildings:       toBuildingSnapshots(snapshot.Buildings),
		Money:           snapshot.Money,
		Power:           snapshot.Power,
		PendingType:     snapshot.PendingType,
		PendingProgress: snapshot.PendingProgress,
		PendingReady:    snapshot.PendingReady,
		Queues:          toQueueSnapshots(snapshot.Queues),
		Ore:             snapshot.Ore,
	}
}

// initialGameState is the one-off full-state message each player gets when
// their match starts — it's the only GameState that carries the map and
// the build menu.
func initialGameState(world *game.World, forOwner int) GameState {
	m := world.Map
	tiles := make([]TileData, 0, m.Width*m.Height)
	for y := 0; y < m.Height; y++ {
		for x := 0; x < m.Width; x++ {
			t := m.Tiles[y][x]
			tiles = append(tiles, TileData{Type: int(t.Type), Passable: t.Passable})
		}
	}

	state := toGameState(world.Snapshot(forOwner))
	state.IsInitial = true
	state.MapWidth = m.Width
	state.MapHeight = m.Height
	state.Tiles = tiles
	state.BuildMenu = toBuildMenu()
	state.OreCells = toOreCells(m.OreCells())
	return state
}

func toOreCells(cells []game.OreCell) []OreCellData {
	out := make([]OreCellData, len(cells))
	for i, c := range cells {
		out[i] = OreCellData{X: c.X, Y: c.Y}
	}
	return out
}

func toBuildMenu() []BuildOption {
	catalog := game.BuildingCatalog()
	out := make([]BuildOption, len(catalog))
	for i, c := range catalog {
		out[i] = BuildOption{
			Type:          c.Type,
			Cost:          c.Cost,
			Width:         c.Width,
			Height:        c.Height,
			Power:         c.Power,
			Produces:      c.Produces,
			Prerequisites: c.Prerequisites,
		}
	}
	return out
}

func toQueueSnapshots(queues []game.QueueState) []QueueSnapshot {
	out := make([]QueueSnapshot, len(queues))
	for i, q := range queues {
		out[i] = QueueSnapshot{
			Category: q.Category,
			Item:     q.Item,
			Progress: q.Progress,
			Length:   q.Length,
		}
	}
	return out
}

func toBuildingSnapshots(buildings []*game.Building) []BuildingSnapshot {
	out := make([]BuildingSnapshot, len(buildings))
	for i, b := range buildings {
		out[i] = BuildingSnapshot{
			ID:        b.ID,
			Type:      b.Type,
			Owner:     b.Owner,
			CellX:     b.CellX,
			CellY:     b.CellY,
			HP:        b.HP,
			MaxHP:     b.MaxHP,
			IsBuilt:   b.IsBuilt,
			IsPrimary: b.IsPrimary,
		}
	}
	return out
}

func toUnitSnapshots(units []*game.Unit) []UnitSnapshot {
	out := make([]UnitSnapshot, len(units))
	for i, u := range units {
		out[i] = UnitSnapshot{
			ID: u.ID, X: u.X, Y: u.Y, Owner: u.Owner,
			HP: u.HP, MaxHP: u.MaxHP, Type: u.Template,
		}
	}
	return out
}

func toGameCommand(cmd ClientCommand, owner int) game.Command {
	return game.Command{
		Type:         cmd.Type,
		UnitIDs:      cmd.UnitIDs,
		TargetX:      cmd.TargetX,
		TargetY:      cmd.TargetY,
		TargetUnitID: cmd.TargetUnitID,
		ItemType:     cmd.ItemType,
		CellX:        cmd.CellX,
		CellY:        cmd.CellY,
		BuildingID:   cmd.BuildingID,
		Owner:        owner,
	}
}
