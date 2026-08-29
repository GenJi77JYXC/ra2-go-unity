package game

import "testing"

// aiWorld is a standard match with the computer at seat 2.
func aiWorld() *World {
	w := NewWorld("buildings")
	w.AddAI(2)
	return w
}

func (ai *AIPlayer) goal(kind GoalKind) *Goal {
	for _, g := range ai.goals {
		if g.Kind == kind {
			return g
		}
	}
	return nil
}

func (w *World) countFor(owner int, buildingType string) int {
	return w.countBuildings(owner, buildingType)
}

func (w *World) unitsOf(owner int, template string) int {
	n := 0
	for _, u := range w.Units {
		if u.Owner == owner && u.Template == template {
			n++
		}
	}
	return n
}

// run advances the world and reports the second at which cond first held,
// or -1. Sampling per tick rather than at the end is what makes the
// milestone's timing claims checkable at all.
func (w *World) run(seconds int, cond func() bool) int {
	for i := 0; i < seconds*20; i++ {
		w.Tick(tickSeconds)
		if cond != nil && cond() {
			return i / 20
		}
	}
	return -1
}

// The learning plan's milestone, as a test: an economy goes up, tanks
// start rolling inside the first minute, and a wave leaves in the second.
func TestAIMeetsTheOpeningMilestone(t *testing.T) {
	w := aiWorld()

	w.run(200, nil)

	for _, want := range []string{"PowerPlant", refineryType, "Barracks", "WarFactory"} {
		if w.countFor(2, want) == 0 {
			t.Errorf("AI never built a %s", want)
		}
	}
	if got := w.unitsOf(2, "Tank"); got < 3 {
		t.Errorf("AI has %d tanks after 200s, expected it to be producing", got)
	}
}

func TestAIStartsTanksAndAttacksOnSchedule(t *testing.T) {
	w := aiWorld()

	// The two tanks it starts with don't count as production.
	firstTank := w.run(120, func() bool { return w.unitsOf(2, "Tank") > 2 })
	if firstTank < 0 {
		t.Fatal("AI never produced a tank")
	}
	if firstTank > 60 {
		t.Errorf("first tank at t=%ds, milestone wants production under way by 60s", firstTank)
	}

	attacking := func() bool {
		for _, u := range w.Units {
			if u.Owner == 2 && u.AttackMove {
				return true
			}
		}
		return false
	}
	launch := firstTank + w.run(120, attacking)
	if launch < firstTank {
		t.Fatal("AI never launched an attack")
	}
	if launch < 60 || launch > 110 {
		t.Errorf("wave left at t=%ds; milestone wants it massed and moving in the 60-90s range", launch)
	}
}

// "电力不足自动补电厂" — the AI shouldn't need a rule for this. Priorities
// are rescored from the board every second, so a brownout simply makes the
// power goal the most wanted thing again.
func TestAIBuildsMorePowerWhenItBrownsOut(t *testing.T) {
	w := aiWorld()
	w.run(60, nil)

	if w.NetPower(2) < 0 {
		t.Skip("AI is already in deficit; nothing to observe")
	}
	before := w.countFor(2, "PowerPlant")

	// A war factory draws 100, which is a whole power plant's worth.
	w.addBuilding(900, "WarFactory", 2, 0, 18)
	w.addBuilding(901, "WarFactory", 2, 4, 18)
	if w.NetPower(2) >= 0 {
		t.Fatalf("setup failed: power is %d, expected a deficit", w.NetPower(2))
	}

	w.run(120, nil)

	if got := w.countFor(2, "PowerPlant"); got <= before {
		t.Errorf("power plants stayed at %d through a deficit of %d", got, w.NetPower(2))
	}
}

// "矿场被拆自动重建" — same mechanism, no extra rule.
func TestAIRebuildsALostRefinery(t *testing.T) {
	w := aiWorld()

	if built := w.run(90, func() bool { return w.countFor(2, refineryType) > 0 }); built < 0 {
		t.Fatal("AI never built a refinery to lose")
	}

	for _, b := range w.Buildings {
		if b.Owner == 2 && b.Type == refineryType {
			b.HP = 0
		}
	}
	w.removeDestroyed()
	if w.countFor(2, refineryType) != 0 {
		t.Fatal("setup failed: the refinery survived")
	}

	if rebuilt := w.run(120, func() bool { return w.countFor(2, refineryType) > 0 }); rebuilt < 0 {
		t.Error("AI never replaced its refinery, so it has no income at all")
	}
}

// The hazard the learning plan calls out by name: Goal.Status has no
// "failed", so a goal that can never be carried out would sit at the top
// of the list retrying every second and the AI would do nothing else.
//
// With no Construction Yard there is nowhere to build from, so every
// structure goal fails permanently — the worst case.
func TestUnachievableGoalBacksOffInsteadOfSpinning(t *testing.T) {
	w := aiWorld()
	for _, b := range w.Buildings {
		if b.Owner == 2 {
			b.HP = 0
		}
	}
	w.removeDestroyed()

	const seconds = 60
	w.run(seconds, nil)

	power := w.AIs[0].goal(GoalBuildPower)
	if power.Failures == 0 {
		t.Fatal("the power goal never even tried")
	}
	if power.Failures >= seconds {
		t.Errorf("power goal failed %d times in %ds — it's retrying every "+
			"assess instead of backing off", power.Failures, seconds)
	}
	if power.Backoff > goalMaxBackoff {
		t.Errorf("backoff grew to %.1fs, past the %.1fs cap", power.Backoff, goalMaxBackoff)
	}
}

// A goal that was hopeless has to get a clean try when the board changes,
// or the AI stays crippled by a problem that has already gone away.
func TestBackoffClearsWhenTheGoalBecomesReachableAgain(t *testing.T) {
	w := aiWorld()
	for _, b := range w.Buildings {
		if b.Owner == 2 {
			b.HP = 0
		}
	}
	w.removeDestroyed()
	w.run(30, nil)

	if w.AIs[0].goal(GoalBuildPower).Failures == 0 {
		t.Fatal("setup failed: the power goal never failed")
	}

	w.addBuilding(900, "ConstructionYard", 2, 16, 11)

	if built := w.run(120, func() bool { return w.countFor(2, "PowerPlant") > 0 }); built < 0 {
		t.Error("AI never recovered once it had somewhere to build from")
	}
}

// The AI issues Commands and nothing else, so everything it does is
// subject to the same ownership checks a connected player's orders are.
// A command stamped with anyone else's ID would be a way to cheat.
func TestAIOnlyEverCommandsItsOwnSeat(t *testing.T) {
	w := aiWorld()
	ai := w.AIs[0]

	for i := 0; i < 200*20; i++ {
		for _, cmd := range ai.Think(w, tickSeconds) {
			if cmd.Owner != ai.Owner {
				t.Fatalf("AI issued %q as owner %d, it plays seat %d",
					cmd.Type, cmd.Owner, ai.Owner)
			}
			w.HandleCommand(cmd)
		}
		w.Tick(tickSeconds)
	}
}

// An attack wave has to actually set out for the enemy, not just be
// flagged as attacking.
func TestAttackWaveMarchesOnTheEnemyBase(t *testing.T) {
	w := aiWorld()

	launched := w.run(150, func() bool {
		for _, u := range w.Units {
			if u.Owner == 2 && u.AttackMove && u.HasGoal {
				return true
			}
		}
		return false
	})
	if launched < 0 {
		t.Fatal("no wave ever left with a destination")
	}

	var enemyYard *Building
	for _, b := range w.Buildings {
		if b.Owner == 1 && b.Type == "ConstructionYard" {
			enemyYard = b
		}
	}
	if enemyYard == nil {
		t.Skip("the enemy yard is already gone")
	}

	for _, u := range w.Units {
		if u.Owner != 2 || !u.AttackMove || !u.HasGoal {
			continue
		}
		d := manhattan(u.Goal, cell{X: enemyYard.CellX, Y: enemyYard.CellY})
		if d > 6 {
			t.Errorf("unit %d is marching to %v, %d cells from the enemy yard at (%d,%d)",
				u.ID, u.Goal, d, enemyYard.CellX, enemyYard.CellY)
		}
	}
}

// Harvesters come off the war factory line, as in the original — the
// refinery only ever hands over the one it arrives with. That makes every
// harvester a tank the player didn't build.
func TestHarvestersAreBuiltAtTheWarFactory(t *testing.T) {
	if buildingTemplates[refineryType].canProduce("Harvester") {
		t.Error("the refinery still offers harvesters; they belong to the war factory")
	}
	if !buildingTemplates["WarFactory"].canProduce("Harvester") {
		t.Error("the war factory can't build harvesters")
	}
	if !buildingTemplates["WarFactory"].canProduce("Tank") {
		t.Error("the war factory lost tanks")
	}
}

// One harvester is far too thin to sustain an army, so the AI adds to the
// fleet — but only after its first wave is together, or the wave never
// leaves on time.
func TestAIGrowsItsMiningFleetOnceItHasAnArmy(t *testing.T) {
	w := aiWorld()

	fleet := func() int {
		n := 0
		for _, u := range w.Units {
			if u.Owner == 2 && u.Harvest != nil {
				n++
			}
		}
		return n
	}

	// The one that came with the first refinery, and no more, until the
	// army is up.
	w.run(60, nil)
	if got := fleet(); got > 1 {
		t.Errorf("AI had %d harvesters at 60s; building them this early "+
			"delays the first attack wave", got)
	}

	w.run(180, nil)
	if got := fleet(); got < 2 {
		t.Errorf("AI still has %d harvesters after 240s — its economy can "+
			"never sustain production", got)
	}
}

// Selling is a way out of being stuck, and the AI is only allowed to sell
// power it can spare. That restriction is what stops it selling a plant,
// browning out, rebuilding the plant and going broke again.
func TestAISellsOnlyPowerItCanSpare(t *testing.T) {
	w := aiWorld()
	w.run(60, nil)

	// Give it plenty of power and nothing to spend it on, then take its
	// money away — exactly the position losing a war factory leaves it in.
	w.addBuilding(900, "PowerPlant", 2, 0, 18)
	w.addBuilding(901, "PowerPlant", 2, 3, 18)
	before := w.countFor(2, "PowerPlant")

	w.Players[2].Money = 0
	w.run(20, nil)

	if got := w.countFor(2, "PowerPlant"); got >= before {
		t.Fatalf("AI kept all %d power plants while broke with power to spare", got)
	}
	if w.Players[2].Money <= 0 {
		t.Error("selling brought in nothing")
	}
	if p := w.NetPower(2); p < 0 {
		t.Errorf("AI sold itself into a brownout (power %d) — the surplus "+
			"check is what keeps selling from looping", p)
	}
}

func TestSellRefundsHalfAndRemovesTheBuilding(t *testing.T) {
	w := newTestWorld()
	w.addBuilding(100, "PowerPlant", 1, 0, 0)
	w.Players[1].Money = 0

	w.HandleCommand(Command{Type: "sell", Owner: 1, BuildingID: 100})
	w.removeDestroyed()

	want := buildingTemplates["PowerPlant"].Cost / 2
	if got := w.Players[1].Money; got != want {
		t.Errorf("refund was %d, want %d (half of %d)",
			got, want, buildingTemplates["PowerPlant"].Cost)
	}
	if w.findBuilding(100) != nil {
		t.Error("the building is still standing")
	}
}

func TestSellIgnoresBuildingsYouDoNotOwn(t *testing.T) {
	w := newTestWorld()
	w.addBuilding(100, "PowerPlant", 2, 0, 0)
	w.Players[1].Money = 0

	w.HandleCommand(Command{Type: "sell", Owner: 1, BuildingID: 100})
	w.removeDestroyed()

	if w.Players[1].Money != 0 {
		t.Errorf("player 1 was paid %d for someone else's building", w.Players[1].Money)
	}
	if w.findBuilding(100) == nil {
		t.Error("player 1 demolished a building they don't own")
	}
}
