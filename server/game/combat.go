package game

import (
	"log"
	"math"
)

// combatTarget is anything that can be shot at. Units and buildings differ
// in nearly everything else, but combat only needs a position to shoot
// toward, an armor type for the damage formula, and a way to take damage —
// so they share this instead of updateCombat special-casing both.
type combatTarget interface {
	Position() (x, y float64)
	ArmorType() string
	TakeDamage(amount int)
	IsAlive() bool
	EntityID() int
}

func (u *Unit) Position() (float64, float64) { return u.X, u.Y }
func (u *Unit) ArmorType() string            { return u.Armor }
func (u *Unit) TakeDamage(amount int)        { u.HP -= amount }
func (u *Unit) IsAlive() bool                { return u.HP > 0 }
func (u *Unit) EntityID() int                { return u.ID }

// Position for a building is the center of its footprint, so units close
// in on the middle of a structure rather than its corner cell.
func (b *Building) Position() (float64, float64) {
	t := buildingTemplates[b.Type]
	return float64(b.CellX) + float64(t.Width)/2, float64(b.CellY) + float64(t.Height)/2
}

func (b *Building) ArmorType() string     { return b.Armor }
func (b *Building) TakeDamage(amount int) { b.HP -= amount }
func (b *Building) IsAlive() bool         { return b.HP > 0 }
func (b *Building) EntityID() int         { return b.ID }

// findTarget resolves an attack target ID against both units and
// buildings — they share one ID space (see World.nextID), so an ID is
// unambiguous. Returns a true nil rather than a nil-valued interface when
// nothing matches, so callers can compare against nil safely.
func (w *World) findTarget(id int) combatTarget {
	if u := w.findUnit(id); u != nil {
		return u
	}
	if b := w.findBuilding(id); b != nil {
		return b
	}
	return nil
}

// updateCombat drives every unit that currently has an attack order: chase
// into weapon range, then fire on cooldown. Called once per tick, after
// movement so a unit that just arrived in range this tick can still fire.
func (w *World) updateCombat(dt float64) {
	for _, u := range w.Units {
		if u.AttackTargetID == 0 {
			continue
		}

		target := w.findTarget(u.AttackTargetID)
		if target == nil {
			u.AttackTargetID = 0 // target died or is gone
			continue
		}

		tx, ty := target.Position()
		weapon := weaponTemplates[unitTemplates[u.Template].Weapon]
		dist := math.Hypot(tx-u.X, ty-u.Y)

		if dist > weapon.Range {
			u.chase(w.Map, tx, ty)
			continue
		}

		u.Path = nil // in range: stop advancing

		if u.FireCooldown > 0 {
			u.FireCooldown -= dt
			continue
		}

		warhead := warheadTemplates[weapon.Warhead]
		damage := weapon.Damage * warhead.Verses[target.ArmorType()] / 100
		target.TakeDamage(damage)
		u.FireCooldown = weapon.Cooldown

		log.Printf("unit %d hit entity %d for %d damage", u.ID, target.EntityID(), damage)
	}
}

// chase paths the unit toward the target's current cell. It's a one-shot
// re-path, not continuous tracking: if Path is already set, it's left
// alone rather than recomputed every tick. That's fine as long as targets
// don't move mid-chase (true for stationary enemies and buildings) — a
// real pursuit AI that re-paths as a moving target relocates is a later
// problem.
//
// A building's own cells are impassable to pathfinding, so the route stops
// at the edge of its footprint; weapon range covers the rest.
func (u *Unit) chase(m *GameMap, targetX, targetY float64) {
	if len(u.Path) > 0 {
		return
	}

	start := worldToCell(u.X, u.Y)
	goal := worldToCell(targetX, targetY)

	path := m.FindPath(start, goal)
	if len(path) <= 1 {
		return
	}
	u.Path = toWaypoints(path[1:])
}

func (w *World) findUnit(id int) *Unit {
	for _, u := range w.Units {
		if u.ID == id {
			return u
		}
	}
	return nil
}

// removeDestroyed drops any unit or building reduced to 0 HP, clearing
// AttackTargetID on anything that was still shooting at it. Dangling
// references are cleared in a first pass over the untouched slices, then
// survivors are collected into fresh ones — filtering in place while also
// scanning for references would mean reading a half-compacted slice.
func (w *World) removeDestroyed() {
	for _, u := range w.Units {
		if u.HP <= 0 {
			log.Printf("unit %d destroyed", u.ID)
			w.clearAttackersOf(u.ID)
		}
	}
	for _, b := range w.Buildings {
		if b.HP <= 0 {
			log.Printf("building %d (%s) destroyed", b.ID, b.Type)
			w.clearAttackersOf(b.ID)
		}
	}

	units := make([]*Unit, 0, len(w.Units))
	for _, u := range w.Units {
		if u.HP > 0 {
			units = append(units, u)
		}
	}
	w.Units = units

	type ownerCategory struct {
		owner    int
		category string
	}

	lostPrimaries := map[ownerCategory]bool{}
	buildings := make([]*Building, 0, len(w.Buildings))
	for _, b := range w.Buildings {
		if b.HP > 0 {
			buildings = append(buildings, b)
			continue
		}
		if b.IsPrimary {
			lostPrimaries[ownerCategory{b.Owner, b.Type}] = true
		}
	}
	w.Buildings = buildings

	// Losing the primary factory hands the flag to a surviving one of the
	// same type, so production doesn't stall just because the designated
	// exit blew up.
	for key := range lostPrimaries {
		w.ensurePrimary(key.owner, key.category)
	}
}

func (w *World) clearAttackersOf(id int) {
	for _, other := range w.Units {
		if other.AttackTargetID == id {
			other.AttackTargetID = 0
		}
	}
}
