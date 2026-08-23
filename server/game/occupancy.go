package game

// occupancy.go is the dynamic half of collision: which cells are taken,
// and what a unit does when the one it wants is among them.
//
// RA2 is a grid game — units occupy cells, block each other, and re-route
// when blocked — so this is built on the same cell grid A* already uses
// rather than on continuous-space steering (separation forces, velocity
// obstacles). Those are approximations of a grid rule the game actually
// has; the trade-off is that units queue up in narrow terrain instead of
// piling through it, which is what the original does too.

const (
	// blockedWaitTime is how long a unit stands still hoping the blocker
	// moves on before it does anything about it. Most blockages are
	// momentary — someone crossing in front — and reacting to every one of
	// them would have units re-pathing constantly for no reason.
	blockedWaitTime = 0.5

	// blockedRetryInterval throttles the (comparatively expensive) re-path
	// attempts once a unit has decided it really is stuck.
	blockedRetryInterval = 0.5

	// blockedGiveUpTime is when a unit abandons the order outright, so a
	// permanently walled-off destination doesn't leave it shoving forever.
	blockedGiveUpTime = 3.0

	// yieldHoldTime is how long a unit that has stepped aside waits before
	// resuming its own trip. Without it the whole mechanism is pointless:
	// a unit that has just cleared a cell re-paths straight back through
	// it on the next tick, re-reserving it before the unit it made room
	// for can move in, and the two oscillate until they both give up.
	// It only has to outlast the hop it's making room for.
	yieldHoldTime = 1.0
)

// canEnter answers "may a unit step into this cell?". Which obstacles it
// accounts for is the caller's choice: World.staticEnterable covers the
// things that never move (terrain and buildings), World.freeFor adds the
// cells other units are standing on or have reserved.
type canEnter func(x, y int) bool

// staticEnterable blocks on terrain and buildings only. This is what a
// fresh move order paths against — pathing around other units up front
// would be wasted work, since by the time the unit arrives they've moved.
// Units are dealt with as they're actually met, in blocked().
func (w *World) staticEnterable() canEnter {
	return func(x, y int) bool {
		return w.Map.PassableAt(x, y) && w.buildingAt(x, y) == nil
	}
}

// freeFor is staticEnterable plus the occupancy table, treating the listed
// units' own cells as free so a unit is never blocked by itself — or, for
// a group move order, by the rest of its own group standing on the
// destination.
func (w *World) freeFor(movers ...*Unit) canEnter {
	static := w.staticEnterable()

	self := make(map[int]bool, len(movers))
	for _, u := range movers {
		self[u.ID] = true
	}

	return func(x, y int) bool {
		if !static(x, y) {
			return false
		}
		id, taken := w.occupied[cell{X: x, Y: y}]
		return !taken || self[id]
	}
}

// occupy claims a cell for a unit. The map is created on demand so a World
// assembled by a struct literal (as the tests do) needs no extra setup.
func (w *World) occupy(c cell, id int) {
	if w.occupied == nil {
		w.occupied = map[cell]int{}
	}
	w.occupied[c] = id
}

// release clears a cell only if the given unit is the one holding it, so a
// unit letting go of the cell behind it can't evict whoever has already
// moved in.
func (w *World) release(c cell, id int) {
	if w.occupied[c] == id {
		delete(w.occupied, c)
	}
}

// unitAt returns whoever holds a cell — which may be a unit that has only
// reserved it and is still walking in.
func (w *World) unitAt(c cell) *Unit {
	id, ok := w.occupied[c]
	if !ok {
		return nil
	}
	return w.findUnit(id)
}

// reserve claims the cell a unit is about to step into.
//
// It checks only the cell being *entered*, never the one being left. That
// asymmetry is load-bearing: canPlace deliberately ignores units, so a
// building can go up on top of one, leaving it standing somewhere it could
// never have walked into. It still has to be able to walk out.
func (w *World) reserve(u *Unit, c cell) bool {
	if !w.freeFor(u)(c.X, c.Y) {
		return false
	}
	w.occupy(c, u.ID)
	return true
}

// arrive completes a hop: the unit lets go of the cell behind it and the
// reserved cell becomes its own. Until this happens the unit holds *both*,
// so nobody can slip through the gap it is halfway across.
func (w *World) arrive(u *Unit) {
	w.release(u.Cell, u.ID)
	u.Cell = u.NextCell
	u.InTransit = false
	u.BlockedTime = 0
	u.RetryCooldown = 0
}

// placeUnit registers a unit's starting cell.
func (w *World) placeUnit(u *Unit) {
	u.Cell = worldToCell(u.X, u.Y)
	w.occupy(u.Cell, u.ID)
}

// clearUnit drops every cell a unit holds, including one it had reserved
// but never reached. Called when it's destroyed — a corpse holding a
// reservation would wall the map off permanently.
func (w *World) clearUnit(u *Unit) {
	w.release(u.Cell, u.ID)
	if u.InTransit {
		w.release(u.NextCell, u.ID)
	}
}

// blocked handles a failed reservation. The escalation is deliberate:
// jumping straight to re-pathing (or to shoving the blocker aside) makes
// units shuffle around each other over blockages that would have cleared
// on their own within a few ticks.
func (w *World) blocked(u *Unit, want cell, dt float64) {
	u.BlockedTime += dt

	// 4. Nothing worked for long enough that nothing is going to. Drop the
	// order rather than retry forever.
	if u.BlockedTime >= blockedGiveUpTime {
		u.stop()
		return
	}

	// 1. Wait — the blocker is probably just passing through.
	if u.BlockedTime < blockedWaitTime {
		return
	}

	if u.RetryCooldown > 0 {
		u.RetryCooldown -= dt
		return
	}
	u.RetryCooldown = blockedRetryInterval

	// 2. Look for a route that goes around everyone, not just the terrain.
	if u.HasGoal && w.pathTo(u, u.Goal, w.freeFor(u)) {
		return
	}

	blocker := w.unitAt(want)
	if blocker == nil || blocker.ID == u.ID {
		return // the map edge or a building: nothing to negotiate with
	}

	// 3a. Head-on: each wants the cell the other is holding, so no amount
	// of waiting or re-pathing helps. Break the tie by ID — arbitrary, but
	// consistent, which is the point: exactly one of the two yields, and
	// the same one every tick, so they don't both dodge into each other.
	if blocker.wants(u.Cell) {
		if u.ID > blocker.ID {
			w.stepAside(u, blocker)
		}
		return
	}

	// 3b. The blocker is simply parked in the way. Ask it to shuffle over,
	// which is what the original game does; because it keeps its own Goal,
	// it resumes whatever it was doing afterwards.
	if blocker.idle() {
		w.stepAside(blocker, u)
	}
}

// stepAside sends a unit one cell out of the way to let requester through.
//
// Which neighbour it picks is what makes this work. Cells still ahead on
// the requester's route are taken only as a last resort: shuffling forward
// along the very path being cleared just recreates the standoff one square
// on, and in a dead end it does that over and over. Anything off to the
// side — a passing bay, open ground — actually settles it.
func (w *World) stepAside(u, requester *Unit) bool {
	if u.InTransit {
		return false
	}

	free := w.freeFor(u)
	var forward *cell

	for _, n := range neighbors4(u.Cell) {
		if n == requester.Cell || !free(n.X, n.Y) {
			continue
		}
		if requester.willEnter(n) {
			if forward == nil {
				out := n
				forward = &out
			}
			continue
		}

		u.divertTo(n)
		return true
	}

	if forward != nil {
		u.divertTo(*forward)
		return true
	}
	return false
}

// divertTo replaces the unit's route with a single hop, leaving Goal
// alone — finishPath re-plans from wherever it ends up, once HoldTime has
// given the other unit its chance to get past.
func (u *Unit) divertTo(c cell) {
	u.Path = []Point{cellCenterWorld(c)}
	u.BlockedTime = 0
	u.RetryCooldown = 0
	u.HoldTime = yieldHoldTime
}

// wants reports whether the unit is about to step into a given cell.
func (u *Unit) wants(c cell) bool {
	if u.InTransit {
		return u.NextCell == c
	}
	if len(u.Path) == 0 {
		return false
	}
	return worldToCell(u.Path[0].X, u.Path[0].Y) == c
}

// willEnter reports whether a cell is anywhere ahead on the unit's route,
// not just the next step.
func (u *Unit) willEnter(c cell) bool {
	if u.InTransit && u.NextCell == c {
		return true
	}
	for _, p := range u.Path {
		if worldToCell(p.X, p.Y) == c {
			return true
		}
	}
	return false
}

// idle reports whether the unit is standing still with nowhere to be, and
// so can afford to give up its cell.
func (u *Unit) idle() bool {
	return !u.InTransit && len(u.Path) == 0
}

// stop abandons the current movement order. Both halves matter: clearing
// Path alone would leave HasGoal set, and finishPath would immediately
// re-issue the very order that just failed.
func (u *Unit) stop() {
	u.Path = nil
	u.HasGoal = false
	u.BlockedTime = 0
	u.RetryCooldown = 0
	u.HoldTime = 0
}
