package game

import "testing"

// totalOre is what's left across the whole map, for tests that care about
// the direction a number moved rather than its exact value.
func (w *World) totalOre() int {
	sum := 0
	for _, c := range w.Map.oreCells {
		sum += w.ore[c]
	}
	return sum
}

// harvestWorld sets player 1 up with a refinery just west of the northern
// ore field, and one harvester parked beside it.
func harvestWorld() (*World, *Unit) {
	w := newTestWorld()
	w.fillOre()
	w.addBuilding(100, "OreRefinery", 1, 1, 0) // covers x1..3, y0..2
	return w, w.addUnitAt(cell{X: 4, Y: 1}, 1, "Harvester")
}

func TestOreFieldsExistAndStartFull(t *testing.T) {
	w := newTestWorld()
	w.fillOre()

	cells := w.Map.OreCells()
	if len(cells) == 0 {
		t.Fatal("the map has no ore field at all")
	}
	if got, want := w.totalOre(), len(cells)*oreCellCapacity; got != want {
		t.Errorf("starting ore = %d, want %d", got, want)
	}
	if got := len(w.OreAmounts()); got != len(cells) {
		t.Errorf("OreAmounts has %d entries, OreCells has %d — the wire "+
			"format pairs them by index", got, len(cells))
	}

	// Ore has to be reachable: a field walled off by terrain would make the
	// whole economy unplayable, and it's the sort of thing that only breaks
	// when someone edits the map.
	enterable := w.staticEnterable()
	for _, c := range cells {
		if !enterable(c.X, c.Y) {
			t.Errorf("ore cell %v can't be stood on", c)
		}
	}
}

// The whole point of the feature: a harvester nobody commands should go
// out, fill up, come home and turn ore into credits.
func TestHarvesterRunsACompleteCycle(t *testing.T) {
	w, h := harvestWorld()

	before := w.Players[1].Money
	oreBefore := w.totalOre()

	runTicks(t, w, 800) // 40s — comfortably more than one round trip

	if w.Players[1].Money <= before {
		t.Fatalf("money went %d -> %d; the harvester never banked anything "+
			"(mode %d, cargo %d, at %v)",
			before, w.Players[1].Money, h.Harvest.Mode, h.Harvest.Cargo, h.Cell)
	}
	if w.totalOre() >= oreBefore {
		t.Errorf("total ore went %d -> %d; credits appeared without being mined",
			oreBefore, w.totalOre())
	}
}

// Income is now entirely earned. A player with no refinery must not drift
// upward on their own — that regression would hide a broken harvester.
func TestNoIncomeWithoutHarvesting(t *testing.T) {
	w := newTestWorld()
	before := w.Players[1].Money

	runTicks(t, w, 400)

	if w.Players[1].Money != before {
		t.Errorf("money moved %d -> %d with nothing harvesting it",
			before, w.Players[1].Money)
	}
}

// A refinery is useless without something to fill it, and at 1400 credits
// a player could easily not afford both — so it arrives with a harvester,
// as in the original.
func TestRefineryArrivesWithAHarvester(t *testing.T) {
	w := newTestWorld()
	w.Players[1].Pending = &Construction{Type: "OreRefinery", Ready: true}

	w.HandleCommand(Command{Type: "place", Owner: 1, CellX: 1, CellY: 0})

	harvesters := 0
	for _, u := range w.Units {
		if u.Owner == 1 && u.Harvest != nil {
			harvesters++
		}
	}
	if harvesters != 1 {
		t.Fatalf("placing a refinery produced %d harvesters, want 1", harvesters)
	}
}

// Drills top up the nearest unfilled cell of their field, so a stripped
// field recovers from the middle outward.
func TestDrillsRegrowOreFromTheCentreOut(t *testing.T) {
	w := newTestWorld()
	w.fillOre()

	drill := w.Map.drills[0]
	nearest, farthest := drill.field[0], drill.field[len(drill.field)-1]

	for _, c := range drill.field {
		w.ore[c] = 0
	}

	// Long enough for several top-ups, but nowhere near enough to refill
	// the whole field — which is what makes the ordering observable.
	runTicks(t, w, int(4*oreGrowthInterval/tickSeconds))

	if w.ore[nearest] <= 0 {
		t.Errorf("the cell next to the drill %v got nothing back", nearest)
	}
	if w.ore[farthest] > 0 {
		t.Errorf("the far edge %v refilled before the middle was full", farthest)
	}
	if w.ore[nearest] > oreCellCapacity {
		t.Errorf("cell %v overfilled to %d, cap is %d", nearest, w.ore[nearest], oreCellCapacity)
	}
}

// Losing the refinery mid-run must not strand or crash the harvester: it
// keeps what it's carrying and goes back to work once there's somewhere to
// deliver again.
func TestHarvesterSurvivesLosingItsRefinery(t *testing.T) {
	w, h := harvestWorld()

	runTicks(t, w, 200) // let it get out and start filling up

	w.Buildings = nil
	runTicks(t, w, 200)

	if h.HP <= 0 {
		t.Fatal("harvester died when its refinery did")
	}

	before := w.Players[1].Money
	w.addBuilding(101, "OreRefinery", 1, 1, 0)
	runTicks(t, w, 800)

	if w.Players[1].Money <= before {
		t.Errorf("money stuck at %d after the refinery was rebuilt; the "+
			"harvester never resumed (mode %d, cargo %d)",
			before, h.Harvest.Mode, h.Harvest.Cargo)
	}
}

// Several harvesters on one field is the normal case, and they must not
// end up working the same cell — runTicks already fails on two units
// sharing one, so this is really checking they all stay productive.
func TestSeveralHarvestersShareAField(t *testing.T) {
	w := newTestWorld()
	w.fillOre()
	w.addBuilding(100, "OreRefinery", 1, 1, 0)

	for _, c := range []cell{{X: 4, Y: 0}, {X: 4, Y: 1}, {X: 4, Y: 2}} {
		w.addUnitAt(c, 1, "Harvester")
	}

	before := w.Players[1].Money
	runTicks(t, w, 800)

	earned := w.Players[1].Money - before
	if earned < harvesterCapacity {
		t.Errorf("three harvesters banked only %d credits in 40s; one alone "+
			"should manage %d", earned, harvesterCapacity)
	}
}

// A player order takes priority over the harvester's own plans, and the
// harvester picks its job back up once it has obeyed.
func TestManualOrderInterruptsThenResumesHarvesting(t *testing.T) {
	w, h := harvestWorld()

	runTicks(t, w, 100) // let it settle into a job first
	if h.Harvest.Mode == harvestIdle {
		t.Fatal("harvester never started working, so there is nothing to interrupt")
	}

	parking := cell{X: 1, Y: 8}
	w.order(h, parking)

	// Checked every tick rather than at the end, because obeying and then
	// going back to work is the whole point — sampling afterwards would
	// just as likely catch it already back on the ore.
	obeyed := false
	for i := 0; i < 400 && !obeyed; i++ {
		w.Tick(tickSeconds)
		obeyed = h.Cell == parking
	}
	if !obeyed {
		t.Fatalf("harvester never reached %v, sitting at %v (mode %d)",
			parking, h.Cell, h.Harvest.Mode)
	}

	before := w.Players[1].Money
	runTicks(t, w, 1200)

	if w.Players[1].Money <= before {
		t.Errorf("money stuck at %d after the harvester was parked; it never "+
			"went back to work (mode %d, at %v)",
			before, h.Harvest.Mode, h.Cell)
	}
}

// The narrow window that makes Unit.commanded worth having: a move order
// arrives while the harvester is mining, and the cell runs dry before the
// unit has physically left it. Mining is evaluated after movement, so at
// that moment the harvester is still standing on its target — and the
// "cell exhausted, find another" path would hand it a fresh destination,
// silently throwing away the order the player just gave.
func TestOrderSurvivesTheCellRunningDryTheSameTick(t *testing.T) {
	w, h := harvestWorld()

	for i := 0; i < 400 && h.Harvest.Mode != harvestMining; i++ {
		w.Tick(tickSeconds)
	}
	if h.Harvest.Mode != harvestMining {
		t.Fatal("harvester never reached the mining state")
	}

	// One tick's worth of ore left, so the cell empties immediately after
	// the order lands but before the unit can walk off it.
	w.ore[h.Harvest.Target] = 1

	parking := cell{X: 1, Y: 8}
	w.order(h, parking)
	runTicks(t, w, 3)

	if !h.HasGoal || h.Goal != parking {
		t.Fatalf("the move order was discarded: goal is %v (set: %v), want %v",
			h.Goal, h.HasGoal, parking)
	}
}
