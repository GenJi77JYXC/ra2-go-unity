package game

// Unit is a single controllable game object.
// Phase 2 will add TargetX/TargetY/HasTarget/State for movement commands.
type Unit struct {
	ID int     `json:"id"`
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
}

// World holds all authoritative game state. The tick loop in main.go is the
// only goroutine that should ever mutate a World — see the "并发安全收敛到
// 主循环" note in the learning plan (Phase 6 will introduce a command
// channel so client goroutines never touch World directly).
type World struct {
	TickCount int64
	Units     []*Unit
}

// NewWorld builds a starting world with a single placeholder unit, matching
// the Phase 1 "Hello Tank" milestone.
func NewWorld() *World {
	return &World{
		Units: []*Unit{
			{ID: 1, X: 0, Y: 0},
		},
	}
}

// Tick advances the simulation by one step (called every 50ms by main.go).
func (w *World) Tick() {
	w.TickCount++

	// TODO(Phase 1): this just nudges unit 1 to the right each tick as a
	// smoke test for the Go -> Unity pipeline. Replace with real movement
	// once Phase 2 adds move commands.
	if len(w.Units) > 0 {
		w.Units[0].X += 0.1
	}
}

// Snapshot is what gets sent to clients every tick.
type Snapshot struct {
	Tick  int64   `json:"tick"`
	Units []*Unit `json:"units"`
}

func (w *World) Snapshot() Snapshot {
	return Snapshot{
		Tick:  w.TickCount,
		Units: w.Units,
	}
}
