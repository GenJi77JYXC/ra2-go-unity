package network

// ServerMessage wraps everything sent server -> client. Before Phase 6 the
// only thing a connection ever received was a GameState, so it could be
// unmarshalled directly; now the same socket carries lobby traffic too and
// the receiver needs to know which it's looking at. Unused payloads are
// omitted so a room list doesn't drag an empty world along with it.
type ServerMessage struct {
	Type   string       `json:"type"` // "rooms", "room", "state", "result", "error"
	Rooms  []RoomInfo   `json:"rooms,omitempty"`
	Room   *RoomInfo    `json:"room,omitempty"`
	State  *GameState   `json:"state,omitempty"`
	Result *MatchResult `json:"result,omitempty"`
	Error  string       `json:"error,omitempty"`
}

// MatchResult is sent once when a match is decided. Outcome is phrased
// from the receiving player's side so the client doesn't have to work out
// whether a winner id means them.
type MatchResult struct {
	Winner  int    `json:"winner"`  // 0 = draw
	Outcome string `json:"outcome"` // "win", "lose", "draw"
}

// RoomInfo describes a room in the lobby. YourPlayerID is only meaningful
// in a "room" message addressed to a member — it's how a client learns
// which Owner in the world is theirs, replacing the hardcoded 1 that stood
// in for identity from Phase 2 through Phase 5.
type RoomInfo struct {
	ID           int          `json:"id"`
	Name         string       `json:"name"`
	State        string       `json:"state"`   // "waiting", "playing", "finished"
	Victory      string       `json:"victory"` // "buildings", "conyard", "annihilation"
	Players      []PlayerInfo `json:"players"`
	YourPlayerID int          `json:"yourPlayerId,omitempty"`
}

type PlayerInfo struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Ready bool   `json:"ready"`
}

// GameState is sent server -> client every tick.
// Field names must match Unity's [Serializable] struct exactly
// (JsonUtility.FromJson<T>() is case-sensitive).
//
// MapWidth/MapHeight/Tiles are only populated on the one-off IsInitial
// message a new connection gets when it joins (see server.go's
// DrainNewClients) — omitempty keeps them out of the other ~20 GameState
// messages sent every second, since a 20x20 map's tile data would otherwise
// dwarf the actual per-tick unit updates.
type GameState struct {
	Tick      int64              `json:"tick"`
	IsInitial bool               `json:"isInitial"`
	MapWidth  int                `json:"mapWidth,omitempty"`
	MapHeight int                `json:"mapHeight,omitempty"`
	Tiles     []TileData         `json:"tiles,omitempty"`
	BuildMenu []BuildOption      `json:"buildMenu,omitempty"` // initial-only, like Tiles
	Units     []UnitSnapshot     `json:"units"`
	Buildings []BuildingSnapshot `json:"buildings"`
	Money     int                `json:"money"`
	Power     int                `json:"power"`

	// The structure the viewing player is currently building. RA2 builds
	// first and places second, so this has no map position — it's what
	// drives the build menu's progress readout and its "ready to place"
	// state. PendingType is "" when nothing is being built.
	PendingType     string  `json:"pendingType"`
	PendingProgress float64 `json:"pendingProgress"`
	PendingReady    bool    `json:"pendingReady"`

	Queues []QueueSnapshot `json:"queues"`
}

// QueueSnapshot is one production category's status. Categories are keyed
// by the building type that produces them and are shared by every factory
// of that type (see game.Player.Queues), so this is per-player state, not
// per-building.
type QueueSnapshot struct {
	Category string  `json:"category"`
	Item     string  `json:"item"`
	Progress float64 `json:"progress"` // 0..1
	Length   int     `json:"length"`
}

// BuildOption is one entry in the client's build menu, sent once on join.
type BuildOption struct {
	Type          string   `json:"type"`
	Cost          int      `json:"cost"`
	Width         int      `json:"width"`
	Height        int      `json:"height"`
	Power         int      `json:"power"`
	Produces      []string `json:"produces"`
	Prerequisites []string `json:"prerequisites"`
}

type BuildingSnapshot struct {
	ID      int    `json:"id"`
	Type    string `json:"type"`
	Owner   int    `json:"owner"`
	CellX   int    `json:"cellX"`
	CellY   int    `json:"cellY"`
	HP      int    `json:"hp"`
	MaxHP   int    `json:"maxHp"`
	IsBuilt bool   `json:"isBuilt"`

	// IsPrimary marks the factory that finished units of this category
	// walk out of — all factories of a type share one queue, so exactly
	// one has to be the exit.
	IsPrimary bool `json:"isPrimary"`
}

// TileData mirrors game.Tile. Type is game.TerrainType as a raw int
// (Grass=0, Road=1, Water=2, Cliff=3, Ore=4) — the Unity-side enum must
// stay in the same order.
type TileData struct {
	Type     int  `json:"type"`
	Passable bool `json:"passable"`
}

type UnitSnapshot struct {
	ID    int     `json:"id"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Owner int     `json:"owner"` // Phase 4: which player controls this unit
	HP    int     `json:"hp"`
	MaxHP int     `json:"maxHp"`
}

// ClientCommand is sent client -> server. It carries both lobby traffic
// ("listRooms", "createRoom", "joinRoom", "leaveRoom", "setReady") and
// in-game orders ("move", "attack", "build", "place", "produce", "cancel",
// "setPrimary"), kept in one struct because Unity's JsonUtility has to
// commit to a single type per FromJson call.
type ClientCommand struct {
	Type string `json:"type"`

	// lobby
	RoomID     int    `json:"roomId"`
	Victory    string `json:"victory"` // createRoom only
	PlayerName string `json:"playerName"`
	Ready      bool   `json:"ready"`

	UnitIDs      []int   `json:"unitIds"`
	TargetX      float64 `json:"targetX"`      // move
	TargetY      float64 `json:"targetY"`      // move
	TargetUnitID int     `json:"targetUnitId"` // attack

	ItemType   string `json:"itemType"`   // build: building type; produce: unit type
	CellX      int    `json:"cellX"`      // build
	CellY      int    `json:"cellY"`      // build
	BuildingID int    `json:"buildingId"` // produce: which factory
}
