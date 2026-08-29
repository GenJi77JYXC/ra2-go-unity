package game

import "testing"

// newTestWorld builds a world with no starting units or buildings, so each
// test can set up exactly the situation it cares about.
func newTestWorld() *World {
	return &World{
		Map:     newFixtureMap(),
		Players: map[int]*Player{1: newPlayer(1), 2: newPlayer(2)},
	}
}

func (w *World) addBuilding(id int, buildingType string, owner, cellX, cellY int) *Building {
	b := newBuilding(id, buildingType, owner, cellX, cellY, true)
	w.Buildings = append(w.Buildings, b)
	return b
}

func primaryID(t *testing.T, w *World, owner int, buildingType string) int {
	t.Helper()

	found := 0
	id := 0
	for _, b := range w.Buildings {
		if b.Owner == owner && b.Type == buildingType && b.IsPrimary {
			found++
			id = b.ID
		}
	}
	if found != 1 {
		t.Fatalf("want exactly 1 primary %s for player %d, got %d", buildingType, owner, found)
	}
	return id
}

func TestEnsurePrimaryPicksFirstAndKeepsIt(t *testing.T) {
	w := newTestWorld()
	w.addBuilding(1, "Barracks", 1, 0, 0)
	w.ensurePrimary(1, "Barracks")

	if got := primaryID(t, w, 1, "Barracks"); got != 1 {
		t.Fatalf("first barracks should be primary, got %d", got)
	}

	// A second one must not steal the flag.
	w.addBuilding(2, "Barracks", 1, 3, 0)
	w.ensurePrimary(1, "Barracks")

	if got := primaryID(t, w, 1, "Barracks"); got != 1 {
		t.Fatalf("primary should still be 1, got %d", got)
	}
}

func TestSetPrimaryMovesFlag(t *testing.T) {
	w := newTestWorld()
	w.addBuilding(1, "Barracks", 1, 0, 0)
	w.addBuilding(2, "Barracks", 1, 3, 0)
	w.ensurePrimary(1, "Barracks")

	w.setPrimary(1, 2)

	if got := primaryID(t, w, 1, "Barracks"); got != 2 {
		t.Fatalf("want primary 2, got %d", got)
	}
}

func TestSetPrimaryIgnoresOtherPlayersBuildings(t *testing.T) {
	w := newTestWorld()
	w.addBuilding(1, "Barracks", 1, 0, 0)
	enemy := w.addBuilding(2, "Barracks", 2, 10, 0)
	w.ensurePrimary(1, "Barracks")

	w.setPrimary(1, enemy.ID) // player 1 trying to flag player 2's barracks

	if got := primaryID(t, w, 1, "Barracks"); got != 1 {
		t.Fatalf("player 1's primary should be untouched, got %d", got)
	}
	if enemy.IsPrimary {
		t.Fatal("enemy barracks must not be flagged by another player's command")
	}
}

func TestPrimaryTransfersWhenDestroyed(t *testing.T) {
	w := newTestWorld()
	first := w.addBuilding(1, "Barracks", 1, 0, 0)
	w.addBuilding(2, "Barracks", 1, 3, 0)
	w.ensurePrimary(1, "Barracks")

	first.HP = 0
	w.removeDestroyed()

	if got := primaryID(t, w, 1, "Barracks"); got != 2 {
		t.Fatalf("surviving barracks should inherit primary, got %d", got)
	}
}

func TestProducedUnitSpawnsAtPrimary(t *testing.T) {
	w := newTestWorld()
	w.addBuilding(1, "Barracks", 1, 0, 0)
	w.addBuilding(2, "Barracks", 1, 8, 0)
	w.ensurePrimary(1, "Barracks")
	w.setPrimary(1, 2) // exit should be the far barracks, not the near one

	// Ordering from the *non*-primary barracks still exits at the primary:
	// all factories of a type share one queue.
	w.HandleCommand(Command{Type: "produce", Owner: 1, BuildingID: 1, ItemType: "Infantry"})

	for i := 0; i < 200 && len(w.Units) == 0; i++ {
		w.updateProduction(0.05)
	}

	if len(w.Units) != 1 {
		t.Fatalf("want 1 produced unit, got %d", len(w.Units))
	}
	if x := w.Units[0].X; x < 8 {
		t.Fatalf("unit should exit next to the primary barracks at cellX 8, got x=%v", x)
	}
}

func TestExtraFactoriesSpeedUpSharedQueue(t *testing.T) {
	one := productionSpeed(1)
	two := productionSpeed(2)

	if one != 1 {
		t.Fatalf("a single factory should run at base speed, got %v", one)
	}
	if two <= one {
		t.Fatalf("a second factory should speed the queue up, got %v then %v", one, two)
	}
	if productionSpeed(0) != 0 {
		t.Fatal("with no factories left the queue must stall")
	}
}
