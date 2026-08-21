package game

import (
	"math"
	"time"
)

// TickInterval is the authoritative simulation step. main.go's ticker and
// World.Tick's dt must agree on this, so it lives here as the single
// source of truth.
const TickInterval = 50 * time.Millisecond

const (
	unitSpeed      = 3.0  // world units per second
	arriveDistance = 0.05 // close enough to a waypoint counts as arrived
)

// Unit is a single controllable game object.
type Unit struct {
	ID    int     `json:"id"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Owner int     `json:"-"`

	// Path is the remaining waypoints (cell centers) to walk through, set by
	// HandleCommand from an A* route. Empty means idle.
	Path []Point `json:"-"`
}

// update advances the unit along Path by unitSpeed*dt world units, carrying
// over any leftover movement budget into the next waypoint within the same
// tick instead of losing it at every waypoint transition.
func (u *Unit) update(dt float64) {
	step := unitSpeed * dt

	for step > 0 && len(u.Path) > 0 {
		target := u.Path[0]
		dx := target.X - u.X
		dy := target.Y - u.Y
		dist := math.Hypot(dx, dy)

		if step >= dist || dist <= arriveDistance {
			u.X, u.Y = target.X, target.Y
			u.Path = u.Path[1:]
			step -= dist
			continue
		}

		u.X += dx / dist * step
		u.Y += dy / dist * step
		step = 0
	}
}

// World holds all authoritative game state. The tick loop in main.go is the
// only goroutine that should ever mutate a World — see the "并发安全收敛到
// 主循环" note in the learning plan. Commands from clients arrive on a
// channel (network.Server.Commands()) and main.go's tick loop drains it
// into HandleCommand before each Tick, so this rule still holds. Map is
// built once and never mutated, so it's exempt from this rule (see
// GameMap's doc comment).
type World struct {
	TickCount int64
	Units     []*Unit
	Map       *GameMap
}

// NewWorld builds a starting world with a single placeholder unit, matching
// the Phase 1 "Hello Tank" milestone, plus the Phase 3 test map. Owner is
// hardcoded to 1 — there's only one player until Phase 6 adds real
// multiplayer identity. The unit starts at the center of cell (0,0) to
// match the cell-centered convention pathfinding uses for waypoints.
func NewWorld() *World {
	return &World{
		Map: NewTestMap(),
		Units: []*Unit{
			{ID: 1, X: 0.5, Y: 0.5, Owner: 1},
		},
	}
}

// Tick advances the simulation by one step of length dt seconds (called
// every TickInterval by main.go).
func (w *World) Tick(dt float64) {
	w.TickCount++

	for _, u := range w.Units {
		u.update(dt)
	}
}

// Command is a move order for one or more units, already translated from
// the wire-format network.ClientCommand into game-internal terms.
type Command struct {
	UnitIDs []int
	TargetX float64
	TargetY float64
	// Owner identifies who issued the command. Hardcoded to 1 by the
	// network layer for now (see server.go), but HandleCommand checks it
	// against each unit's Owner regardless — establishing the habit now
	// means Phase 6 multiplayer won't need every command handler patched
	// to add ownership checks retroactively.
	Owner int
}

// HandleCommand routes a move order through A* and applies the resulting
// path to every unit it names that the command's owner actually controls.
// A unit whose target is unreachable or impassable (e.g. the middle of a
// lake) is left exactly as it was — it doesn't move, and any path it was
// already following is undisturbed.
func (w *World) HandleCommand(cmd Command) {
	for _, u := range w.Units {
		if u.Owner != cmd.Owner || !containsID(cmd.UnitIDs, u.ID) {
			continue
		}

		start := worldToCell(u.X, u.Y)
		goal := worldToCell(cmd.TargetX, cmd.TargetY)

		path := w.Map.FindPath(start, goal)
		if len(path) <= 1 {
			continue // already there, unreachable, or impassable
		}

		// path[0] is the unit's current cell; skip it so the unit heads
		// straight for the next waypoint instead of first snapping to the
		// center of the cell it's already standing in.
		u.Path = make([]Point, len(path)-1)
		for i, c := range path[1:] {
			u.Path[i] = cellCenterWorld(c)
		}
	}
}

func containsID(ids []int, id int) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

// Snapshot is what gets sent to clients every tick.
type Snapshot struct {
	Tick  int64   `json:"tick"`
	Units []*Unit `json:"units"`
}

func (w *World) Snapshot() Snapshot {
	return Snapshot{
		Tick:  w.TickCount,
		Units: w.Units,
	}
}
