package game

import "container/heap"

// FindPath runs A* from start to goal using 4-directional movement and a
// Manhattan-distance heuristic. The learning plan originally suggested
// Euclidean distance, but for a grid that only allows axis-aligned moves,
// Manhattan is still admissible and expands fewer nodes — it's the tighter
// heuristic here. Returns the cell path including both start and goal, or
// nil if the goal is unreachable or impassable.
func (m *GameMap) FindPath(start, goal cell) []cell {
	if !m.PassableAt(goal.X, goal.Y) {
		return nil
	}
	if start == goal {
		return []cell{start}
	}

	open := &cellHeap{{point: start, priority: manhattan(start, goal)}}
	cameFrom := map[cell]cell{}
	gScore := map[cell]int{start: 0}
	closed := map[cell]bool{}

	for open.Len() > 0 {
		current := heap.Pop(open).(cellHeapItem).point
		if current == goal {
			return reconstructPath(cameFrom, start, goal)
		}
		if closed[current] {
			continue
		}
		closed[current] = true

		for _, next := range neighbors4(current) {
			if !m.PassableAt(next.X, next.Y) || closed[next] {
				continue
			}

			tentative := gScore[current] + 1
			if existing, ok := gScore[next]; ok && tentative >= existing {
				continue
			}

			cameFrom[next] = current
			gScore[next] = tentative
			heap.Push(open, cellHeapItem{point: next, priority: tentative + manhattan(next, goal)})
		}
	}

	return nil // unreachable
}

func neighbors4(c cell) [4]cell {
	return [4]cell{
		{c.X + 1, c.Y}, {c.X - 1, c.Y},
		{c.X, c.Y + 1}, {c.X, c.Y - 1},
	}
}

func manhattan(a, b cell) int {
	return absInt(a.X-b.X) + absInt(a.Y-b.Y)
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// nearbyPassableCells finds count distinct passable cells at or near
// center, searching outward ring by ring (Chebyshev distance 1, 2, 3, ...)
// until enough are found. Used to spread a group move order across several
// nearby cells instead of sending every selected unit to the exact same
// point, where they'd otherwise end up stacked on each other. Callers are
// expected to have already checked center itself is passable — this
// doesn't special-case an impassable center, it just includes it first if
// PassableAt says so.
func nearbyPassableCells(m *GameMap, center cell, count int) []cell {
	result := make([]cell, 0, count)

	if m.PassableAt(center.X, center.Y) {
		result = append(result, center)
	}

	for radius := 1; len(result) < count && radius <= m.Width+m.Height; radius++ {
		for _, c := range ringCells(center, radius) {
			if len(result) >= count {
				break
			}
			if m.PassableAt(c.X, c.Y) {
				result = append(result, c)
			}
		}
	}

	// Ran out of passable cells nearby (a tiny isolated pocket, or a huge
	// group order) — pad with center so callers always get exactly count
	// entries; FindPath will just report those as unreachable.
	for len(result) < count {
		result = append(result, center)
	}

	return result
}

// ringCells returns every cell exactly `radius` away from center by
// Chebyshev distance (the edge of a (2*radius+1)-square), i.e. one step
// further out than the previous radius's ring.
func ringCells(center cell, radius int) []cell {
	cells := make([]cell, 0, radius*8)
	for dx := -radius; dx <= radius; dx++ {
		for dy := -radius; dy <= radius; dy++ {
			if absInt(dx) != radius && absInt(dy) != radius {
				continue
			}
			cells = append(cells, cell{X: center.X + dx, Y: center.Y + dy})
		}
	}
	return cells
}

func reconstructPath(cameFrom map[cell]cell, start, goal cell) []cell {
	path := []cell{goal}
	for path[0] != start {
		path = append([]cell{cameFrom[path[0]]}, path...)
	}
	return path
}

// cellHeap is a container/heap priority queue ordered by priority
// (gScore + heuristic). It uses lazy deletion — a cell can be pushed more
// than once, and stale entries are skipped via the closed set in FindPath —
// instead of decrease-key, which needs heap.Fix and per-item indices. For a
// map this small the extra heap entries are negligible, and this is a lot
// simpler.
type cellHeapItem struct {
	point    cell
	priority int
}

type cellHeap []cellHeapItem

func (h cellHeap) Len() int           { return len(h) }
func (h cellHeap) Less(i, j int) bool { return h[i].priority < h[j].priority }
func (h cellHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *cellHeap) Push(x interface{}) {
	*h = append(*h, x.(cellHeapItem))
}

func (h *cellHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}
