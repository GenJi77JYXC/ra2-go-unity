package game

// harvest.go is the ore economy: how much ore each field cell holds, how
// drills grow it back, and the little state machine that drives a
// harvester around without anyone commanding it.
//
// Harvesters are the first units in this project that act on their own —
// everything before them only ever moved because a player said so. The
// machine deliberately reuses the movement plumbing from occupancy.go
// rather than steering directly: each state just sets a Goal and waits to
// arrive, so being blocked, stepping aside for someone and re-routing all
// come for free.

const (
	// oreCellCapacity is what a full ore cell is worth in credits. A
	// harvester load is a few cells' worth, so a field visibly thins out
	// as it's worked.
	oreCellCapacity = 200

	// Growth: every interval, each drill tops up the nearest cell of its
	// field that isn't full. Working outward from the centre makes ore
	// creep back the way it does in the original, and it means a field
	// that's been stripped recovers from the inside out.
	//
	// One drill regrows ~12 credits/second against a harvester's ~25, so a
	// single harvester slowly outpaces its field and two strip it — the
	// field is a resource to contest, not a tap.
	oreGrowthInterval = 2.0
	oreGrowthAmount   = 25

	// A round trip is roughly: walk out, ~7s mining, walk back, 3s
	// unloading — call it 25-30 seconds for 700 credits, so one harvester
	// is worth about 25 credits/second. That's the yardstick the old
	// passiveIncome constant (20/second) used to stand in for.
	harvesterCapacity = 700
	harvestRate       = 100 // credits per second
	unloadTime        = 3.0

	// harvesterRetryInterval throttles the search when there's nowhere to
	// go — no free ore, or no refinery standing.
	harvesterRetryInterval = 1.0

	refineryType = "OreRefinery"
)

type harvestMode int

const (
	harvestIdle harvestMode = iota
	harvestToField
	harvestMining
	harvestToRefinery
	harvestUnloading
)

// HarvestState hangs off the units that have one — a plain nil on
// everything else, which is why it's a pointer rather than five more
// fields on every infantryman.
type HarvestState struct {
	Mode  harvestMode
	Cargo int

	// Target is the cell this harvester is working: the ore cell it mines,
	// or the cell beside a refinery it unloads from. Checking the unit is
	// actually standing there is what keeps the machine honest when a
	// player drags the harvester away mid-job.
	Target cell

	Timer float64 // unload countdown
	Retry float64 // throttles the search while there's nowhere to go
	carry float64 // sub-credit remainder, same idea as Player.fraction
}

// fillOre stocks every field cell to capacity. Called once at world
// creation — the amounts live here rather than on GameMap because they
// change, and GameMap is read without synchronisation.
func (w *World) fillOre() {
	w.ore = make(map[cell]int, len(w.Map.oreCells))
	for _, c := range w.Map.oreCells {
		w.ore[c] = oreCellCapacity
	}
}

// OreAmounts reports every field cell's contents in GameMap.OreCells
// order. Clients get the coordinates once in the initial snapshot and
// just this array afterwards, which is what keeps a per-frame broadcast
// of the whole field affordable.
func (w *World) OreAmounts() []int {
	out := make([]int, len(w.Map.oreCells))
	for i, c := range w.Map.oreCells {
		out[i] = w.ore[c]
	}
	return out
}

func (w *World) growOre(dt float64) {
	if w.ore == nil {
		w.fillOre() // a World built by a struct literal, as the tests do
	}

	w.oreGrowth += dt
	if w.oreGrowth < oreGrowthInterval {
		return
	}
	w.oreGrowth -= oreGrowthInterval

	for _, d := range w.Map.drills {
		for _, c := range d.field {
			if w.ore[c] >= oreCellCapacity {
				continue
			}
			w.ore[c] = minInt(w.ore[c]+oreGrowthAmount, oreCellCapacity)
			break // nearest unfilled cell only, so ore creeps outward
		}
	}
}

func (w *World) updateHarvesters(dt float64) {
	for _, u := range w.Units {
		if u.Harvest != nil {
			w.updateHarvester(u, dt)
		}
	}
}

func (w *World) updateHarvester(u *Unit, dt float64) {
	h := u.Harvest

	if h.Retry > 0 {
		h.Retry -= dt
	}

	switch h.Mode {
	case harvestIdle:
		// A player-issued move order leaves HasGoal set; staying out of
		// the way until it's done is what lets a harvester be driven
		// manually and then go back to work on its own.
		if h.Retry > 0 || u.HasGoal || !u.idle() {
			return
		}
		if h.Cargo >= harvesterCapacity {
			w.sendToRefinery(u)
			return
		}
		w.sendToOre(u)

	case harvestToField:
		if u.Cell == h.Target {
			h.Mode = harvestMining
			return
		}
		if u.idle() && !u.HasGoal {
			u.pauseHarvest() // never made it — pick a new cell
		}

	case harvestMining:
		w.mine(u, dt)

	case harvestToRefinery:
		if u.Cell == h.Target {
			h.Mode = harvestUnloading
			h.Timer = unloadTime
			return
		}
		if u.idle() && !u.HasGoal {
			u.pauseHarvest()
		}

	case harvestUnloading:
		if u.commanded() || u.Cell != h.Target {
			u.pauseHarvest() // driven off the dock mid-unload
			return
		}
		h.Timer -= dt
		if h.Timer > 0 {
			return
		}
		if p := w.Players[u.Owner]; p != nil {
			p.refund(float64(h.Cargo))
		}
		h.Cargo = 0
		h.Mode = harvestIdle
	}
}

// mine takes ore out of the cell the harvester is standing on. Rate is
// converted to whole credits through a carry, so a tick's worth of mining
// isn't rounded away like it would be by an int conversion.
func (w *World) mine(u *Unit, dt float64) {
	h := u.Harvest

	// Being under orders has to be checked before anything else here, not
	// just on arrival: mining a cell dry calls sendToOre, which would hand
	// the unit a brand new destination and quietly discard the one the
	// player just gave it.
	if u.commanded() || u.Cell != h.Target {
		u.pauseHarvest()
		return
	}

	h.carry += harvestRate * dt
	want := int(h.carry)
	if want <= 0 {
		return
	}
	h.carry -= float64(want)

	want = minInt(want, harvesterCapacity-h.Cargo)
	want = minInt(want, w.ore[h.Target])

	if want > 0 {
		h.Cargo += want
		w.ore[h.Target] -= want
	}

	if h.Cargo >= harvesterCapacity {
		w.sendToRefinery(u)
		return
	}
	if w.ore[h.Target] <= 0 {
		// Straight on to the next cell — stripping one is the normal course
		// of work, not a failure, so it shouldn't pay the retry delay.
		// sendToOre falls back to halting if there's genuinely nowhere left.
		w.sendToOre(u)
	}
}

// sendToOre heads for the nearest field cell that still holds ore and
// isn't already being worked. A harvester that can't find one but is
// carrying something goes and banks it instead of waiting on an exhausted
// field.
func (w *World) sendToOre(u *Unit) bool {
	free := w.freeFor(u)

	var best cell
	bestDist := 0
	found := false

	for _, c := range w.Map.oreCells {
		if w.ore[c] <= 0 || !free(c.X, c.Y) {
			continue
		}
		if d := manhattan(u.Cell, c); !found || d < bestDist {
			best, bestDist, found = c, d, true
		}
	}

	if !found {
		if u.Harvest.Cargo > 0 {
			return w.sendToRefinery(u)
		}
		u.pauseHarvest()
		return false
	}

	return w.dispatch(u, best, harvestToField, harvestMining)
}

// sendToRefinery heads for a free cell beside the nearest refinery this
// player owns. The footprint itself isn't enterable, so — exactly like
// chasing a building in combat — the harvester aims for the closest cell
// it can actually stand on.
func (w *World) sendToRefinery(u *Unit) bool {
	var nearest *Building
	bestDist := 0

	for _, b := range w.Buildings {
		if b.Owner != u.Owner || b.Type != refineryType || !b.IsBuilt {
			continue
		}
		bx, by := b.Position()
		d := manhattan(u.Cell, worldToCell(bx, by))
		if nearest == nil || d < bestDist {
			nearest, bestDist = b, d
		}
	}

	if nearest == nil {
		u.pauseHarvest() // refinery died while it was loaded; hold the cargo
		return false
	}

	bx, by := nearest.Position()
	free := w.freeFor(u)
	dock := nearbyCells(w.Map, worldToCell(bx, by), 1, free)[0]
	if !free(dock.X, dock.Y) {
		u.pauseHarvest()
		return false
	}

	return w.dispatch(u, dock, harvestToRefinery, harvestUnloading)
}

// dispatch sends the harvester to a cell, or switches straight to the
// arrival state when it's already standing there.
func (w *World) dispatch(u *Unit, target cell, travelling, arrived harvestMode) bool {
	h := u.Harvest
	h.Target = target

	if u.Cell == target {
		h.Mode = arrived
		if arrived == harvestUnloading {
			h.Timer = unloadTime
		}
		return true
	}

	u.Goal = target
	u.HasGoal = true
	if !w.pathTo(u, target, w.staticEnterable()) {
		u.stop() // the goal on the line above is ours to take back
		u.pauseHarvest()
		return false
	}

	h.Mode = travelling
	return true
}

// commanded reports whether the unit has somewhere to be. A harvester at
// work sits still between hops, so anything here means an order arrived
// from outside — and orders from outside win.
func (u *Unit) commanded() bool {
	return u.HasGoal || !u.idle()
}

// pauseHarvest drops the current job and waits a beat before looking for
// another, so a harvester with nowhere to go doesn't re-scan the whole
// field every tick.
//
// It deliberately leaves movement alone. Most of the ways a job ends are
// "the player took the wheel" — dragging the harvester off its ore cell or
// off the refinery dock — and cancelling its Path there would undo the
// very order that interrupted it. The one case that does need the unit
// stopped is a path this file asked for and didn't get, which dispatch
// handles itself.
func (u *Unit) pauseHarvest() {
	u.Harvest.Mode = harvestIdle
	u.Harvest.Retry = harvesterRetryInterval
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
