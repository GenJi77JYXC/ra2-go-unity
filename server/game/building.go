package game

import (
	"log"
	"math"
)

// lowPowerSpeedFactor slows construction and production when a player's
// net power is negative, per the learning plan's Phase 5 note. It doesn't
// stall them outright — a browned-out base still limps along.
const lowPowerSpeedFactor = 0.5

// sellRefund is the share of a structure's cost handed back when it's
// sold, at the original's rate. Half, so that shuffling a base around is a
// real cost rather than a free redesign.
const sellRefund = 0.5

// Building is a placed structure. It occupies a Width x Height block of
// cells with CellX/CellY as its lower-left corner.
type Building struct {
	ID    int    `json:"id"`
	Type  string `json:"type"`
	Owner int    `json:"owner"`
	CellX int    `json:"cellX"`
	CellY int    `json:"cellY"`
	HP    int    `json:"hp"`
	MaxHP int    `json:"maxHp"`
	Armor string `json:"-"`
	// IsBuilt is always true for anything in World.Buildings: a structure
	// only lands on the map once it's finished (see Player.Pending for the
	// build-then-place flow). It stays on the wire because the client
	// still distinguishes finished structures from the placement preview.
	IsBuilt bool `json:"isBuilt"`

	// IsPrimary marks the structure that finished units walk out of. All
	// factories of a type share one queue, so exactly one of them has to
	// be the exit — RA2 calls this the primary building and lets the
	// player move the flag around (see World.setPrimary).
	IsPrimary bool `json:"isPrimary"`
}

func newBuilding(id int, buildingType string, owner, cellX, cellY int, prebuilt bool) *Building {
	t := buildingTemplates[buildingType]
	return &Building{
		ID:      id,
		Type:    buildingType,
		Owner:   owner,
		CellX:   cellX,
		CellY:   cellY,
		HP:      t.MaxHP,
		MaxHP:   t.MaxHP,
		Armor:   t.Armor,
		IsBuilt: prebuilt,
	}
}

// occupies reports whether this building covers the given cell.
func (b *Building) occupies(x, y int) bool {
	t := buildingTemplates[b.Type]
	return x >= b.CellX && x < b.CellX+t.Width &&
		y >= b.CellY && y < b.CellY+t.Height
}

// canPlace validates a footprint: every cell must be on-map, passable
// terrain, and free of existing buildings. Units standing there are not
// checked — they're mobile, and blocking placement on a passing unit would
// make the build cursor feel unpredictable.
func (w *World) canPlace(buildingType string, cellX, cellY int) bool {
	t, ok := buildingTemplates[buildingType]
	if !ok {
		return false
	}

	for y := cellY; y < cellY+t.Height; y++ {
		for x := cellX; x < cellX+t.Width; x++ {
			if !w.Map.PassableAt(x, y) {
				return false
			}
			for _, b := range w.Buildings {
				if b.occupies(x, y) {
					return false
				}
			}
		}
	}
	return true
}

// hasPrerequisites reports whether the player owns a *completed* instance
// of every building the template requires. Structures still under
// construction deliberately don't count — otherwise queuing a power plant
// would instantly unlock everything downstream of it.
func (w *World) hasPrerequisites(owner int, prerequisites []string) bool {
	for _, required := range prerequisites {
		found := false
		for _, b := range w.Buildings {
			if b.Owner == owner && b.Type == required && b.IsBuilt {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// powerFactor is the rate multiplier from a player's power situation,
// halved while their net power is negative.
func (w *World) powerFactor(owner int) float64 {
	if w.NetPower(owner) < 0 {
		return lowPowerSpeedFactor
	}
	return 1.0
}

// productionSpeed converts a count of completed factories of one type into
// a rate multiplier for that category's shared queue. Extra factories make
// the queue faster rather than adding parallel ones, matching RA2; zero
// means production stalls entirely (every factory of that type was lost).
//
// The exact RA2 curve isn't specified in the learning plan, so this uses a
// simple +50% per additional factory with no cap.
func productionSpeed(factories int) float64 {
	if factories <= 0 {
		return 0
	}
	return 1 + 0.5*float64(factories-1)
}

// countBuildings reports how many completed structures of a type a player
// owns.
func (w *World) countBuildings(owner int, buildingType string) int {
	n := 0
	for _, b := range w.Buildings {
		if b.Owner == owner && b.Type == buildingType && b.IsBuilt {
			n++
		}
	}
	return n
}

// NetPower sums produced minus consumed power across a player's completed
// buildings. Under-construction buildings contribute nothing either way —
// a half-built power plant produces no power, and a half-built barracks
// draws none.
func (w *World) NetPower(owner int) int {
	net := 0
	for _, b := range w.Buildings {
		if b.Owner == owner && b.IsBuilt {
			net += buildingTemplates[b.Type].Power
		}
	}
	return net
}

// updateConstruction advances each player's pending structure toward
// Ready, charging them for the progress made this tick. Nothing appears on
// the map here — that happens when the player places it (see
// handlePlaceCommand).
func (w *World) updateConstruction(dt float64) {
	for _, p := range w.Players {
		c := p.Pending
		if c == nil || c.Ready {
			continue
		}

		t := buildingTemplates[c.Type]
		if t.BuildTime <= 0 {
			c.Ready = true
			continue
		}

		// Losing every Construction Yard scraps whatever was on the way
		// rather than freezing it: with nothing left to build from, the
		// order can never finish, and the original game refunds it too.
		yards := w.countBuildings(p.ID, "ConstructionYard")
		if yards == 0 {
			p.refund(c.Paid)
			p.Pending = nil
			continue
		}

		// Extra Construction Yards speed the build up, same as extra
		// factories do for units.
		speed := w.powerFactor(p.ID) * productionSpeed(yards)

		progress, ok := w.chargeProgress(p, c.Progress, &c.Paid, dt*speed, t.BuildTime, float64(t.Cost))
		if !ok {
			continue
		}

		c.Progress = progress
		if c.Progress >= t.BuildTime {
			c.Ready = true
		}
	}
}

// updateProduction advances the head of each player's category queues,
// spawning the finished unit next to one of the factories that built it.
func (w *World) updateProduction(dt float64) {
	for _, p := range w.Players {
		for category, q := range p.Queues {
			if len(q.Items) == 0 {
				continue
			}

			// Losing every factory of a type cancels that queue and
			// refunds it, for the same reason construction is scrapped
			// when the last Construction Yard falls: nothing is left to
			// produce from, so the order can never complete.
			factory := w.primaryBuilding(p.ID, category)
			if factory == nil {
				p.refund(q.Paid)
				q.Items = nil
				q.Progress = 0
				q.Paid = 0
				continue
			}

			speed := w.powerFactor(p.ID) * productionSpeed(w.countBuildings(p.ID, category))

			t := unitTemplates[q.Items[0]]
			if t.BuildTime > 0 {
				progress, ok := w.chargeProgress(p, q.Progress, &q.Paid, dt*speed, t.BuildTime, float64(t.Cost))
				if !ok {
					continue
				}
				q.Progress = progress

				if q.Progress < t.BuildTime {
					continue
				}
			}

			w.spawnUnit(q.Items[0], factory)
			q.Items = q.Items[1:]
			q.Progress = 0
			q.Paid = 0
		}
	}
}

// chargeProgress advances elapsed by dt seconds of work and bills the
// player for the fraction of totalCost that advance represents, keeping
// paid in step with progress so a cancellation can refund exactly what was
// charged. Callers scale dt by their own speed multipliers before calling.
// Returns the new elapsed time, or ok=false when the player can't afford
// this tick — in which case nothing is charged and nothing advances, so an
// over-extended base stalls mid-build instead of finishing for free.
func (w *World) chargeProgress(player *Player, elapsed float64, paid *float64, dt, totalTime, totalCost float64) (float64, bool) {
	next := math.Min(elapsed+dt, totalTime)
	owed := totalCost*(next/totalTime) - *paid

	if !player.tryPay(owed) {
		return elapsed, false
	}

	*paid += owed
	return next, true
}

// primaryBuilding returns the structure finished units of a category walk
// out of. Falls back to any completed one of that type if the flag somehow
// went missing, so production can never stall on bookkeeping alone.
func (w *World) primaryBuilding(owner int, buildingType string) *Building {
	var fallback *Building

	for _, b := range w.Buildings {
		if b.Owner != owner || b.Type != buildingType || !b.IsBuilt {
			continue
		}
		if b.IsPrimary {
			return b
		}
		if fallback == nil {
			fallback = b
		}
	}
	return fallback
}

// setPrimary moves the primary flag to one structure, clearing it from the
// owner's other structures of the same type. Ignored for a building that
// produces nothing, since the flag would mean nothing there.
func (w *World) setPrimary(owner, buildingID int) {
	target := w.findBuilding(buildingID)
	if target == nil || target.Owner != owner || !target.IsBuilt {
		return
	}
	if len(buildingTemplates[target.Type].Produces) == 0 {
		return
	}

	for _, b := range w.Buildings {
		if b.Owner == owner && b.Type == target.Type {
			b.IsPrimary = b.ID == target.ID
		}
	}
}

// ensurePrimary makes sure every category the player still has factories
// for has exactly one primary. Called after placements and after losses,
// so the first factory of a type becomes primary automatically and the
// flag moves on when the primary is destroyed.
func (w *World) ensurePrimary(owner int, buildingType string) {
	var candidate *Building

	for _, b := range w.Buildings {
		if b.Owner != owner || b.Type != buildingType || !b.IsBuilt {
			continue
		}
		if b.IsPrimary {
			return // already has one
		}
		if candidate == nil {
			candidate = b
		}
	}

	if candidate != nil {
		candidate.IsPrimary = true
	}
}

// spawnUnit places a freshly produced unit on a free cell near its
// factory, reusing the same outward ring search that spreads out group
// move orders. "Free" here includes other units: stacking a new tank on
// top of one that hasn't driven off yet would put two units in one cell,
// which the whole occupancy scheme assumes can't happen.
func (w *World) spawnUnit(unitType string, from *Building) {
	t := buildingTemplates[from.Type]
	exit := cell{X: from.CellX + t.Width, Y: from.CellY}

	free := w.freeFor()
	// nearbyCells pads its result with center when it runs out of real
	// candidates, so the predicate has to be re-checked here.
	for _, c := range nearbyCells(w.Map, exit, 8, free) {
		if !free(c.X, c.Y) {
			continue
		}
		pos := cellCenterWorld(c)
		w.addUnit(pos.X, pos.Y, from.Owner, unitType)
		return
	}
}

// sell tears a structure down and hands back part of what it cost.
//
// Teardown goes through the same path a destroyed building takes — HP to
// zero, and removeDestroyed does the rest later this tick — so clearing
// attackers and handing on the primary flag are handled in one place
// rather than two that can drift apart.
//
// Nothing is exempt. Selling the last Construction Yard, or every
// structure you own, is allowed and will lose you the match under the
// buildings rule: that's the original's behaviour, and a player who wants
// to concede that way should be able to.
func (w *World) sell(owner, buildingID int) {
	b := w.findBuilding(buildingID)
	if b == nil || b.Owner != owner || !b.IsBuilt || b.HP <= 0 {
		return
	}
	player := w.Players[owner]
	if player == nil {
		return
	}

	refund := float64(buildingTemplates[b.Type].Cost) * sellRefund
	player.refund(refund)
	b.HP = 0

	log.Printf("player %d sold building %d (%s) for %.0f", owner, b.ID, b.Type, refund)
}

func (w *World) buildingAt(x, y int) *Building {
	for _, b := range w.Buildings {
		if b.occupies(x, y) {
			return b
		}
	}
	return nil
}
