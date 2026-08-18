package network

// GameState is sent server -> client every tick.
// Field names must match Unity's [Serializable] struct exactly
// (JsonUtility.FromJson<T>() is case-sensitive).
type GameState struct {
	Tick      int64          `json:"tick"`
	IsInitial bool           `json:"isInitial"` // Phase 3: true on first full map snapshot
	Units     []UnitSnapshot `json:"units"`
}

type UnitSnapshot struct {
	ID int     `json:"id"`
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
}

// ClientCommand is sent client -> server.
type ClientCommand struct {
	Type    string  `json:"type"` // "move" (Phase 2), "attack" (Phase 4), "build" (Phase 5), ...
	UnitIDs []int   `json:"unitIds"`
	TargetX float64 `json:"targetX"`
	TargetY float64 `json:"targetY"`

	// TODO(Phase 2): every handler for this command must verify the
	// requesting connection actually owns UnitIDs before acting on them.
	// Add the check now (even with a hardcoded Owner=1) so Phase 6
	// multiplayer doesn't need to retrofit it everywhere.
}
