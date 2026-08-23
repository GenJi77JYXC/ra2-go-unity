package network

import (
	"encoding/json"
	"errors"
	"log"
	"sort"
	"sync"
	"time"

	"server/game"
)

var (
	errRoomFull    = errors.New("room is full")
	errRoomStarted = errors.New("game already started")
)

func newTicker() *time.Ticker { return time.NewTicker(game.TickInterval) }

func sortRoomsByID(rooms []*Room) {
	sort.Slice(rooms, func(i, j int) bool { return rooms[i].ID < rooms[j].ID })
}

const maxPlayersPerRoom = 2

const (
	RoomWaiting  = "waiting"
	RoomPlaying  = "playing"
	RoomFinished = "finished"
)

// RoomPlayer is one seat in a room. ID doubles as the player's Owner in
// that room's World, so ownership checks in the game package finally act
// on something real instead of the hardcoded 1 they saw through Phase 5.
type RoomPlayer struct {
	ID     int
	Name   string
	Ready  bool
	client *Client
}

// Room owns one game. Its own goroutine (run) is the only thing that ever
// touches world, preserving the single-writer rule the game package
// depends on — commands arrive over a channel exactly like they did when
// there was one global tick loop, just one channel per room now.
//
// mu guards the lobby-side fields (players, state), which the connection
// goroutines do touch: joining, leaving and readying all happen outside
// the tick loop.
type Room struct {
	ID      int
	Name    string
	Victory string // chosen by whoever created the room

	mu      sync.Mutex
	state   string
	players []*RoomPlayer
	nextID  int

	world    *game.World
	commands chan game.Command
	stop     chan struct{}
}

func newRoom(id int, name, victory string) *Room {
	if !game.ValidVictoryCondition(victory) {
		victory = game.VictoryBuildings
	}

	return &Room{
		ID:       id,
		Name:     name,
		Victory:  victory,
		state:    RoomWaiting,
		commands: make(chan game.Command, 64),
		stop:     make(chan struct{}),
	}
}

// join seats a client, returning their seat. Fails once the room is full
// or the match has already started — there's no mid-game join, since a
// late arrival would need the world handed to it separately.
func (r *Room) join(client *Client, name string) (*RoomPlayer, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.state != RoomWaiting {
		return nil, errRoomStarted
	}
	if len(r.players) >= maxPlayersPerRoom {
		return nil, errRoomFull
	}

	r.nextID++
	player := &RoomPlayer{ID: r.nextID, Name: name, client: client}
	r.players = append(r.players, player)
	return player, nil
}

// leave removes a seat. A player dropping out of a match in progress ends
// it rather than leaving a half-empty game running: with only two seats
// there's no game left to play.
func (r *Room) leave(player *RoomPlayer) (empty bool) {
	r.mu.Lock()

	for i, p := range r.players {
		if p == player {
			r.players = append(r.players[:i], r.players[i+1:]...)
			break
		}
	}

	stopped := false
	if r.state == RoomPlaying {
		r.state = RoomFinished
		stopped = true
	}
	empty = len(r.players) == 0
	r.mu.Unlock()

	if stopped {
		close(r.stop)
	}
	return empty
}

// setReady flips a seat's ready flag and reports whether that was the
// click that starts the match.
func (r *Room) setReady(player *RoomPlayer, ready bool) (started bool) {
	r.mu.Lock()

	player.Ready = ready

	allReady := len(r.players) == maxPlayersPerRoom
	for _, p := range r.players {
		if !p.Ready {
			allReady = false
		}
	}

	if !allReady || r.state != RoomWaiting {
		r.mu.Unlock()
		return false
	}

	r.state = RoomPlaying
	r.world = game.NewWorld(r.Victory)
	r.mu.Unlock()

	go r.run()
	return true
}

// submit hands a command to the room's tick loop. Non-blocking for the
// same reason Client.Send is: a connection goroutine must never stall on
// a busy room.
func (r *Room) submit(cmd game.Command) {
	select {
	case r.commands <- cmd:
	default:
		log.Printf("room %d command buffer full, dropping command", r.ID)
	}
}

// run is the room's tick loop, the single writer to r.world.
func (r *Room) run() {
	log.Printf("room %d starting", r.ID)

	ticker := newTicker()
	defer ticker.Stop()

	dt := game.TickInterval.Seconds()

	// Everyone gets the map and build menu once, up front — the same
	// one-off full snapshot a fresh connection used to get in Phase 3.
	for _, p := range r.snapshotPlayers() {
		p.client.Send(mustMarshal(ServerMessage{
			Type:  "state",
			State: ptr(initialGameState(r.world, p.ID)),
		}))
	}

	for {
		select {
		case <-r.stop:
			log.Printf("room %d stopped", r.ID)
			return
		case <-ticker.C:
			r.drainCommands()
			r.world.Tick(dt)
			r.broadcast()

			if over, winner := r.world.Outcome(); over {
				r.finish(winner)
				return
			}
		}
	}
}

// finish ends a decided match: it tells each player how it went for them
// specifically, then stops ticking. Players stay seated until they leave,
// which is what cleans the room up.
func (r *Room) finish(winner int) {
	r.mu.Lock()
	if r.state != RoomPlaying {
		r.mu.Unlock()
		return // already ended some other way, e.g. a player quit
	}
	r.state = RoomFinished
	r.mu.Unlock()

	log.Printf("room %d finished, winner=%d", r.ID, winner)

	for _, p := range r.snapshotPlayers() {
		p.client.Send(mustMarshal(ServerMessage{
			Type: "result",
			Result: &MatchResult{
				Winner:  winner,
				Outcome: outcomeFor(p.ID, winner),
			},
		}))
	}

	r.notifyRoomState()
	close(r.stop)
}

func outcomeFor(playerID, winner int) string {
	switch winner {
	case 0:
		return "draw"
	case playerID:
		return "win"
	default:
		return "lose"
	}
}

func (r *Room) drainCommands() {
	for {
		select {
		case cmd := <-r.commands:
			r.world.HandleCommand(cmd)
		default:
			return
		}
	}
}

// broadcast sends each player their own view. Unlike the single-player
// phases this can't marshal once and reuse the bytes: the economy block
// differs per viewer, so it's one marshal per player per tick.
func (r *Room) broadcast() {
	for _, p := range r.snapshotPlayers() {
		state := toGameState(r.world.Snapshot(p.ID))
		p.client.Send(mustMarshal(ServerMessage{Type: "state", State: &state}))
	}
}

// snapshotPlayers copies the seat list so the tick loop can iterate it
// without holding mu while sending.
func (r *Room) snapshotPlayers() []*RoomPlayer {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]*RoomPlayer, len(r.players))
	copy(out, r.players)
	return out
}

// info describes the room for the lobby. Passing a seat fills in
// YourPlayerID so that client knows which Owner is theirs.
func (r *Room) info(forPlayer *RoomPlayer) RoomInfo {
	r.mu.Lock()
	defer r.mu.Unlock()

	players := make([]PlayerInfo, len(r.players))
	for i, p := range r.players {
		players[i] = PlayerInfo{ID: p.ID, Name: p.Name, Ready: p.Ready}
	}

	info := RoomInfo{
		ID:      r.ID,
		Name:    r.Name,
		State:   r.state,
		Victory: r.Victory,
		Players: players,
	}
	if forPlayer != nil {
		info.YourPlayerID = forPlayer.ID
	}
	return info
}

// notifyRoomState pushes the current room to every member, so a join or a
// ready-click shows up on the other player's screen without polling.
func (r *Room) notifyRoomState() {
	for _, p := range r.snapshotPlayers() {
		info := r.info(p)
		p.client.Send(mustMarshal(ServerMessage{Type: "room", Room: &info}))
	}
}

// RoomManager is the lobby: it owns every room and hands them out by ID.
type RoomManager struct {
	mu     sync.Mutex
	rooms  map[int]*Room
	nextID int
}

func NewRoomManager() *RoomManager {
	return &RoomManager{rooms: map[int]*Room{}}
}

func (m *RoomManager) create(name, victory string) *Room {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.nextID++
	room := newRoom(m.nextID, name, victory)
	m.rooms[room.ID] = room
	log.Printf("room %d (%q) created, victory=%s", room.ID, name, room.Victory)
	return room
}

func (m *RoomManager) get(id int) *Room {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.rooms[id]
}

func (m *RoomManager) remove(id int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.rooms, id)
	log.Printf("room %d removed", id)
}

// list returns every room for the lobby view, sorted by ID so the client's
// list doesn't reshuffle between refreshes.
func (m *RoomManager) list() []RoomInfo {
	m.mu.Lock()
	rooms := make([]*Room, 0, len(m.rooms))
	for _, r := range m.rooms {
		rooms = append(rooms, r)
	}
	m.mu.Unlock()

	sortRoomsByID(rooms)

	out := make([]RoomInfo, len(rooms))
	for i, r := range rooms {
		out[i] = r.info(nil)
	}
	return out
}

func mustMarshal(msg ServerMessage) []byte {
	data, err := json.Marshal(msg)
	if err != nil {
		// Every payload here is plain structs of strings/numbers/slices,
		// so a failure would mean a programming error, not bad input.
		log.Printf("marshal server message failed: %v", err)
		return nil
	}
	return data
}

func ptr(state GameState) *GameState { return &state }
