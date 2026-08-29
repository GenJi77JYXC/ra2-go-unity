package game

import (
	"math"
	"sort"
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

	// Harvest is non-nil only on harvesters, which run themselves off it
	// (see harvest.go). A pointer rather than more fields on Unit, since
	// nothing else in the game has any use for them.
	Harvest *HarvestState `json:"-"`

	// Cell is the cell the unit occupies. While InTransit it is walking
	// from Cell to NextCell and holds *both* in the occupancy table — see
	// World.arrive. X/Y stay the authoritative position for rendering and
	// weapon range; these are the collision view of the same thing.
	Cell      cell `json:"-"`
	NextCell  cell `json:"-"`
	InTransit bool `json:"-"`

	// Goal is where the unit was ultimately told to go. Keeping it means
	// anything that interrupts the trip — stepping aside for someone, a
	// re-route that only got partway — resumes on its own instead of
	// silently dropping the order (see World.finishPath).
	Goal    cell `json:"-"`
	HasGoal bool `json:"-"`

	// BlockedTime is how long the next cell has been occupied;
	// RetryCooldown throttles re-pathing while that lasts. HoldTime keeps
	// a unit that just stepped aside parked long enough for whoever it
	// made room for to actually get past. All three live in occupancy.go.
	BlockedTime   float64 `json:"-"`
	RetryCooldown float64 `json:"-"`
	HoldTime      float64 `json:"-"`
}

func newUnit(id int, x, y float64, owner int, template string) *Unit {
	t := unitTemplates[template]

	var harvest *HarvestState
	if t.Harvester {
		harvest = &HarvestState{}
	}

	return &Unit{
		ID:       id,
		Harvest:  harvest,
		X:        x,
		Y:        y,
		Cell:     worldToCell(x, y),
		Owner:    owner,
		Template: template,
		Armor:    t.Armor,
		HP:       t.MaxHP,
		MaxHP:    t.MaxHP,
	}
}

// updateUnit advances a unit along Path by unitSpeed*dt world units,
// carrying leftover movement budget between waypoints within the same tick
// instead of losing it at every transition.
//
// Movement is a series of cell-to-cell hops rather than free motion: to
// leave its current cell a unit must first *reserve* the next one, which
// is what stops two units walking through each other. A reservation that
// fails hands off to blocked() in occupancy.go.
func (w *World) updateUnit(u *Unit, dt float64) {
	step := unitSpeed * dt

	for step > 0 {
		if !u.InTransit {
			if len(u.Path) == 0 {
				w.finishPath(u, dt)
				if len(u.Path) == 0 {
					return
				}
			}

			next := worldToCell(u.Path[0].X, u.Path[0].Y)
			if next == u.Cell {
				u.Path = u.Path[1:] // already standing there
				continue
			}
			if !w.reserve(u, next) {
				w.blocked(u, next, dt)
				return
			}

			u.NextCell = next
			u.InTransit = true
			u.BlockedTime = 0
			u.RetryCooldown = 0
			// Popped here rather than on arrival: while InTransit the
			// destination is NextCell, so a fresh order can replace Path
			// mid-hop without the unit losing its first new waypoint.
			u.Path = u.Path[1:]
		}

		target := cellCenterWorld(u.NextCell)
		dx := target.X - u.X
		dy := target.Y - u.Y
		dist := math.Hypot(dx, dy)

		if step >= dist || dist <= arriveDistance {
			u.X, u.Y = target.X, target.Y
			step -= dist
			w.arrive(u)
			continue
		}

		u.X += dx / dist * step
		u.Y += dy / dist * step
		step = 0
	}
}

// finishPath decides what happens when a unit runs out of waypoints:
// either it's where it was headed, or something diverted it and it should
// pick the trip back up.
func (w *World) finishPath(u *Unit, dt float64) {
	if u.HoldTime > 0 {
		u.HoldTime -= dt
		return // just stepped aside; let the other unit through first
	}
	if !u.HasGoal {
		return
	}
	if u.Cell == u.Goal {
		u.HasGoal = false
		return
	}
	if !w.pathTo(u, u.Goal, w.staticEnterable()) {
		u.HasGoal = false // no route left — drop it rather than spin
	}
}

// pathTo runs A* toward goal and installs the result as the unit's Path.
// Reports false when there's no route, or the unit is already there.
func (w *World) pathTo(u *Unit, goal cell, enterable canEnter) bool {
	path := w.Map.FindPath(u.pathStart(), goal, enterable)
	if len(path) <= 1 {
		return false
	}

	// path[0] is where the unit already is (or is already committed to
	// reaching), so skip it — otherwise it would first snap back to the
	// center of the cell it's standing in.
	u.Path = toWaypoints(path[1:])
	return true
}

// pathStart is the cell a new path has to start from. A unit halfway into
// its next cell is committed to finishing that hop — it still holds the
// reservation — so re-routing from the cell behind it would make it walk
// backwards.
func (u *Unit) pathStart() cell {
	if u.InTransit {
		return u.NextCell
	}
	return u.Cell
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
	Buildings []*Building
	Players   map[int]*Player
	Map       *GameMap

	// Victory is the rule this match is played under, chosen by whoever
	// created the room (see victory.go).
	Victory string

	// occupied maps a cell to the ID of the unit holding it. This is the
	// dynamic obstacle layer — buildings are found by scanning Buildings,
	// terrain lives on Map. See occupancy.go.
	occupied map[cell]int

	// ore is how much each field cell still holds, and oreGrowth is the
	// countdown to the next top-up. Same split as occupied: Map knows
	// which cells are ore field, World knows what's left in them. See
	// harvest.go.
	ore       map[cell]int
	oreGrowth float64

	// nextID is a single ID space shared by units and buildings, so a
	// command naming an ID can never be ambiguous about which it meant.
	nextID int
}

// addUnit creates a unit, gives it the next ID and registers the cell it
// stands on. Every unit has to go through here (or placeUnit) — one that
// never lands in the occupancy table is invisible to everyone else's
// collision checks.
func (w *World) addUnit(x, y float64, owner int, template string) *Unit {
	w.nextID++
	u := newUnit(w.nextID, x, y, owner, template)
	w.Units = append(w.Units, u)
	w.placeUnit(u)
	return u
}

// NewWorld builds a starting world: 3 player-owned tanks and 2 enemy tanks
// on either side of the Phase 3 test map's cliff wall, so a Phase 4 attack
// order has to path around the same obstacle a move order would, plus a
// pre-built Construction Yard per side to seed Phase 5's tech tree. Owner 1
// is hardcoded as "the player" and Owner 2 as "the enemy" until Phase 6
// adds real multiplayer identity.
func NewWorld(victory string) *World {
	if !ValidVictoryCondition(victory) {
		victory = VictoryBuildings
	}

	w := &World{
		Map:      NewTestMap(),
		Players:  map[int]*Player{1: newPlayer(1), 2: newPlayer(2)},
		Victory:  victory,
		occupied: map[cell]int{},
	}
	w.fillOre()

	for _, u := range []struct {
		x, y  float64
		owner int
	}{
		{0.5, 0.5, 1}, {1.5, 0.5, 1}, {2.5, 0.5, 1},
		{15.5, 15.5, 2}, {17.5, 15.5, 2},
	} {
		w.addUnit(u.x, u.y, u.owner, "Tank")
	}

	for _, b := range []struct {
		cellX, cellY int
		owner        int
	}{
		{1, 2, 1},
		{16, 11, 2},
	} {
		w.nextID++
		w.Buildings = append(w.Buildings, newBuilding(w.nextID, "ConstructionYard", b.owner, b.cellX, b.cellY, true))
	}

	return w
}

// Tick advances the simulation by one step of length dt seconds (called
// every TickInterval by main.go).
func (w *World) Tick(dt float64) {
	w.TickCount++

	w.growOre(dt)

	for _, u := range w.Units {
		w.updateUnit(u, dt)
	}

	w.updateHarvesters(dt)
	w.updateCombat(dt)
	w.removeDestroyed()

	w.updateConstruction(dt)
	w.updateProduction(dt)
}

// Command is an order already translated from the wire-format
// network.ClientCommand into game-internal terms.
type Command struct {
	Type    string // "move", "attack", "build", "produce" or "cancel"
	UnitIDs []int

	// move
	TargetX float64
	TargetY float64

	// attack
	TargetUnitID int

	// build: what to place and where (cell coordinates of the footprint's
	// lower-left corner). produce: BuildingID is the factory, ItemType the
	// unit template to queue.
	ItemType   string
	CellX      int
	CellY      int
	BuildingID int

	// Owner identifies who issued the command. Hardcoded to 1 by the
	// network layer for now (see server.go), but HandleCommand checks it
	// against each unit's Owner regardless — establishing the habit now
	// means Phase 6 multiplayer won't need every command handler patched
	// to add ownership checks retroactively.
	Owner int
}

// HandleCommand dispatches a command to its handler. Every handler
// verifies the requesting Owner actually controls the units/buildings it
// names before doing anything — a hardcoded no-op while every command
// comes from Owner 1, but the checks are in place for when Phase 6 gives
// connections real identity.
func (w *World) HandleCommand(cmd Command) {
	switch cmd.Type {
	case "attack":
		w.handleAttackCommand(cmd)
	case "build":
		w.handleBuildCommand(cmd)
	case "place":
		w.handlePlaceCommand(cmd)
	case "produce":
		w.handleProduceCommand(cmd)
	case "cancel":
		w.handleCancelCommand(cmd)
	case "setPrimary":
		w.setPrimary(cmd.Owner, cmd.BuildingID)
	default:
		w.handleMoveCommand(cmd)
	}
}

// handleBuildCommand starts construction of a structure if the tech tree
// allows it and the player isn't already building something. No position
// is involved yet — RA2 builds first and places second, so the structure
// exists only as Player.Pending until it's Ready and the player picks a
// spot (see handlePlaceCommand).
//
// Nothing is charged here either: cost is drained gradually as it builds
// (see updateConstruction), so a cancellation refunds exactly what was
// spent, and a player who can't keep up with the payments simply stalls
// mid-construction.
func (w *World) handleBuildCommand(cmd Command) {
	if _, ok := buildingTemplates[cmd.ItemType]; !ok {
		return
	}

	player := w.Players[cmd.Owner]
	if player == nil || player.Pending != nil {
		return // one structure at a time
	}
	if !w.hasPrerequisites(cmd.Owner, buildingTemplates[cmd.ItemType].Prerequisites) {
		return
	}

	player.Pending = &Construction{Type: cmd.ItemType}
}

// handlePlaceCommand drops a finished-but-unplaced structure onto the map.
// It arrives complete: the build time was already spent getting it to
// Ready, so there's no on-map construction phase to wait through.
func (w *World) handlePlaceCommand(cmd Command) {
	player := w.Players[cmd.Owner]
	if player == nil || player.Pending == nil || !player.Pending.Ready {
		return
	}
	if !w.canPlace(player.Pending.Type, cmd.CellX, cmd.CellY) {
		return
	}

	buildingType := player.Pending.Type

	w.nextID++
	placed := newBuilding(w.nextID, buildingType, cmd.Owner, cmd.CellX, cmd.CellY, true)
	w.Buildings = append(w.Buildings, placed)
	player.Pending = nil

	// The first factory of a type becomes the one units walk out of; later
	// ones leave the flag where it is until the player moves it.
	w.ensurePrimary(cmd.Owner, buildingType)

	// A refinery arrives with a harvester, as in the original. Without it
	// you'd pay 1400 for a building that earns nothing until you also pay
	// 600 — and with no passive income, that's a hole a player who spent
	// down to the last credit could never climb out of.
	if buildingType == refineryType {
		w.spawnUnit("Harvester", placed)
	}
}

// handleProduceCommand queues a unit in the category queue belonging to
// the named factory's *type* — not to that specific building, since all
// factories of a type share one queue (see Player.Queues). Like
// construction, cost is charged incrementally as the item builds rather
// than up front, and only the head of the queue is ever being charged.
func (w *World) handleProduceCommand(cmd Command) {
	if _, ok := unitTemplates[cmd.ItemType]; !ok {
		return
	}

	player := w.Players[cmd.Owner]
	if player == nil {
		return
	}

	factory := w.findBuilding(cmd.BuildingID)
	if factory == nil || factory.Owner != cmd.Owner || !factory.IsBuilt {
		return
	}
	if !buildingTemplates[factory.Type].canProduce(cmd.ItemType) {
		return
	}

	q := player.queue(factory.Type)
	q.Items = append(q.Items, cmd.ItemType)
}

// handleCancelCommand undoes an order and refunds whatever it had been
// charged. With no BuildingID it scraps the player's pending structure;
// with one it drops the last item queued at that factory — which refunds
// nothing unless that item had reached the head of the queue, since only
// the head is ever charged.
func (w *World) handleCancelCommand(cmd Command) {
	player := w.Players[cmd.Owner]
	if player == nil {
		return
	}

	if cmd.BuildingID == 0 {
		if player.Pending != nil {
			player.refund(player.Pending.Paid)
			player.Pending = nil
		}
		return
	}

	factory := w.findBuilding(cmd.BuildingID)
	if factory == nil || factory.Owner != cmd.Owner {
		return
	}

	q := player.queue(factory.Type)
	last := len(q.Items) - 1
	if last < 0 {
		return
	}

	if last == 0 {
		player.refund(q.Paid)
		q.Progress = 0
		q.Paid = 0
	}
	q.Items = q.Items[:last]
}

func (w *World) findBuilding(id int) *Building {
	for _, b := range w.Buildings {
		if b.ID == id {
			return b
		}
	}
	return nil
}

// handleMoveCommand routes a move order through A* and applies the
// resulting path to every unit it names that the command's owner actually
// controls. If the clicked cell can't be stood on — the middle of a lake,
// or a building's footprint — nobody moves, same as a single unit would
// do. When more than one unit is named they're spread across distinct free
// cells near the target instead of all pathing to the exact same point.
func (w *World) handleMoveCommand(cmd Command) {
	goal := worldToCell(cmd.TargetX, cmd.TargetY)
	if !w.staticEnterable()(goal.X, goal.Y) {
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

	// Destinations avoid units as well as terrain, so a group doesn't get
	// sent onto cells someone is already parked on — but the group's own
	// members don't count, or ordering them to close ranks where they
	// already stand would find nowhere to put them.
	targets := nearbyCells(w.Map, goal, len(movers), w.freeFor(movers...))

	for i, u := range movers {
		u.AttackTargetID = 0 // a fresh move order cancels any attack order

		// Goal is set even if the path fails: the unit re-tries from
		// finishPath, and a destination that's crowded right now may well
		// have opened up by the time it gets close.
		u.Goal = targets[i]
		u.HasGoal = true
		u.BlockedTime = 0
		u.RetryCooldown = 0

		// Routed against terrain and buildings only — other units are
		// dealt with on contact rather than designed around up front.
		if !w.pathTo(u, targets[i], w.staticEnterable()) {
			u.Path = nil
		}
	}
}

// handleAttackCommand assigns AttackTargetID on every named unit the
// command's owner controls; updateCombat does the actual chasing/firing
// each tick. The target may be a unit or a building — they share one ID
// space — and is ignored if it doesn't exist or belongs to the same owner,
// since you can't attack your own side.
func (w *World) handleAttackCommand(cmd Command) {
	targetID, targetOwner, ok := w.targetOwner(cmd.TargetUnitID)
	if !ok || targetOwner == cmd.Owner {
		return
	}

	for _, u := range w.Units {
		if u.Owner != cmd.Owner || !containsID(cmd.UnitIDs, u.ID) {
			continue
		}
		if unitTemplates[u.Template].Weapon == "" {
			continue // unarmed units can't be given attack orders
		}

		u.AttackTargetID = targetID
		u.stop() // let updateCombat's chase() path toward the target fresh
	}
}

// targetOwner looks up who owns an entity, whether it's a unit or a
// building. Ownership lives on the concrete types rather than on
// combatTarget, since combat itself never needs to know it.
func (w *World) targetOwner(id int) (entityID, owner int, ok bool) {
	if u := w.findUnit(id); u != nil {
		return u.ID, u.Owner, true
	}
	if b := w.findBuilding(id); b != nil {
		return b.ID, b.Owner, true
	}
	return 0, 0, false
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
	Tick      int64       `json:"tick"`
	Units     []*Unit     `json:"units"`
	Buildings []*Building `json:"buildings"`

	// Economy is the viewing player's own money/power, and the structure
	// they're currently building — units and buildings are shared (there's
	// no fog of war yet, ownership is read off their Owner field), but
	// this part differs per viewer, which is why Snapshot takes one.
	Money int `json:"money"`
	Power int `json:"power"`

	// PendingType is "" when the player isn't building anything.
	PendingType     string  `json:"pendingType"`
	PendingProgress float64 `json:"pendingProgress"` // 0..1
	PendingReady    bool    `json:"pendingReady"`

	Queues []QueueState `json:"queues"`

	// Ore is how much every ore-field cell holds, in GameMap.OreCells
	// order. The coordinates ship once with the initial snapshot; this is
	// just the amounts, which is what makes sending the whole field every
	// tick cheap enough to bother with.
	Ore []int `json:"ore"`
}

// QueueState is one category's production status, keyed by the building
// type that produces it.
type QueueState struct {
	Category string  `json:"category"`
	Item     string  `json:"item"`
	Progress float64 `json:"progress"` // 0..1
	Length   int     `json:"length"`
}

// Snapshot builds the view of the world sent to one player. Everything but
// the economy block is identical for every viewer.
func (w *World) Snapshot(forOwner int) Snapshot {
	money := 0
	pendingType := ""
	pendingProgress := 0.0
	pendingReady := false
	var queues []QueueState

	if p := w.Players[forOwner]; p != nil {
		money = p.Money
		if c := p.Pending; c != nil {
			pendingType = c.Type
			pendingProgress = c.progressRatio()
			pendingReady = c.Ready
		}
		queues = p.queueStates()
	}

	return Snapshot{
		Ore:             w.OreAmounts(),
		Tick:            w.TickCount,
		Units:           w.Units,
		Buildings:       w.Buildings,
		Money:           money,
		Power:           w.NetPower(forOwner),
		PendingType:     pendingType,
		PendingProgress: pendingProgress,
		PendingReady:    pendingReady,
		Queues:          queues,
	}
}

// progressRatio reports construction completion in [0,1] — derived rather
// than stored, so Progress and BuildTime can't drift apart.
func (c *Construction) progressRatio() float64 {
	total := buildingTemplates[c.Type].BuildTime
	if c.Ready || total <= 0 {
		return 1
	}
	return c.Progress / total
}

// queueStates flattens the player's queues for the wire. Sorted by
// category so the client's UI doesn't reshuffle between ticks — Go
// randomizes map iteration order.
func (p *Player) queueStates() []QueueState {
	states := make([]QueueState, 0, len(p.Queues))

	for category, q := range p.Queues {
		if len(q.Items) == 0 {
			continue
		}

		progress := 1.0
		if total := unitTemplates[q.Items[0]].BuildTime; total > 0 {
			progress = q.Progress / total
		}

		states = append(states, QueueState{
			Category: category,
			Item:     q.Items[0],
			Progress: progress,
			Length:   len(q.Items),
		})
	}

	sort.Slice(states, func(i, j int) bool { return states[i].Category < states[j].Category })
	return states
}
