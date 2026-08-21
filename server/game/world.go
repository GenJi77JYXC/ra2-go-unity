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

	Template string `json:"-"` // key into unitTemplates
	Armor    string `json:"-"`
	HP       int    `json:"-"`
	MaxHP    int    `json:"-"`

	// Path is the remaining waypoints (cell centers) to walk through, set by
	// HandleCommand (move) or updateCombat's chase (attack). Empty means idle.
	Path []Point `json:"-"`

	// AttackTargetID is the unit ID currently being pursued/fired on, 0 if
	// none. FireCooldown counts down to the next shot once in range.
	AttackTargetID int     `json:"-"`
	FireCooldown   float64 `json:"-"`
}

func newUnit(id int, x, y float64, owner int, template string) *Unit {
	t := unitTemplates[template]
	return &Unit{
		ID:       id,
		X:        x,
		Y:        y,
		Owner:    owner,
		Template: template,
		Armor:    t.Armor,
		HP:       t.MaxHP,
		MaxHP:    t.MaxHP,
	}
}

// update advances the unit along Path by unitSpeed*dt world units, carrying
// over any leftover movement budget into the next waypoint within the same
// tick instead of losing it at every waypoint transition.
//
// TODO(known gap, deferred): units have no awareness of each other here —
// nothing stops two units' paths from overlapping mid-transit, so they can
// visually pass through one another while moving (their *destination*
// cells are kept distinct by handleMoveCommand's nearbyPassableCells, but
// nothing separates them along the way). Real local avoidance (steering
// behaviors, reciprocal velocity obstacles, etc.) is a substantial feature
// in its own right and out of scope for Phase 4's stated milestone —
// deliberately deferred rather than overlooked. If picked up later, this
// is the function that needs to become aware of nearby units' positions.
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

// NewWorld builds a starting world: 3 player-owned tanks and 2 enemy tanks
// on either side of the Phase 3 test map's cliff wall, so a Phase 4 attack
// order has to path around the same obstacle a move order would. Owner 1 is
// hardcoded as "the player" and Owner 2 as "the enemy" until Phase 6 adds
// real multiplayer identity.
func NewWorld() *World {
	return &World{
		Map: NewTestMap(),
		Units: []*Unit{
			newUnit(1, 0.5, 0.5, 1, "Tank"),
			newUnit(2, 1.5, 0.5, 1, "Tank"),
			newUnit(3, 2.5, 0.5, 1, "Tank"),
			newUnit(4, 15.5, 15.5, 2, "Tank"),
			newUnit(5, 17.5, 15.5, 2, "Tank"),
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

	w.updateCombat(dt)
	w.removeDeadUnits()
}

// Command is a move or attack order for one or more units, already
// translated from the wire-format network.ClientCommand into game-internal
// terms.
type Command struct {
	Type    string // "move" or "attack"
	UnitIDs []int

	// move
	TargetX float64
	TargetY float64

	// attack
	TargetUnitID int

	// Owner identifies who issued the command. Hardcoded to 1 by the
	// network layer for now (see server.go), but HandleCommand checks it
	// against each unit's Owner regardless — establishing the habit now
	// means Phase 6 multiplayer won't need every command handler patched
	// to add ownership checks retroactively.
	Owner int
}

// HandleCommand dispatches a command to the move or attack handler. Both
// verify the requesting Owner actually controls the named units before
// doing anything — this is the last Phase where that's a hardcoded no-op
// (every command comes from Owner 1 for now), so it's worth getting right
// here rather than retrofitting it once Phase 6 multiplayer needs it.
func (w *World) HandleCommand(cmd Command) {
	if cmd.Type == "attack" {
		w.handleAttackCommand(cmd)
		return
	}
	w.handleMoveCommand(cmd)
}

// handleMoveCommand routes a move order through A* and applies the
// resulting path to every unit it names that the command's owner actually
// controls. If the clicked cell is impassable (e.g. the middle of a lake),
// nobody moves — same as a single unit would do. Otherwise, when more than
// one unit is named, they're spread across distinct passable cells near
// the target instead of all pathing to the exact same point, where they'd
// end up stacked on top of each other.
func (w *World) handleMoveCommand(cmd Command) {
	goal := worldToCell(cmd.TargetX, cmd.TargetY)
	if !w.Map.PassableAt(goal.X, goal.Y) {
		return
	}

	var movers []*Unit
	for _, u := range w.Units {
		if u.Owner == cmd.Owner && containsID(cmd.UnitIDs, u.ID) {
			movers = append(movers, u)
		}
	}
	if len(movers) == 0 {
		return
	}

	targets := nearbyPassableCells(w.Map, goal, len(movers))

	for i, u := range movers {
		u.AttackTargetID = 0 // a fresh move order cancels any attack order

		start := worldToCell(u.X, u.Y)
		path := w.Map.FindPath(start, targets[i])
		if len(path) <= 1 {
			continue // already there or unreachable
		}

		// path[0] is the unit's current cell; skip it so the unit heads
		// straight for the next waypoint instead of first snapping to the
		// center of the cell it's already standing in.
		u.Path = toWaypoints(path[1:])
	}
}

// handleAttackCommand assigns AttackTargetID on every named unit the
// command's owner controls; updateCombat does the actual chasing/firing
// each tick. Ignored entirely if the target doesn't exist or belongs to
// the same owner — you can't attack your own units.
func (w *World) handleAttackCommand(cmd Command) {
	target := w.findUnit(cmd.TargetUnitID)
	if target == nil || target.Owner == cmd.Owner {
		return
	}

	for _, u := range w.Units {
		if u.Owner != cmd.Owner || !containsID(cmd.UnitIDs, u.ID) {
			continue
		}
		if unitTemplates[u.Template].Weapon == "" {
			continue // unarmed units can't be given attack orders
		}

		u.AttackTargetID = target.ID
		u.Path = nil // let updateCombat's chase() path toward the target fresh
	}
}

func toWaypoints(path []cell) []Point {
	pts := make([]Point, len(path))
	for i, c := range path {
		pts[i] = cellCenterWorld(c)
	}
	return pts
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
