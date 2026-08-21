package network

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
	Tick      int64          `json:"tick"`
	IsInitial bool           `json:"isInitial"`
	MapWidth  int            `json:"mapWidth,omitempty"`
	MapHeight int            `json:"mapHeight,omitempty"`
	Tiles     []TileData     `json:"tiles,omitempty"`
	Units     []UnitSnapshot `json:"units"`
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

// ClientCommand is sent client -> server.
type ClientCommand struct {
	Type         string  `json:"type"` // "move" (Phase 2), "attack" (Phase 4), "build" (Phase 5), ...
	UnitIDs      []int   `json:"unitIds"`
	TargetX      float64 `json:"targetX"`      // move
	TargetY      float64 `json:"targetY"`      // move
	TargetUnitID int     `json:"targetUnitId"` // attack
}
