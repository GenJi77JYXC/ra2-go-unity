package game

import "math"

// TerrainType classifies a tile. Order matters: it's sent to clients as a
// raw int (see network.TileData), so inserting/reordering values changes
// the wire format and must be mirrored in the Unity-side enum.
type TerrainType int

const (
	Grass TerrainType = iota
	Road
	Water
	Cliff
	Ore
)

type Tile struct {
	Type     TerrainType
	Passable bool
}

// GameMap is static terrain data. It's built once at startup and never
// mutated afterward, so — unlike Unit — it's safe to read from any
// goroutine without synchronization.
type GameMap struct {
	Width, Height int
	Tiles         [][]Tile // Tiles[y][x]
}

func (m *GameMap) InBounds(x, y int) bool {
	return x >= 0 && x < m.Width && y >= 0 && y < m.Height
}

func (m *GameMap) PassableAt(x, y int) bool {
	return m.InBounds(x, y) && m.Tiles[y][x].Passable
}

// NewTestMap builds a small hand-authored 20x20 map for Phase 3: a lake and
// a cliff wall (each with gaps at the ends, not fully sealed off) so there's
// actually something for A* to route around, plus a cosmetic road strip.
func NewTestMap() *GameMap {
	const w, h = 20, 20
	m := &GameMap{Width: w, Height: h, Tiles: make([][]Tile, h)}

	for y := 0; y < h; y++ {
		m.Tiles[y] = make([]Tile, w)
		for x := 0; x < w; x++ {
			m.Tiles[y][x] = Tile{Type: Grass, Passable: true}
		}
	}

	for y := 6; y < 12; y++ {
		for x := 4; x < 10; x++ {
			m.Tiles[y][x] = Tile{Type: Water, Passable: false}
		}
	}

	for y := 2; y < 16; y++ {
		m.Tiles[y][14] = Tile{Type: Cliff, Passable: false}
	}

	for x := 0; x < w; x++ {
		m.Tiles[16][x] = Tile{Type: Road, Passable: true}
	}

	return m
}

// Point is a continuous world-space position, as opposed to cell (in
// pathfind.go), which is a discrete grid coordinate used only internally
// by pathfinding.
type Point struct {
	X, Y float64
}

type cell struct {
	X, Y int
}

// worldToCell / cellCenterWorld are the learning plan's WorldToCell /
// CellCenterWorld: the former feeds a unit's continuous position into A*,
// the latter turns the resulting cell path back into waypoints to walk.
func worldToCell(x, y float64) cell {
	return cell{X: int(math.Floor(x)), Y: int(math.Floor(y))}
}

func cellCenterWorld(c cell) Point {
	return Point{X: float64(c.X) + 0.5, Y: float64(c.Y) + 0.5}
}
