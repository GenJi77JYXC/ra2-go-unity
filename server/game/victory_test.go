package game

import "testing"

// victoryWorld sets up a two-player match under one rule, with each side
// holding a Construction Yard and one unit.
func victoryWorld(condition string) *World {
	w := &World{
		Map:     newFixtureMap(),
		Players: map[int]*Player{1: newPlayer(1), 2: newPlayer(2)},
		Victory: condition,
	}

	w.addBuilding(1, "ConstructionYard", 1, 0, 0)
	w.addBuilding(2, "ConstructionYard", 2, 10, 0)
	w.Units = []*Unit{
		newUnit(3, 0.5, 5.5, 1, "Tank"),
		newUnit(4, 10.5, 5.5, 2, "Tank"),
	}
	return w
}

// destroyBuildings removes every structure a player owns, the way combat
// would.
func (w *World) destroyBuildings(owner int) {
	for _, b := range w.Buildings {
		if b.Owner == owner {
			b.HP = 0
		}
	}
	w.removeDestroyed()
}

func (w *World) destroyUnits(owner int) {
	for _, u := range w.Units {
		if u.Owner == owner {
			u.HP = 0
		}
	}
	w.removeDestroyed()
}

func assertOutcome(t *testing.T, w *World, wantOver bool, wantWinner int) {
	t.Helper()

	over, winner := w.Outcome()
	if over != wantOver || winner != wantWinner {
		t.Fatalf("Outcome() = (%v, %d), want (%v, %d)", over, winner, wantOver, wantWinner)
	}
}

func TestMatchIsUndecidedAtStart(t *testing.T) {
	for _, condition := range []string{VictoryBuildings, VictoryConstructionYard, VictoryAnnihilation} {
		if over, _ := victoryWorld(condition).Outcome(); over {
			t.Fatalf("%s: a fresh match must not already be decided", condition)
		}
	}
}

func TestVictoryByBuildingsIgnoresSurvivingUnits(t *testing.T) {
	w := victoryWorld(VictoryBuildings)
	w.destroyBuildings(2)

	// Player 2 still has a tank, but under this rule structures are what
	// keep you in.
	if !w.hasAnyUnit(2) {
		t.Fatal("test setup: player 2 should still have a unit")
	}
	assertOutcome(t, w, true, 1)
}

func TestVictoryByConstructionYardIgnoresOtherBuildings(t *testing.T) {
	w := victoryWorld(VictoryConstructionYard)
	w.addBuilding(5, "Barracks", 2, 14, 0)

	// Losing the yard alone decides it, even with a barracks standing.
	for _, b := range w.Buildings {
		if b.Owner == 2 && b.Type == "ConstructionYard" {
			b.HP = 0
		}
	}
	w.removeDestroyed()

	if !w.hasAnyBuilding(2) {
		t.Fatal("test setup: player 2 should still have the barracks")
	}
	assertOutcome(t, w, true, 1)
}

func TestVictoryByAnnihilationNeedsEverythingGone(t *testing.T) {
	w := victoryWorld(VictoryAnnihilation)

	w.destroyBuildings(2)
	assertOutcome(t, w, false, 0) // a surviving tank keeps player 2 alive

	w.destroyUnits(2)
	assertOutcome(t, w, true, 1)
}

func TestSimultaneousWipeoutIsADraw(t *testing.T) {
	w := victoryWorld(VictoryBuildings)
	w.destroyBuildings(1)
	w.destroyBuildings(2)

	assertOutcome(t, w, true, 0)
}

func TestUnknownConditionFallsBackToDefault(t *testing.T) {
	w := NewWorld("not-a-real-rule", DefaultMapName)
	if w.Victory != VictoryBuildings {
		t.Fatalf("want fallback to %q, got %q", VictoryBuildings, w.Victory)
	}
}
