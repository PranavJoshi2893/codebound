package diagram

import (
	"bufio"
	"fmt"
	"io"
	"path"
	"sort"
	"strconv"

	"github.com/PranavJoshi2893/codebound/internal/calltree/model"
)

// BrowserOptions controls presentation only; exports and analysis stay intact.
type BrowserOptions struct {
	Contexts bool
	Focus    int // -1 shows everything; otherwise show this function's direct calls/callers.
}

// WriteBrowserDOT reuses declaration identities, incident calls, and relationship
// counts. A focused query visits only the selected function's neighborhood. This
// compact browser presentation does not change the detailed DOT/JSON exports.
func (index *IndexedGraph) WriteBrowserDOT(writer io.Writer, options BrowserOptions) error {
	if len(index.Functions) == 0 || options.Focus < -1 || options.Focus >= len(index.Functions) {
		return fmt.Errorf("invalid graph focus: %d", options.Focus)
	}
	var visible, calls []int
	var included map[int]int
	filtered := index.graph
	if options.Focus >= 0 {
		calls = index.incident[index.offsets[options.Focus]:index.offsets[options.Focus+1]]
		included = map[int]int{options.Focus: 0}
		pairs := make(map[[2]int]bool)
		for _, id := range calls {
			call := index.Calls[id]
			pair := [2]int{call.From, call.To}
			if !pairs[pair] {
				pairs[pair] = true
				included[call.From]++
				if call.From != call.To {
					included[call.To]++
				}
			}
		}
		for id := range included {
			visible = append(visible, id)
		}
		sort.Ints(visible)
		if options.Contexts {
			filtered = &model.Graph{Root: index.Functions[options.Focus], Calls: make([]model.Call, len(calls))}
			for i, id := range calls {
				filtered.Calls[i] = *index.Calls[id].Call
			}
		}
	}
	output := bufio.NewWriter(writer)
	fmt.Fprintln(output, `digraph CallTree {
  graph [rankdir=TB, splines=polyline, nodesep=0.55, ranksep=0.8, pad=0.3, bgcolor="white", fontname="sans-serif", fontsize=11];
  node [shape=box, style="rounded,filled", fillcolor="#f8fafc", color="#94a3b8", fontcolor="#172033", fontname="sans-serif", fontsize=13, margin="0.2,0.14"];
  edge [color="#475569", fontcolor="#334155", fontname="sans-serif", fontsize=11, arrowsize=0.8];`)
	nodeCount := len(index.Functions)
	if options.Focus >= 0 {
		nodeCount = len(visible)
	}
	for position := 0; position < nodeCount; position++ {
		id := position
		if options.Focus >= 0 {
			id = visible[position]
		}
		function := index.Functions[id]
		label := function.Name + "\n" + shortPackage(function.Package)
		color, fill := "#94a3b8", "#f8fafc"
		penwidth := "1"
		if id == 0 {
			label = "START HERE · root\n" + label
			color, fill, penwidth = "#2563eb", "#dbeafe", "2.5"
		}
		if id == options.Focus {
			label = "FOCUS\n" + label
			if id != 0 {
				color, fill, penwidth = "#b45309", "#fef3c7", "2.5"
			}
		}
		if function.Layer != "" {
			label += "\nLayer: " + function.Layer
		}
		if options.Contexts {
			summaries := contextFunctionSummary(function.Context)
			for _, summary := range summaries[:min(3, len(summaries))] {
				label += "\n" + summary
			}
			if len(summaries) > 3 {
				label += fmt.Sprintf("\n+ %d context findings in details", len(summaries)-3)
			}
		}
		if options.Focus >= 0 && index.relations[id] > included[id] {
			label += fmt.Sprintf("\n+ %d other call relationships hidden", index.relations[id]-included[id])
		}
		tooltip := function.Name + "\n" + function.Package + "\n" + function.File + ":" + strconv.Itoa(function.Line)
		line := output.AvailableBuffer()
		line = append(line, "  n"...)
		line = strconv.AppendInt(line, int64(id), 10)
		line = append(line, " [label="...)
		line = strconv.AppendQuote(line, label)
		line = append(line, ", color=\""...)
		line = append(line, color...)
		line = append(line, "\", fillcolor=\""...)
		line = append(line, fill...)
		line = append(line, "\", penwidth="...)
		line = append(line, penwidth...)
		line = append(line, ", URL=\"function:"...)
		line = strconv.AppendInt(line, int64(id), 10)
		line = append(line, "\", tooltip="...)
		line = strconv.AppendQuote(line, tooltip)
		line = append(line, "];\n"...)
		if _, err := output.Write(line); err != nil {
			return err
		}
	}
	seen := make(map[dotCallID]bool)
	callCount := len(index.Calls)
	if options.Focus >= 0 {
		callCount = len(calls)
	}
	for position := 0; position < callCount; position++ {
		id := position
		if options.Focus >= 0 {
			id = calls[position]
		}
		indexed := index.Calls[id]
		call := indexed.Call
		from, to := indexed.From, indexed.To
		label := "calls"
		switch call.Execution {
		case "goroutine":
			label = "starts goroutine"
		case "deferred":
			label = "defers call until return"
		}
		if from == to {
			label += " (recursive)"
		}
		style, color := "solid", "#475569"
		if call.ThroughInterface {
			label += "\npossible interface target"
			style = "dotted"
		}
		if call.Violation != "" {
			label += "\narchitecture warning"
			color = "#b91c1c"
		}
		if options.Contexts {
			for _, binding := range call.Contexts {
				label += "\npasses " + binding.Parameter + "=" + contextIDsLabel(binding.IDs, binding.Unresolved)
			}
		}
		key := dotCallID{from: from, to: to, contextLabel: label, violation: call.Violation}
		if seen[key] {
			continue
		}
		seen[key] = true
		line := output.AvailableBuffer()
		line = append(line, "  n"...)
		line = strconv.AppendInt(line, int64(from), 10)
		line = append(line, " -> n"...)
		line = strconv.AppendInt(line, int64(to), 10)
		line = append(line, " [label="...)
		line = strconv.AppendQuote(line, label)
		line = append(line, ", style=\""...)
		line = append(line, style...)
		line = append(line, "\", color=\""...)
		line = append(line, color...)
		line = append(line, "\", fontcolor=\""...)
		line = append(line, color...)
		line = append(line, "\", tooltip="...)
		line = strconv.AppendQuote(line, call.Violation)
		line = append(line, "];\n"...)
		if _, err := output.Write(line); err != nil {
			return err
		}
	}
	if options.Contexts {
		ids := make(map[model.FunctionID]int, nodeCount)
		if options.Focus >= 0 {
			for _, id := range visible {
				ids[index.Functions[id].Identity()] = id
			}
		} else {
			for id, function := range index.Functions {
				ids[function.Identity()] = id
			}
		}
		if err := writeContextDOT(output, filtered, ids); err != nil {
			return err
		}
	}
	// Guidance belongs in the stable dashboard chrome. A long legend inside the
	// SVG changes its bounding box and makes otherwise identical nodes shrink.
	fmt.Fprintln(output, "  graph [label=\"\"];\n}")
	return output.Flush()
}

func shortPackage(pkg string) string {
	parent := path.Base(path.Dir(pkg))
	if parent == "." || parent == "/" {
		return pkg
	}
	return parent + "/" + path.Base(pkg)
}
