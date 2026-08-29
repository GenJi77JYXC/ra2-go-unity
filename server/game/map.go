package game

import (
	"math"
	"sort"
)

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
	OreDrill
)

type Tile struct {
	Type     TerrainType
	Passable bool
}

// GameMap is static terrain data. It's built once at startup and never
// mutated afterward, so — unlike Unit — it's safe to read from any
// goroutine without synchronization.
//
// Ore is the interesting case here: how much ore a cell still holds
// changes constantly, which would break exactly that guarantee. So the map
// only records *which* cells are ore field (that never changes) and the
// live amounts live on World, the same split occupancy.go uses for units.
type GameMap struct {
	Width, Height int
	Tiles         [][]Tile // Tiles[y][x]

	// drills are the neutral fixtures that regrow ore around themselves.
	// They're map features rather than Buildings on purpose: a Building
	// needs an owner, and every victory rule and combat check counts
	// buildings by owner — a neutral one would be a special case in all of
	// them.
	drills []oreDrill

	// oreCells is every cell that can ever hold ore, in a fixed order.
	// Amounts are broadcast as a bare array in this order, so the order is
	// part of the wire format and must not be rearranged after the initial
	// snapshot goes out.
	oreCells []cell

	// starts maps a player number to where their Construction Yard goes.
	// Keeping it on the map is what stopped the layout and the spawn
	// coordinates from being two separate pieces of hardcoding that had to
	// agree (see maps.go).
	starts map[int]cell
}

// oreDrill regrows ore in the cells around it. field is pre-sorted by
// distance from the drill, so growth just tops up the first cell that
// isn't full — which makes ore creep back outward from the centre the way
// it does in the original.
type oreDrill struct {
	center cell
	field  []cell
}

// OreCell is one ore-field cell on the wire. It exists because cell is
// unexported (it's a pathfinding detail) but the network layer has to name
// these positions in the initial snapshot.
type OreCell struct{ X, Y int }

// OreCells returns the ore-field cells in broadcast order, for the initial
// snapshot. Every later frame sends only the amounts, in this same order.
func (m *GameMap) OreCells() []OreCell {
	out := make([]OreCell, len(m.oreCells))
	for i, c := range m.oreCells {
		out[i] = OreCell{X: c.X, Y: c.Y}
	}
	return out
}

func (m *GameMap) InBounds(x, y int) bool {
	return x >= 0 && x < m.Width && y >= 0 && y < m.Height
}

func (m *GameMap) PassableAt(x, y int) bool {
	return m.InBounds(x, y) && m.Tiles[y][x].Passable
}

// oreFieldRadius is how far ore spreads around a drill, as a square
// radius. Two of these fields is plenty of income for a 20x20 map; the
// size is what makes a field small enough to be worth contesting.
const oreFieldRadius = 2

// addOreDrill plants a drill and turns the plain grass around it into ore
// field. Anything already meaningful — water, cliff, road — is skipped
// rather than overwritten, so a field can be dropped near terrain without
// hand-checking its whole radius. (The two Construction Yards are placed
// by NewWorld, which the map never sees, so the drills are positioned
// clear of them by hand.)
func (m *GameMap) addOreDrill(center cell, radius int) {
	if !m.InBounds(center.X, center.Y) {
		return
	}
	m.Tiles[center.Y][center.X] = Tile{Type: OreDrill, Passable: false}

	drill := oreDrill{center: center}

	for dy := -radius; dy <= radius; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			c := cell{X: center.X + dx, Y: center.Y + dy}
			if c == center || !m.PassableAt(c.X, c.Y) {
				continue
			}
			if m.Tiles[c.Y][c.X].Type != Grass {
				continue // roads and anything else already meaningful
			}
			m.Tiles[c.Y][c.X] = Tile{Type: Ore, Passable: true}
			drill.field = append(drill.field, c)
			m.oreCells = append(m.oreCells, c)
		}
	}

	sort.Slice(drill.field, func(i, j int) bool {
		return squaredDistance(drill.field[i], center) < squaredDistance(drill.field[j], center)
	})
	m.drills = append(m.drills, drill)
}

func squaredDistance(a, b cell) int {
	dx, dy := a.X-b.X, a.Y-b.Y
	return dx*dx + dy*dy
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
