package game

import "testing"

// tick runs the production/construction updates for roughly the given
// number of seconds, at the real tick rate.
func (w *World) tickFor(seconds float64) {
	dt := TickInterval.Seconds()
	for elapsed := 0.0; elapsed < seconds; elapsed += dt {
		w.updateConstruction(dt)
		w.updateProduction(dt)
	}
}

func TestLosingLastConstructionYardCancelsAndRefunds(t *testing.T) {
	w := newTestWorld()
	yard := w.addBuilding(1, "ConstructionYard", 1, 0, 0)

	player := w.Players[1]
	w.HandleCommand(Command{Type: "build", Owner: 1, ItemType: "PowerPlant"})
	w.tickFor(2) // part-way through a 5s build

	spent := startingMoney - player.Money
	if spent <= 0 {
		t.Fatalf("expected the build to have been charged, spent=%d", spent)
	}
	if player.Pending == nil {
		t.Fatal("expected a pending construction")
	}

	yard.HP = 0
	w.removeDestroyed()
	w.tickFor(0.1)

	if player.Pending != nil {
		t.Fatal("losing the last Construction Yard must cancel the build, not freeze it")
	}
	if player.Money != startingMoney {
		t.Fatalf("want a full refund back to %d, got %d", startingMoney, player.Money)
	}
}

func TestLosingLastFactoryCancelsItsQueue(t *testing.T) {
	w := newTestWorld()
	w.addBuilding(1, "ConstructionYard", 1, 0, 0)
	barracks := w.addBuilding(2, "Barracks", 1, 5, 0)
	w.ensurePrimary(1, "Barracks")

	player := w.Players[1]
	w.HandleCommand(Command{Type: "produce", Owner: 1, BuildingID: barracks.ID, ItemType: "Infantry"})
	w.tickFor(1) // part-way through a 3s unit

	if player.Money >= startingMoney {
		t.Fatal("expected the unit to have been charged")
	}

	barracks.HP = 0
	w.removeDestroyed()
	w.tickFor(0.1)

	if len(player.queue("Barracks").Items) != 0 {
		t.Fatal("losing the last Barracks must clear its queue")
	}
	if player.Money != startingMoney {
		t.Fatalf("want a full refund back to %d, got %d", startingMoney, player.Money)
	}
}

func TestOtherCategoriesSurviveLosingOneFactory(t *testing.T) {
	w := newTestWorld()
	w.addBuilding(1, "ConstructionYard", 1, 0, 0)
	barracks := w.addBuilding(2, "Barracks", 1, 5, 0)
	factory := w.addBuilding(3, "WarFactory", 1, 10, 0)
	w.ensurePrimary(1, "Barracks")
	w.ensurePrimary(1, "WarFactory")

	player := w.Players[1]
	w.HandleCommand(Command{Type: "produce", Owner: 1, BuildingID: barracks.ID, ItemType: "Infantry"})
	w.HandleCommand(Command{Type: "produce", Owner: 1, BuildingID: factory.ID, ItemType: "Tank"})
	w.tickFor(1)

	barracks.HP = 0
	w.removeDestroyed()
	w.tickFor(0.1)

	if len(player.queue("Barracks").Items) != 0 {
		t.Fatal("the infantry queue should have been cancelled")
	}
	if len(player.queue("WarFactory").Items) != 1 {
		t.Fatal("the vehicle queue belongs to a different category and must be untouched")
	}
}
