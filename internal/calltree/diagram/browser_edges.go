package diagram

import (
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"

	"github.com/PranavJoshi2893/codebound/internal/calltree/model"
)

// A browser edge is a relationship, not a call site. Keep one route between a
// pair of functions and summarize its possible variants without mutating them.
// Detailed call-site evidence remains in the explorer and DOT/JSON exports.
type browserEdge struct {
	from, to   int
	calls      []*model.Call
	steps      map[string]indexedStep
	backward   bool
	targetFlow bool
	gridSource bool
	minlen     int // layout-only shelf distance; zero uses Graphviz's default
}

func (index *IndexedGraph) browserEdges(calls []int, focused bool) []browserEdge {
	var edges []browserEdge
	pairs := make(map[[2]int]int)
	count := len(index.Calls)
	if focused {
		count = len(calls)
	}
	for position := 0; position < count; position++ {
		id := position
		if focused {
			id = calls[position]
		}
		call := index.Calls[id]
		pair := [2]int{call.From, call.To}
		edge, exists := pairs[pair]
		if !exists {
			edge = len(edges)
			pairs[pair] = edge
			edges = append(edges, browserEdge{from: call.From, to: call.To, steps: index.sites[call.From], targetFlow: index.Functions[call.To].Flow != nil, gridSource: index.layouts[call.From].columns > 1})
		}
		edges[edge].calls = append(edges[edge].calls, call.Call)
	}
	routeBrowserEdges(edges)
	return edges
}

// Rank the acyclic call backbone left-to-right. Feedback calls are still drawn,
// but don't force large cards into a cyclic rank or leave from the forward side.
// This is layout evidence only; it does not change execution/control flow.
func routeBrowserEdges(edges []browserEdge) {
	outgoing := make(map[int][]int)
	var nodes []int
	seen := make(map[int]bool)
	for id, edge := range edges {
		outgoing[edge.from] = append(outgoing[edge.from], id)
		for _, node := range []int{edge.from, edge.to} {
			if !seen[node] {
				seen[node] = true
				nodes = append(nodes, node)
			}
		}
	}
	type frame struct{ node, next int }
	colors := make(map[int]uint8)
	for _, node := range nodes {
		if colors[node] != 0 {
			continue
		}
		colors[node] = 1
		stack := []frame{{node: node}}
		for len(stack) > 0 {
			current := &stack[len(stack)-1]
			if current.next == len(outgoing[current.node]) {
				colors[current.node] = 2
				stack = stack[:len(stack)-1]
				continue
			}
			id := outgoing[current.node][current.next]
			current.next++
			next := edges[id].to
			if colors[next] == 1 {
				edges[id].backward = true
			} else if colors[next] == 0 {
				colors[next] = 1
				stack = append(stack, frame{node: next})
			}
		}
	}
}

func (edge browserEdge) writeDOT(output io.Writer, contexts bool) error {
	if edge.localRecursion() {
		return nil // Rendered at its actual execution row, not as an exterior loop.
	}
	var executions []string
	var direct, indirect, warning bool
	var hidden, dispatch bool
	var callback bool
	var invocation bool
	for _, call := range edge.calls {
		invocation = invocation || call.Path == nil
		hidden = hidden || call.Path != nil
		dispatch = dispatch || (call.Path != nil && call.Path.HTTPDispatch)
		execution := "call"
		switch call.Execution {
		case "goroutine":
			execution = "go"
		case "deferred":
			execution = "defer"
		}
		if !slices.Contains(executions, execution) {
			executions = append(executions, execution)
		}
		indirect = indirect || call.ThroughInterface
		direct = direct || (!call.ThroughInterface && !call.Dynamic && call.Path == nil)
		callback = callback || call.Dynamic
		warning = warning || call.Violation != ""
	}
	var labels []string
	if len(executions) != 1 || executions[0] != "call" {
		labels = append(labels, strings.Join(executions, " / "))
	}
	if edge.from == edge.to {
		labels = append(labels, "recursive")
	}
	style, color, penwidth := "solid", "#64748b", 1.0
	tooltip := "Possible call; call/go/defer variants are listed in function details"
	if indirect {
		tooltip += "\nPossible interface target"
		if direct {
			labels = append(labels, "direct / interface")
		} else {
			style = "dotted"
		}
	}
	if hidden {
		style = "dashed"
		label := "indirect · hidden calls"
		if dispatch {
			label = "possible HTTP dispatch\nindirect"
		}
		if invocation {
			label = "invocation / " + label
		}
		labels = append(labels, label)
		tooltip += "\nIncludes a hidden call path, not a direct invocation; source row is path entry, context bindings are at path exit"
	}
	if callback {
		labels = append(labels, "possible callback target")
		if !direct && !hidden {
			style = "dotted"
		}
	}
	if contexts {
		bindings := edge.contextLabels()
		if len(bindings) > 0 {
			labels = append(labels, bindings...)
			color, penwidth = "#2563eb", 2
		}
	}
	fontcolor := color
	if warning {
		color = "#b91c1c"
		tooltip += "\nArchitecture warning (naming heuristic); see function details"
	}
	sites, complete := edge.flowSites()
	tail, head := "e", "w"
	if edge.backward {
		tail, head = "w", "e"
		if edge.from == edge.to {
			tail = "e" // Keep self-loops away from the incoming header lane.
		}
	}
	if edge.targetFlow {
		head = "entry:" + head
	}
	if len(sites) == 1 && complete && !edge.gridSource {
		if edge.backward && edge.from != edge.to {
			tail = sites[0].ID + "_w:w"
		} else {
			tail = sites[0].ID + ":e"
		}
	} else if len(sites) > 0 {
		var references []string
		for _, site := range sites {
			references = append(references, site.Reference)
		}
		label := "at " + strings.Join(references, ", ")
		if !complete {
			label += " + unpositioned sites"
		}
		labels = append(labels, label)
	}
	labelAttribute := "label"
	if edge.from == edge.to {
		// A self-loop is not a rank-to-rank edge. Place its caption after
		// routing, rather than letting it collide with the loop's own curve.
		labelAttribute = "xlabel"
	}
	caption := strings.Join(labels, "\n")
	if edge.gridSource && caption != "" && edge.from != edge.to {
		// Wide cards route from their border, not an interior row. Reserve a
		// text-free gutter around the caption so the turning spline cannot cut
		// through its first/last line on a tall fan-out. No evidence is dropped.
		caption = " \n" + caption + "\n "
	}
	_, err := fmt.Fprintf(output, "  n%d -> n%d [%s=%q, style=%q, color=%q, fontcolor=%q, penwidth=%.1f, tooltip=%q",
		edge.from, edge.to, labelAttribute, caption, style, color, fontcolor, penwidth, tooltip)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, ", tailport=%q, headport=%q", tail, head); err != nil {
		return err
	}
	if edge.backward {
		if _, err := fmt.Fprint(output, ", constraint=false"); err != nil {
			return err
		}
	}
	if edge.minlen > 1 {
		if _, err := fmt.Fprintf(output, ", minlen=%d", edge.minlen); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintln(output, "];")
	return err
}

func (edge browserEdge) localRecursion() bool {
	if edge.from != edge.to || len(edge.calls) == 0 {
		return false
	}
	for _, call := range edge.calls {
		if call.Site == "" || call.Path != nil {
			return false
		}
		if _, exists := edge.steps[call.Site]; !exists {
			return false
		}
	}
	return true
}

// Recursion stays at its call site inside the owning function. Bindings are
// grouped per SITE, not per relationship, so two recursive invocations do not
// borrow each other's context origins or imply an invented execution position.
func localRecursionNotes(edges []browserEdge, functions []model.Function, contexts bool) map[int]map[string][]string {
	notes := make(map[int]map[string][]string)
	for _, edge := range edges {
		if !edge.localRecursion() {
			continue
		}
		bySite := make(map[string]*browserEdge)
		for _, call := range edge.calls {
			if bySite[call.Site] == nil {
				bySite[call.Site] = &browserEdge{}
			}
			bySite[call.Site].calls = append(bySite[call.Site].calls, call)
		}
		notes[edge.from] = make(map[string][]string)
		entry := functions[edge.from].Flow.Scopes[0].Entry
		for site, relationship := range bySite {
			label := fmt.Sprintf("↻ recursive call → entry B%d", entry)
			for _, call := range relationship.calls {
				if call.ThroughInterface {
					label += " · possible interface target"
					break
				}
			}
			notes[edge.from][site] = []string{label}
			if contexts {
				notes[edge.from][site] = append(notes[edge.from][site], relationship.contextLabels()...)
			}
		}
	}
	return notes
}

// A single invocation can anchor its arrow at that exact row. Repeated calls
// keep one bundled route with explicit site references instead of pretending
// that all invocations occur at the first row (or reintroducing parallel lines).
func (edge browserEdge) flowSites() ([]model.FlowStep, bool) {
	if edge.steps == nil {
		return nil, false
	}
	wanted := make(map[string]bool)
	complete := true
	for _, call := range edge.calls {
		if call.Site == "" {
			complete = false
		} else {
			wanted[call.Site] = true
		}
	}
	var sites []model.FlowStep
	for id := range wanted {
		if site, exists := edge.steps[id]; exists {
			sites = append(sites, site.step)
		} else {
			complete = false
		}
	}
	sort.Slice(sites, func(i, j int) bool { return edge.steps[sites[i].ID].order < edge.steps[sites[j].ID].order })
	return sites, complete
}

// Merge origins per callee parameter, preserving uncertainty and marking gaps
// in the call variants' binding evidence. Position keeps repeated
// blank parameter names separate. Order follows the original analysis evidence.
func (edge browserEdge) contextLabels() []string {
	type parameter struct {
		position int
		name     string
	}
	type binding struct {
		ids        []string
		unresolved bool
		calls      int
	}
	var parameters []parameter
	bindings := make(map[parameter]*binding)
	for _, call := range edge.calls {
		for position, context := range call.Contexts {
			key := parameter{position, context.Parameter}
			merged := bindings[key]
			if merged == nil {
				merged = &binding{}
				bindings[key] = merged
				parameters = append(parameters, key)
			}
			merged.calls++
			merged.unresolved = merged.unresolved || context.Unresolved || len(context.IDs) == 0
			for _, id := range context.IDs {
				if !slices.Contains(merged.ids, id) {
					merged.ids = append(merged.ids, id)
				}
			}
		}
	}
	var labels []string
	for _, key := range parameters {
		merged := bindings[key]
		partial := merged.calls < len(edge.calls)
		label := key.name + "=" + contextIDsLabel(merged.ids, merged.unresolved || partial)
		if partial {
			label += " (partial)"
		}
		labels = append(labels, label)
	}
	return labels
}
