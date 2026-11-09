package diagram

import "github.com/PranavJoshi2893/codebound/internal/calltree/model"

const (
	maxFlowColumns        = 3
	flowStepsBeforeColumn = 5
)

// Layout is independent of analysis: panels are contiguous slices of a basic
// block, not new execution blocks. Only labeled exits/continuations define order.
type flowPanel struct {
	block       int // Index in the owning scope, not its block ID.
	first, last int // Half-open step range.
}

type flowScopeLayout struct {
	scope   model.FlowScope
	rows    [][]flowPanel
	columns int
}

type flowLayout struct {
	scopes  []flowScopeLayout
	groups  [][]int // Independent scope panels sharing a horizontal row.
	columns int
}

func planFlowLayout(flow *model.FunctionFlow) flowLayout {
	plan := flowLayout{columns: 1}
	if flow == nil {
		return plan
	}
	var group []int
	width := 0
	flush := func() {
		if len(group) > 0 {
			plan.groups = append(plan.groups, group)
			plan.columns = max(plan.columns, width)
			group, width = nil, 0
		}
	}
	for _, scope := range flow.Scopes {
		layout := planFlowScope(scope)
		id := len(plan.scopes)
		plan.scopes = append(plan.scopes, layout)
		if len(layout.rows) == 0 {
			continue
		}
		if width+layout.columns > maxFlowColumns {
			flush()
		}
		group = append(group, id)
		width += layout.columns
	}
	flush()
	return plan
}

func planFlowScope(scope model.FlowScope) flowScopeLayout {
	plan := flowScopeLayout{scope: scope, columns: 1}
	byID := make(map[int]int, len(scope.Blocks))
	for i, block := range scope.Blocks {
		byID[block.ID] = i
	}
	// Normal entry, recovery, and disconnected regions remain separate. A
	// recovery block must never look like the next ordinary execution step.
	roots := []int{scope.Entry}
	if scope.Recover != nil {
		roots = append(roots, *scope.Recover)
	}
	for _, block := range scope.Blocks {
		roots = append(roots, block.ID)
	}
	seen := make([]bool, len(scope.Blocks))
	appendPanels := func(panels []flowPanel) {
		for len(panels) > 0 {
			count := min(maxFlowColumns, len(panels))
			plan.rows = append(plan.rows, panels[:count])
			plan.columns = max(plan.columns, count)
			panels = panels[count:]
		}
	}
	for _, root := range roots {
		entry, exists := byID[root]
		if !exists || seen[entry] {
			continue
		}
		seen[entry] = true
		region := []int{entry}
		for i := 0; i < len(region); i++ {
			for _, next := range scope.Blocks[region[i]].Next {
				if target, exists := byID[next.To]; exists && !seen[target] {
					seen[target] = true
					region = append(region, target)
				}
			}
		}
		ranks, acyclic := flowRanks(scope, region, byID)
		if !acyclic {
			// Incomplete/custom evidence may omit back-edge flags. Preserve all
			// blocks without inventing a parallel layout for an unknown cycle.
			for _, block := range region {
				appendPanels(splitFlowBlock(block, len(scope.Blocks[block].Steps)))
			}
			continue
		}
		var levels [][]flowPanel
		for _, block := range region {
			for len(levels) <= ranks[block] {
				levels = append(levels, nil)
			}
			levels[ranks[block]] = append(levels[ranks[block]], splitFlowBlock(block, len(scope.Blocks[block].Steps))...)
		}
		for _, panels := range levels {
			appendPanels(panels)
		}
	}
	// Branch siblings may already require several columns. Reuse that width
	// for a preceding/joining straight-line block instead of leaving its panel
	// stretched across the row while all of its steps still stack vertically.
	if plan.columns > 1 {
		for i, row := range plan.rows {
			if len(row) != 1 {
				continue
			}
			panel := row[0]
			steps := len(scope.Blocks[panel.block].Steps)
			if panel.first == 0 && panel.last == steps && steps > 2 {
				plan.rows[i] = flowBlockPanels(panel.block, steps, min(plan.columns, (steps+1)/2))
			}
		}
	}
	return plan
}

// Longest-path ranks put a join AFTER every incoming branch, not just after the
// shortest branch. Back edges retain their explicit loop exits but do not make
// the visual ranking cyclic. BFS region order keeps true/false siblings stable.
func flowRanks(scope model.FlowScope, region []int, byID map[int]int) ([]int, bool) {
	members := make(map[int]bool, len(region))
	for _, block := range region {
		members[block] = true
	}
	indegree := make([]int, len(scope.Blocks))
	ranks := make([]int, len(scope.Blocks))
	for _, block := range region {
		for _, next := range scope.Blocks[block].Next {
			if target, exists := byID[next.To]; exists && members[target] && !next.BackEdge {
				indegree[target]++
			}
		}
	}
	var queue []int
	for _, block := range region {
		if indegree[block] == 0 {
			queue = append(queue, block)
		}
	}
	for i := 0; i < len(queue); i++ {
		block := queue[i]
		for _, next := range scope.Blocks[block].Next {
			target, exists := byID[next.To]
			if !exists || !members[target] || next.BackEdge {
				continue
			}
			ranks[target] = max(ranks[target], ranks[block]+1)
			indegree[target]--
			if indegree[target] == 0 {
				queue = append(queue, target)
			}
		}
	}
	return ranks, len(queue) == len(region)
}

func splitFlowBlock(block, steps int) []flowPanel {
	parts := min(maxFlowColumns, max(1, (steps+flowStepsBeforeColumn-1)/flowStepsBeforeColumn))
	return flowBlockPanels(block, steps, parts)
}

func flowBlockPanels(block, steps, parts int) []flowPanel {
	if parts == 1 {
		return []flowPanel{{block: block, last: steps}}
	}
	perPanel := (steps + parts - 1) / parts
	var panels []flowPanel
	for first := 0; first < steps; first += perPanel {
		panels = append(panels, flowPanel{block: block, first: first, last: min(steps, first+perPanel)})
	}
	return panels
}
