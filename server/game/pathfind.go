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
