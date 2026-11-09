package diagram

import "sort"

// Balance a tall fan-out across two adjacent layout shelves. Only private,
// terminal, singleton-package cards can move: shared targets, grouped packages,
// dispatch boundaries, and the call backbone keep their original constraints.
// minlen changes geometric rank distance, never source execution or call order.
func (index *IndexedGraph) compactBrowserFanout(edges []browserEdge, visible []int) {
	outgoing := make(map[int][]int)
	for id, edge := range edges {
		if !edge.backward && edge.from != edge.to {
			outgoing[edge.from] = append(outgoing[edge.from], id)
		}
	}
	groupSize := make(map[int]int)
	for _, group := range index.browserGroups(visible) {
		for _, node := range group.nodes {
			groupSize[node] = len(group.nodes)
		}
	}
	heights := make(map[int]int)
	heightOf := func(id int) int {
		if height, exists := heights[id]; exists {
			return height
		}
		height := browserCardHeight(index.layouts[id])
		heights[id] = height
		return height
	}
	for _, source := range visible {
		branches := outgoing[source]
		if len(branches) < 4 {
			continue
		}
		first, second := 0, 0
		children := map[int]bool{source: true}
		var candidates []int
		for _, id := range branches {
			target := edges[id].to
			children[target] = true
			height := heightOf(target)
			first += height
			// Use full-tree relationship counts: a focused view must not turn
			// a target with hidden callers/calls into a private terminal.
			if height >= 8 && index.relations[target] == 1 && index.registrationRelations[target] == 0 && groupSize[target] == 1 &&
				index.Functions[source].Package != index.Functions[target].Package && index.phase(source) == index.phase(target) {
				candidates = append(candidates, id)
			}
		}
		if len(candidates) == 0 {
			continue
		}
		// Account for the existing next rank rather than blindly widening every
		// fan-out. Node identities are deduplicated when branches share a helper.
		grandchildren := make(map[int]bool)
		for _, id := range branches {
			for _, child := range outgoing[edges[id].to] {
				target := edges[child].to
				if !children[target] && !grandchildren[target] {
					second += heightOf(target)
					grandchildren[target] = true
				}
			}
		}
		sort.SliceStable(candidates, func(i, j int) bool {
			return heightOf(edges[candidates[i]].to) > heightOf(edges[candidates[j]].to)
		})
		for _, id := range candidates {
			height := heightOf(edges[id].to)
			if second+height >= first {
				continue
			}
			edges[id].minlen = 2
			first -= height
			second += height
		}
	}
}

// Relative panel height in text rows, not an attempt to reproduce Graphviz's
// font metrics. Use the same estimate in both modes to keep packing stable when
// context annotations are toggled. A scope group's height is its tallest panel.
func browserCardHeight(layout flowLayout) int {
	height := 3 // header
	for _, group := range layout.groups {
		rows := 0
		for _, scope := range group {
			rows = max(rows, len(layout.scopes[scope].rows))
		}
		for row := 0; row < rows; row++ {
			band := 0
			for _, scope := range group {
				if row >= len(layout.scopes[scope].rows) {
					continue
				}
				for _, panel := range layout.scopes[scope].rows[row] {
					band = max(band, panel.last-panel.first+3)
				}
			}
			height += band
		}
	}
	return height
}
