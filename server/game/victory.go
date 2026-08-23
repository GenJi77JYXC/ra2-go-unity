package game

// VictoryCondition decides what counts as being knocked out. The learning
// plan never specified one — it only says a room can reach a "finished"
// state — so these follow the options the original game offers.
const (
	// VictoryBuildings is RA2's default: lose your last structure and
	// you're out, however many units you still have wandering around.
	VictoryBuildings = "buildings"

	// VictoryConstructionYard is RA2's "short game": the Construction
	// Yard alone decides it, so a decapitation strike ends the match.
	VictoryConstructionYard = "conyard"

	// VictoryAnnihilation requires wiping out everything — the longest
	// of the three, since a single surviving infantryman keeps a player
	// alive.
	VictoryAnnihilation = "annihilation"
)

// ValidVictoryCondition reports whether a client-supplied condition is one
// we recognise, so an unknown string falls back to the default rather than
// creating a match nobody can ever win.
func ValidVictoryCondition(condition string) bool {
	switch condition {
	case VictoryBuildings, VictoryConstructionYard, VictoryAnnihilation:
		return true
	}
	return false
}

// isDefeated reports whether a player has been knocked out under this
// world's rules.
func (w *World) isDefeated(owner int) bool {
	switch w.Victory {
	case VictoryConstructionYard:
		return w.countBuildings(owner, "ConstructionYard") == 0

	case VictoryAnnihilation:
		return !w.hasAnyBuilding(owner) && !w.hasAnyUnit(owner)

	default: // VictoryBuildings
		return !w.hasAnyBuilding(owner)
	}
}

// hasAnyBuilding counts only structures actually on the map. A build in
// progress lives on Player.Pending with no map presence, so a player whose
// last structure just died is out even with something queued — an order in
// progress isn't a base.
func (w *World) hasAnyBuilding(owner int) bool {
	for _, b := range w.Buildings {
		if b.Owner == owner {
			return true
		}
	}
	return false
}

func (w *World) hasAnyUnit(owner int) bool {
	for _, u := range w.Units {
		if u.Owner == owner {
			return true
		}
	}
	return false
}

// Outcome reports whether the match has been decided, and by whom. A
// winner of 0 with over=true is a draw — possible when the last two sides
// are wiped out in the same tick.
func (w *World) Outcome() (over bool, winner int) {
	alive := 0
	for id := range w.Players {
		if !w.isDefeated(id) {
			alive++
			winner = id
		}
	}

	if alive > 1 {
		return false, 0
	}
	if alive == 1 {
		return true, winner
	}
	return true, 0
}
