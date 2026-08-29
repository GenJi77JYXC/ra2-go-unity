package game

import "testing"

// parseMap's contract is that the first line of a layout is the *top* of
// the map. A lopsided two-line layout pins it; a symmetric one would hide
// a flip, and one did slip through in exactly that way.
func TestFirstLayoutLineIsTheTopOfTheMap(t *testing.T) {
	m := parseMap("~~\n..")

	if got := m.Tiles[1][0].Type; got != Water {
		t.Errorf("top row (y=1) is %v, want Water — the layout is upside down", got)
	}
	if got := m.Tiles[0][0].Type; got != Grass {
		t.Errorf("bottom row (y=0) is %v, want Grass", got)
	}
}

func TestFixtureMatchesTheOldHandBuiltMap(t *testing.T) {
	m := newFixtureMap()
	if m.Width != 20 || m.Height != 20 {
		t.Fatalf("fixture is %dx%d, want 20x20", m.Width, m.Height)
	}

	for _, c := range []struct {
		x, y int
		want TerrainType
	}{
		{0, 0, Grass}, {4, 6, Water}, {9, 11, Water}, {4, 5, Grass},
		{14, 2, Cliff}, {14, 15, Cliff}, {14, 16, Road}, {0, 16, Road},
		{7, 2, OreDrill}, {17, 6, OreDrill}, {5, 2, Ore},
	} {
		if got := m.Tiles[c.y][c.x].Type; got != c.want {
			t.Errorf("(%d,%d) is %v, want %v", c.x, c.y, got, c.want)
		}
	}
}

// Every map has to hold a match: two seats far apart, room for a base at
// each, ore that can be driven to, and a route between them.
func TestEveryMapIsWellFormed(t *testing.T) {
	for _, def := range MapCatalog() {
		t.Run(def.Name, func(t *testing.T) {
			w := NewWorld(VictoryBuildings, def.Name)
			m := w.Map

			s1, ok1 := m.Start(1)
			s2, ok2 := m.Start(2)
			if !ok1 || !ok2 {
				t.Fatalf("map has %d starts, want 2", m.StartCount())
			}
			if apart, want := manhattan(s1, s2), m.Width/2; apart < want {
				t.Errorf("starts %v and %v are only %d cells apart; on a %dx%d map "+
					"that's close enough to rush before anyone has a base",
					s1, s2, apart, m.Width, m.Height)
			}

			// Seating is what actually proves the starts are usable: it
			// places a Construction Yard and tanks at each.
			for owner := 1; owner <= 2; owner++ {
				if got := w.countBuildings(owner, "ConstructionYard"); got != 1 {
					t.Errorf("player %d got %d Construction Yards", owner, got)
				}
				if got := w.unitsOf(owner, "Tank"); got != startingTanks {
					t.Errorf("player %d got %d tanks, want %d", owner, got, startingTanks)
				}
			}

			if len(m.oreCells) == 0 {
				t.Fatal("map has no ore at all")
			}
			enterable := w.staticEnterable()
			for _, c := range m.oreCells {
				if !enterable(c.X, c.Y) {
					t.Errorf("ore cell %v can't be driven onto", c)
				}
			}

			// A map where the two sides can't reach each other can never be
			// won under any rule. Checked against terrain alone: the starts
			// have Construction Yards standing on them by now, and those
			// would block a route to their own doorstep.
			terrain := func(x, y int) bool { return m.PassableAt(x, y) }
			if path := m.FindPath(s1, s2, terrain); len(path) == 0 {
				t.Error("there is no route between the two starting positions")
			}
		})
	}
}

// The AI has to be able to actually play every map, not just the one it
// was developed against. This is the test that would have caught a base
// packed so tightly it sealed its own harvester into a pocket — the
// symptom was an AI that built a full base on one map and then sat at
// zero credits for the rest of the match.
func TestAICanPlayEveryMap(t *testing.T) {
	for _, def := range MapCatalog() {
		t.Run(def.Name, func(t *testing.T) {
			w := NewWorld(VictoryBuildings, def.Name)
			w.AddAI(2)
			w.run(200, nil)

			for _, want := range []string{"PowerPlant", refineryType, "Barracks", "WarFactory"} {
				if w.countBuildings(2, want) == 0 {
					t.Errorf("AI never built a %s", want)
				}
			}
			if got := w.unitsOf(2, "Tank"); got <= startingTanks {
				t.Errorf("AI still has its %d starting tanks after 200s — "+
					"it produced nothing, so its economy is stuck", got)
			}

			harvesters := 0
			for _, u := range w.Units {
				if u.Owner == 2 && u.Harvest != nil {
					harvesters++
				}
			}
			if harvesters < 2 {
				t.Errorf("AI has %d harvesters after 200s", harvesters)
			}
		})
	}
}

// The AI builds with a clear cell around every structure. Packed flush,
// it can wall a unit in — and canPlace deliberately ignores units, so
// nothing else stops it.
func TestAILeavesLanesThroughItsBase(t *testing.T) {
	w := NewWorld(VictoryBuildings, "ridge")
	w.AddAI(2)
	w.run(150, nil)

	for _, a := range w.Buildings {
		if a.Owner != 2 {
			continue
		}
		t2 := buildingTemplates[a.Type]
		for y := a.CellY - 1; y <= a.CellY+t2.Height; y++ {
			for x := a.CellX - 1; x <= a.CellX+t2.Width; x++ {
				if b := w.buildingAt(x, y); b != nil && b.ID != a.ID {
					t.Fatalf("%s at (%d,%d) is flush against %s at (%d,%d)",
						a.Type, a.CellX, a.CellY, b.Type, b.CellX, b.CellY)
				}
			}
		}
	}
}

// Both real maps are rotationally symmetric, so neither seat gets terrain
// the other doesn't. Worth a test rather than a promise: the maps are
// hand-editable ASCII now, and a stray character is invisible to the eye
// but hands one player a shortcut or an extra ore field.
func TestRealMapsAreRotationallySymmetric(t *testing.T) {
	for _, def := range MapCatalog() {
		t.Run(def.Name, func(t *testing.T) {
			m := NewMap(def.Name)
			for y := 0; y < m.Height; y++ {
				for x := 0; x < m.Width; x++ {
					ox, oy := m.Width-1-x, m.Height-1-y
					if got, want := m.Tiles[y][x].Type, m.Tiles[oy][ox].Type; got != want {
						t.Fatalf("(%d,%d) is %v but its opposite (%d,%d) is %v",
							x, y, got, ox, oy, want)
					}
				}
			}
		})
	}
}
