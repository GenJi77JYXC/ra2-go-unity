package game

// Unit/weapon/warhead templates are hardcoded maps for now — Phase 8 will
// replace this with real INI parsing (matching how the original RA2 rules
// files worked), but the shape of the data (and the damage formula that
// consumes it) won't need to change when that happens.

type UnitTemplate struct {
	MaxHP  int
	Armor  string
	Weapon string // "" = unarmed
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

var unitTemplates = map[string]UnitTemplate{
	"Tank": {MaxHP: 100, Armor: "heavy", Weapon: "TankCannon"},
}

var weaponTemplates = map[string]WeaponTemplate{
	"TankCannon": {Damage: 25, Range: 4.0, Cooldown: 1.0, Warhead: "HE"},
}

var warheadTemplates = map[string]WarheadTemplate{
	"HE": {Verses: map[string]int{"heavy": 100, "light": 150}},
}
