package game

import "sort"

// Unit/building/weapon/warhead templates are hardcoded maps for now —
// Phase 8 will replace this with real INI parsing (matching how the
// original RA2 rules files worked), but the shape of the data (and the
// damage/cost/tech-tree logic that consumes it) won't need to change when
// that happens.

type UnitTemplate struct {
	MaxHP     int
	Armor     string
	Weapon    string // "" = unarmed
	Cost      int
	BuildTime float64 // seconds
}

type WeaponTemplate struct {
	Damage   int
	Range    float64
	Cooldown float64 // seconds between shots
	Warhead  string
}

type WarheadTemplate struct {
	Verses map[string]int // armor type -> percent damage, e.g. 100 = full
}

// BuildingTemplate describes a placeable structure. Width/Height are in
// cells; Power is signed — positive produces, negative consumes — so the
// player's net power is a plain sum over built structures.
type BuildingTemplate struct {
	MaxHP         int
	Armor         string
	Cost          int
	BuildTime     float64 // seconds
	Width, Height int
	Power         int
	Produces      []string // unit template names this structure can build
	Prerequisites []string // building template names required before this can be placed
}

var unitTemplates = map[string]UnitTemplate{
	"Infantry": {MaxHP: 50, Armor: "light", Weapon: "Rifle", Cost: 100, BuildTime: 3},
	"Tank":     {MaxHP: 100, Armor: "heavy", Weapon: "TankCannon", Cost: 700, BuildTime: 8},
}

var weaponTemplates = map[string]WeaponTemplate{
	"Rifle":      {Damage: 10, Range: 3.0, Cooldown: 0.6, Warhead: "SA"},
	"TankCannon": {Damage: 25, Range: 4.0, Cooldown: 1.0, Warhead: "HE"},
}

var warheadTemplates = map[string]WarheadTemplate{
	"SA": {Verses: map[string]int{"heavy": 25, "light": 100}}, // small arms: poor vs armor
	"HE": {Verses: map[string]int{"heavy": 100, "light": 150}},
}

var buildingTemplates = map[string]BuildingTemplate{
	"ConstructionYard": {
		MaxHP: 1000, Armor: "heavy", Cost: 0, BuildTime: 0,
		Width: 3, Height: 3, Power: 0,
	},
	"PowerPlant": {
		MaxHP: 400, Armor: "heavy", Cost: 300, BuildTime: 5,
		Width: 2, Height: 2, Power: 100,
		Prerequisites: []string{"ConstructionYard"},
	},
	"Barracks": {
		MaxHP: 500, Armor: "heavy", Cost: 500, BuildTime: 8,
		Width: 2, Height: 2, Power: -50,
		Produces:      []string{"Infantry"},
		Prerequisites: []string{"ConstructionYard", "PowerPlant"},
	},
	"WarFactory": {
		MaxHP: 600, Armor: "heavy", Cost: 2000, BuildTime: 15,
		Width: 3, Height: 3, Power: -100,
		Produces:      []string{"Tank"},
		Prerequisites: []string{"ConstructionYard", "Barracks"},
	},
}

// BuildingOption describes one entry in the client's build menu. The
// client can't compute costs or prerequisites itself — it only knows what
// the server tells it — so the catalog ships once in the initial snapshot.
type BuildingOption struct {
	Type          string
	Cost          int
	Width, Height int
	Power         int
	Produces      []string
	Prerequisites []string
}

// BuildingCatalog lists every placeable structure, sorted by cost so the
// build menu has a stable, sensible order (Go map iteration order is
// randomized, so sorting here keeps the UI from reshuffling per run).
func BuildingCatalog() []BuildingOption {
	options := make([]BuildingOption, 0, len(buildingTemplates))
	for name, t := range buildingTemplates {
		if t.Cost <= 0 {
			continue // pre-placed only (Construction Yard), never in the menu
		}
		options = append(options, BuildingOption{
			Type:          name,
			Cost:          t.Cost,
			Width:         t.Width,
			Height:        t.Height,
			Power:         t.Power,
			Produces:      t.Produces,
			Prerequisites: t.Prerequisites,
		})
	}

	sort.Slice(options, func(i, j int) bool { return options[i].Cost < options[j].Cost })
	return options
}

// UnitCost reports what a produced unit costs, for the client's
// production UI.
func UnitCost(unitType string) int {
	return unitTemplates[unitType].Cost
}

// BuildingProduces reports which unit types a structure can build.
func BuildingProduces(buildingType string) []string {
	return buildingTemplates[buildingType].Produces
}

func (t BuildingTemplate) canProduce(unitType string) bool {
	for _, u := range t.Produces {
		if u == unitType {
			return true
		}
	}
	return false
}
