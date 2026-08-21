package game

import (
	"log"
	"math"
)

// updateCombat drives every unit that currently has an attack order: chase
// into weapon range, then fire on cooldown. Called once per tick, after
// movement so a unit that just arrived in range this tick can still fire.
func (w *World) updateCombat(dt float64) {
	for _, u := range w.Units {
		if u.AttackTargetID == 0 {
			continue
		}

		target := w.findUnit(u.AttackTargetID)
		if target == nil {
			u.AttackTargetID = 0 // target died or is gone
			continue
		}

		weapon := weaponTemplates[unitTemplates[u.Template].Weapon]
		dist := math.Hypot(target.X-u.X, target.Y-u.Y)

		if dist > weapon.Range {
			u.chase(w.Map, target)
			continue
		}

		u.Path = nil // in range: stop advancing

		if u.FireCooldown > 0 {
			u.FireCooldown -= dt
			continue
		}

		warhead := warheadTemplates[weapon.Warhead]
		damage := weapon.Damage * warhead.Verses[target.Armor] / 100
		target.HP -= damage
		u.FireCooldown = weapon.Cooldown

		log.Printf("unit %d hit unit %d for %d damage (hp=%d/%d)", u.ID, target.ID, damage, target.HP, target.MaxHP)
	}
}

// chase paths the unit toward target's current cell. It's a one-shot
// re-path, not continuous tracking: if Path is already set, it's left
// alone rather than recomputed every tick. That's fine as long as targets
// don't move mid-chase (true for Phase 4's stationary enemies) — a real
// pursuit AI that re-paths as a moving target relocates is a later
// problem.
func (u *Unit) chase(m *GameMap, target *Unit) {
	if len(u.Path) > 0 {
		return
	}

	start := worldToCell(u.X, u.Y)
	goal := worldToCell(target.X, target.Y)

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

// removeDeadUnits drops any unit with HP <= 0, clearing AttackTargetID on
// anything that was still shooting at it. Two passes over the original
// slice (rather than filtering in place while also scanning for dangling
// references) keeps this correct without worrying about a partially
// compacted slice being read mid-loop.
func (w *World) removeDeadUnits() {
	for _, u := range w.Units {
		if u.HP > 0 {
			continue
		}

		log.Printf("unit %d destroyed", u.ID)
		for _, other := range w.Units {
			if other.AttackTargetID == u.ID {
				other.AttackTargetID = 0
			}
		}
	}

	alive := make([]*Unit, 0, len(w.Units))
	for _, u := range w.Units {
		if u.HP > 0 {
			alive = append(alive, u)
		}
	}
	w.Units = alive
}
