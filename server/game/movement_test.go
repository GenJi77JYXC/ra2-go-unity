package game

import "testing"

// tickSeconds is one simulation step, matching what the room loop feeds
// World.Tick.
var tickSeconds = TickInterval.Seconds()

// laneWorld builds a world on a hand-made map: a one-cell-wide corridor
// along y=1, with a passing bay above each x in bays. A corridor is the
// only way to test blocked-unit handling honestly — on open ground a unit
// can always route around, so the tests would pass without any of this
// machinery working.
func laneWorld(width int, bays ...int) *World {
	m := &GameMap{Width: width, Height: 3, Tiles: make([][]Tile, 3)}
	for y := 0; y < 3; y++ {
		m.Tiles[y] = make([]Tile, width)
		for x := 0; x < width; x++ {
			m.Tiles[y][x] = Tile{Type: Cliff, Passable: false}
		}
	}
	for x := 0; x < width; x++ {
		m.Tiles[1][x] = Tile{Type: Grass, Passable: true}
	}
	for _, x := range bays {
		m.Tiles[2][x] = Tile{Type: Grass, Passable: true}
	}

	return &World{
		Map:      m,
		Players:  map[int]*Player{1: newPlayer(1), 2: newPlayer(2)},
		occupied: map[cell]int{},
	}
}

// addUnitAt drops a unit on the center of a cell, the way every real spawn
// path does.
func (w *World) addUnitAt(c cell, owner int, template string) *Unit {
	p := cellCenterWorld(c)
	return w.addUnit(p.X, p.Y, owner, template)
}

// order issues a move command for one unit to the center of a cell.
func (w *World) order(u *Unit, c cell) {
	p := cellCenterWorld(c)
	w.HandleCommand(Command{Type: "move", Owner: u.Owner, UnitIDs: []int{u.ID}, TargetX: p.X, TargetY: p.Y})
}

// runTicks advances the world, failing if two units ever end up reading as
// being in the same cell. That invariant is the whole point of the
// feature, so it's checked continuously rather than only at the end.
func runTicks(t *testing.T, w *World, n int) {
	t.Helper()

	for i := 0; i < n; i++ {
		w.Tick(tickSeconds)

		seen := map[cell]int{}
		for _, u := range w.Units {
			at := worldToCell(u.X, u.Y)
			if other, clash := seen[at]; clash {
				t.Fatalf("tick %d: units %d and %d both in cell %v", i, other, u.ID, at)
			}
			seen[at] = u.ID
		}
	}
}

// A group order spreads its destinations, but units still get in each
// other's way en route. What has to hold is that they never share a cell,
// they all come to rest, and they end up where they were sent — allowing
// one cell of slack, since a unit that has already parked can be shoved
// off its spot by a latecomer squeezing past and, with no Goal left of its
// own, stays where it was pushed.
func TestGroupMoveSettlesWithoutOverlapping(t *testing.T) {
	w := newTestWorld()

	var ids []int
	for x := 0; x < 5; x++ {
		ids = append(ids, w.addUnitAt(cell{X: x, Y: 0}, 1, "Tank").ID)
	}

	w.HandleCommand(Command{Type: "move", Owner: 1, UnitIDs: ids, TargetX: 10.5, TargetY: 0.5})

	goals := map[int]cell{}
	for _, u := range w.Units {
		if !u.HasGoal {
			t.Fatalf("unit %d got no goal from the move order", u.ID)
		}
		goals[u.ID] = u.Goal
	}
	if len(goals) != len(ids) {
		t.Fatalf("group order handed out %d distinct goals, want %d", len(goals), len(ids))
	}

	runTicks(t, w, 300)

	for _, u := range w.Units {
		if u.InTransit || len(u.Path) > 0 {
			t.Errorf("unit %d never came to rest (at %v)", u.ID, u.Cell)
		}

		want := goals[u.ID]
		if absInt(u.Cell.X-want.X) > 1 || absInt(u.Cell.Y-want.Y) > 1 {
			t.Errorf("unit %d stopped at %v, too far from %v", u.ID, u.Cell, want)
		}
	}
}

func TestUnitsRouteAroundBuildings(t *testing.T) {
	w := newTestWorld()
	w.addBuilding(100, "ConstructionYard", 1, 2, 2) // covers x2..4, y2..4

	u := w.addUnitAt(cell{X: 1, Y: 3}, 1, "Tank")
	w.order(u, cell{X: 5, Y: 3})

	for i := 0; i < 300; i++ {
		w.Tick(tickSeconds)
		if w.buildingAt(u.Cell.X, u.Cell.Y) != nil {
			t.Fatalf("tick %d: unit walked into the building at %v", i, u.Cell)
		}
	}

	if want := (cell{X: 5, Y: 3}); u.Cell != want {
		t.Errorf("unit ended at %v, want %v", u.Cell, want)
	}
}

// A building can be placed on top of a unit — canPlace deliberately
// ignores units — so a unit can be standing somewhere it could never have
// walked into. It still has to be able to walk out.
func TestUnitUnderABuildingCanLeave(t *testing.T) {
	w := newTestWorld()
	u := w.addUnitAt(cell{X: 2, Y: 3}, 1, "Tank")
	w.addBuilding(100, "ConstructionYard", 1, 2, 2) // swallows the unit's cell

	w.order(u, cell{X: 8, Y: 3})
	runTicks(t, w, 300)

	if want := (cell{X: 8, Y: 3}); u.Cell != want {
		t.Fatalf("trapped unit ended at %v, want %v", u.Cell, want)
	}
}

func TestIdleBlockerStepsAside(t *testing.T) {
	w := laneWorld(6, 3) // corridor along y=1, one bay above x=3
	blocker := w.addUnitAt(cell{X: 3, Y: 1}, 1, "Tank")
	mover := w.addUnitAt(cell{X: 0, Y: 1}, 1, "Tank")

	w.order(mover, cell{X: 5, Y: 1})
	runTicks(t, w, 300)

	if want := (cell{X: 5, Y: 1}); mover.Cell != want {
		t.Errorf("mover ended at %v, want %v", mover.Cell, want)
	}
	if blocker.Cell == (cell{X: 3, Y: 1}) {
		t.Errorf("blocker never moved out of the corridor")
	}
}

// Two units walking into each other in a corridor: waiting can't resolve
// it and neither can re-pathing, so one of them has to yield. Whichever
// one it is, both journeys have to complete.
func TestHeadOnCorridorBothArrive(t *testing.T) {
	w := laneWorld(5, 3) // bay above x=3, where the yielding unit stands
	east := w.addUnitAt(cell{X: 1, Y: 1}, 1, "Tank")
	west := w.addUnitAt(cell{X: 3, Y: 1}, 1, "Tank")

	w.order(east, cell{X: 4, Y: 1})
	w.order(west, cell{X: 0, Y: 1})

	runTicks(t, w, 400)

	if want := (cell{X: 4, Y: 1}); east.Cell != want {
		t.Errorf("eastbound unit ended at %v, want %v", east.Cell, want)
	}
	if want := (cell{X: 0, Y: 1}); west.Cell != want {
		t.Errorf("westbound unit ended at %v, want %v", west.Cell, want)
	}
}

// A destination that stays blocked has to be abandoned, not retried
// forever. The corridor here is plugged by two parked units: the first
// can't step aside because the second is right behind it, so nothing the
// mover does will ever work.
func TestPermanentlyBlockedUnitGivesUp(t *testing.T) {
	w := laneWorld(4) // cells (0,1)..(3,1), no bays
	mover := w.addUnitAt(cell{X: 0, Y: 1}, 1, "Tank")
	w.addUnitAt(cell{X: 1, Y: 1}, 1, "Tank")
	w.addUnitAt(cell{X: 2, Y: 1}, 1, "Tank")

	w.order(mover, cell{X: 3, Y: 1})
	if !mover.HasGoal || len(mover.Path) == 0 {
		t.Fatalf("expected the order to be accepted: the destination itself is free")
	}

	// Still trying, right up to the limit.
	runTicks(t, w, int(blockedGiveUpTime/tickSeconds)-4)
	if !mover.HasGoal {
		t.Fatalf("unit gave up early, before the %.1fs limit", blockedGiveUpTime)
	}

	runTicks(t, w, 40)
	if mover.HasGoal || len(mover.Path) != 0 {
		t.Errorf("unit still chasing a blocked cell after %.1fs", blockedGiveUpTime)
	}
	if mover.Cell != (cell{X: 0, Y: 1}) {
		t.Errorf("unit ended at %v, want it to still be at its start", mover.Cell)
	}
}

// Destroyed units must release the cells they hold — including one merely
// reserved mid-hop — or a wreck walls the map off permanently.
func TestDestroyedUnitReleasesItsCells(t *testing.T) {
	w := laneWorld(4)
	victim := w.addUnitAt(cell{X: 0, Y: 1}, 1, "Tank")

	w.order(victim, cell{X: 3, Y: 1})
	w.Tick(tickSeconds)

	if !victim.InTransit {
		t.Fatalf("expected the unit to be mid-hop after one tick")
	}
	held := []cell{victim.Cell, victim.NextCell}

	victim.HP = 0
	w.Tick(tickSeconds)

	for _, c := range held {
		if _, taken := w.occupied[c]; taken {
			t.Errorf("cell %v still reserved by a destroyed unit", c)
		}
	}
}

// Chasing changed shape when buildings and units became obstacles: the
// target's own cell is no longer somewhere the attacker can path to, so
// chase has to aim for the nearest cell it can actually stand on and let
// weapon range close the gap. If that regressed, attackers would simply
// never arrive.
func TestAttackerClosesOnABuilding(t *testing.T) {
	w := newTestWorld()
	target := w.addBuilding(100, "ConstructionYard", 2, 10, 10)
	attacker := w.addUnitAt(cell{X: 1, Y: 10}, 1, "Tank")

	w.HandleCommand(Command{
		Type: "attack", Owner: 1,
		UnitIDs: []int{attacker.ID}, TargetUnitID: target.ID,
	})

	before := target.HP
	runTicks(t, w, 400)

	if target.HP >= before {
		t.Fatalf("attacker never damaged the building (HP %d -> %d, ended at %v)",
			before, target.HP, attacker.Cell)
	}
	if w.buildingAt(attacker.Cell.X, attacker.Cell.Y) != nil {
		t.Errorf("attacker ended up inside the building at %v", attacker.Cell)
	}
}

// The same for a mobile target, which additionally holds the cell it
// stands on — so the attacker has to settle for a neighbouring one.
func TestAttackerClosesOnAUnit(t *testing.T) {
	w := newTestWorld()
	prey := w.addUnitAt(cell{X: 12, Y: 10}, 2, "Tank")
	attacker := w.addUnitAt(cell{X: 1, Y: 10}, 1, "Tank")

	w.HandleCommand(Command{
		Type: "attack", Owner: 1,
		UnitIDs: []int{attacker.ID}, TargetUnitID: prey.ID,
	})

	before := prey.HP
	runTicks(t, w, 400)

	if prey.HP >= before {
		t.Fatalf("attacker never damaged the target (HP %d -> %d, ended at %v)",
			before, prey.HP, attacker.Cell)
	}
}
