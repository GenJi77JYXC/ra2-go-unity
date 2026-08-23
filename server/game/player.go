package game

import "math"

const (
	startingMoney = 5000

	// passiveIncome stands in for ore harvesting, which no phase of the
	// learning plan actually introduces — without some income you'd hit a
	// hard wall a few structures into a session with no way to recover.
	// Real RA2 economy (refineries, harvesters, ore fields) would replace
	// this outright.
	passiveIncome = 20 // credits per second
)

// Construction is a structure being built but not yet on the map. RA2
// builds first and places second: the credits drain while it's in this
// state, and only once Ready does the player get a placement cursor. It
// has no cell coordinates until then, which is exactly why it can't live
// in World.Buildings.
type Construction struct {
	Type     string
	Progress float64 // seconds elapsed
	Paid     float64 // charged so far, refunded in full on cancel
	Ready    bool
}

// ProductionQueue is one category's worth of queued units — Items[0] is
// the one currently building, and the only one being charged for.
type ProductionQueue struct {
	Items    []string
	Progress float64 // seconds elapsed on Items[0]
	Paid     float64 // charged so far on Items[0]
}

// Player holds per-player economy state plus everything they're currently
// producing. Placed buildings and units are stored on World rather than
// here, so ownership is always derived from the Owner field on those —
// there's a single source of truth for "who owns what".
type Player struct {
	ID    int
	Money int

	// Pending is the single in-progress structure, nil when idle, and
	// Queues holds one unit queue per producing building type ("Barracks",
	// "WarFactory", ...).
	//
	// Queues are per category, not per building: in RA2 a second Barracks
	// doesn't let you train two infantry at once, it makes the infantry
	// queue run faster (see productionSpeed). Different categories do run
	// in parallel, which is why infantry and vehicles each get their own
	// queue — and why buildings, being their own category, are limited to
	// the single Pending slot.
	Pending *Construction
	Queues  map[string]*ProductionQueue

	// fraction carries the sub-credit remainder between ticks. Both income
	// (20/sec spread over 20 ticks) and construction charges (a build's
	// cost spread across its whole build time) move less than a whole
	// credit per tick, and truncating each one to an int would round them
	// away to nothing. Kept in [0,1) by settle().
	fraction float64
}

func newPlayer(id int) *Player {
	return &Player{
		ID:     id,
		Money:  startingMoney,
		Queues: map[string]*ProductionQueue{},
	}
}

// queue returns the player's queue for a category, creating it on first
// use so callers don't each have to nil-check.
func (p *Player) queue(category string) *ProductionQueue {
	q, ok := p.Queues[category]
	if !ok {
		q = &ProductionQueue{}
		p.Queues[category] = q
	}
	return q
}

func (p *Player) addIncome(dt float64) {
	p.fraction += passiveIncome * dt
	p.settle()
}

// balance is the player's true worth including the sub-credit remainder.
func (p *Player) balance() float64 {
	return float64(p.Money) + p.fraction
}

// tryPay deducts amount if the player can cover it, reporting whether it
// went through. Callers must not advance whatever they're paying for when
// this returns false.
func (p *Player) tryPay(amount float64) bool {
	if amount <= 0 {
		return true // nothing owed this tick (rounding, or a free item)
	}
	if p.balance() < amount {
		return false
	}

	p.fraction -= amount
	p.settle()
	return true
}

func (p *Player) refund(amount float64) {
	p.fraction += amount
	p.settle()
}

// settle moves whole credits out of fraction and into Money, using Floor
// so a negative fraction borrows a credit rather than truncating toward
// zero and quietly inventing money.
func (p *Player) settle() {
	whole := math.Floor(p.fraction)
	p.Money += int(whole)
	p.fraction -= whole
}
