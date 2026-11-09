package diagram

import (
	"bufio"
	"fmt"
	"io"
	"path"
	"sort"
	"strconv"
	"strings"

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
	} else {
		visible = make([]int, len(index.Functions))
		for id := range visible {
			visible[id] = id
		}
	}
	edges := index.browserEdges(calls, options.Focus >= 0)
	index.compactBrowserFanout(edges, visible)
	recursion := localRecursionNotes(edges, index.Functions, options.Contexts)
	boundary := make(map[int]bool)
	if options.Contexts {
		for _, edge := range edges {
			if index.Functions[edge.from].Package != index.Functions[edge.to].Package {
				boundary[edge.from], boundary[edge.to] = true, true
			}
		}
	}
	output := bufio.NewWriter(writer)
	fmt.Fprintln(output, `digraph CallTree {
  graph [rankdir=LR, splines=spline, newrank=true, nodesep=0.6, ranksep=1.1, mclimit=4, remincross=true, pad=0.25, bgcolor="white", fontname="sans-serif", fontsize=11];
  node [shape=box, style="rounded,filled", fillcolor="white", color="#94a3b8", fontcolor="#172033", fontname="sans-serif", fontsize=13, margin="0.16,0.1"];
  edge [color="#64748b", fontcolor="#475569", fontname="sans-serif", fontsize=10, arrowsize=0.7];`)
	// Flat package/lifecycle groups, only for multiple visible functions.
	// Put its package caption on the first function: unlike cluster headings, node
	// labels are protected from incoming/outgoing arrows by Graphviz's routing.
	var packages []string
	packageSeen := make(map[string]bool)
	for _, id := range visible {
		pkg := index.Functions[id].Package
		if !packageSeen[pkg] {
			packages = append(packages, pkg)
			packageSeen[pkg] = true
		}
	}
	captions := browserPackageCaptions(packages)
	writeNode := func(id int, showPackage bool) error {
		function := index.Functions[id]
		label := function.Name
		if showPackage && function.Package != "" {
			label += "\n" + captions[function.Package]
		}
		if phase := index.phase(id).caption(); phase != "" {
			label += "\n" + phase
		}
		color, fill := "#94a3b8", "white"
		penwidth := "1"
		if id == 0 {
			label = "ROOT · " + label
			color, fill, penwidth = "#2563eb", "#dbeafe", "2.5"
		}
		if id == options.Focus && id != 0 {
			color, fill, penwidth = "#b45309", "#fef3c7", "2.5"
		}
		if options.Contexts && function.Flow == nil {
			for _, summary := range browserContextSummary(function.Context, id == 0) {
				label += "\n" + summary
			}
		}
		tooltip := function.Name + "\n" + function.Package + "\n" + function.File + ":" + strconv.Itoa(function.Line)
		if function.Layer != "" {
			tooltip += "\nLayer: " + function.Layer
		}
		if options.Focus >= 0 && index.relations[id] > included[id] {
			hidden := index.relations[id] - included[id]
			label += fmt.Sprintf("\n+%d links", hidden)
			tooltip += fmt.Sprintf("\n%d other call relationships hidden", hidden)
		}
		line := output.AvailableBuffer()
		line = append(line, "  n"...)
		line = strconv.AppendInt(line, int64(id), 10)
		line = append(line, " [label="...)
		if function.Flow != nil {
			line = append(line, '<')
			line = append(line, renderFlowLabel(label, function, options.Contexts, id == 0, recursion[id], index.layouts[id])...)
			line = append(line, ">, margin=0"...)
		} else {
			line = strconv.AppendQuote(line, label)
		}
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
		_, err := output.Write(line)
		return err
	}
	for groupID, group := range index.browserGroups(visible) {
		grouped := group.pkg != "" && len(group.nodes) > 1
		if grouped {
			fmt.Fprintf(output, "  subgraph cluster_package_%d {\n    graph [label=\"\", tooltip=%q, style=\"rounded,filled\", fillcolor=%q, color=\"#e2e8f0\", margin=14];\n", groupID, group.pkg+"\n"+group.phase.caption(), group.phase.fill())
		}
		for position, id := range group.nodes {
			if err := writeNode(id, position == 0 || boundary[id]); err != nil {
				return err
			}
		}
		if grouped {
			fmt.Fprintln(output, "  }")
		}
	}
	for _, edge := range edges {
		if err := edge.writeDOT(output, options.Contexts); err != nil {
			return err
		}
	}
	// Guidance belongs in the stable dashboard chrome. A long legend inside the
	// SVG changes its bounding box and makes otherwise identical nodes shrink.
	fmt.Fprintln(output, "  graph [label=\"\"];\n}")
	return output.Flush()
}

// Creation belongs to its owning function, not a separate graph. Forwarding is
// shown on the existing call arrow. Only secondary local usage is abbreviated.
func browserContextSummary(info *model.ContextInfo, root bool) []string {
	if info == nil {
		return nil
	}
	var summaries []string
	seen := make(map[string]bool)
	for _, operation := range info.Operations {
		if !operation.CreatesContext() {
			continue
		}
		label := "new " + contextIDsLabel(operation.IDs, operation.Unresolved)
		if operation.Kind == "derive" || operation.Kind == "detach" || operation.Parent != "" || len(operation.ParentIDs) > 0 || operation.ParentUnresolved {
			label = contextIDsLabel(operation.IDs, operation.Unresolved) + " ← " + contextIDsLabel(operation.ParentIDs, operation.ParentUnresolved)
		}
		label += " · " + shortContextTarget(operation.Target) + contextScopeLabel(operation)
		if !seen[label] {
			seen[label] = true
			summaries = append(summaries, label)
		}
	}
	if root {
		for _, binding := range info.Bindings {
			summaries = append(summaries, "input "+binding.Parameter+"="+contextIDsLabel(binding.IDs, binding.Unresolved))
		}
	}
	var usage []string
	for _, summary := range contextFunctionSummary(info) {
		if strings.HasPrefix(summary, "cancel/stop ") || strings.HasPrefix(summary, "observes Done:") ||
			strings.HasPrefix(summary, "gets Done channel:") || strings.HasPrefix(summary, "reads ") {
			usage = append(usage, summary)
		}
	}
	summaries = append(summaries, usage[:min(2, len(usage))]...)
	if len(usage) > 2 {
		summaries = append(summaries, fmt.Sprintf("+%d actions in details", len(usage)-2))
	}
	return summaries
}

func shortPackage(pkg string) string {
	parent := path.Base(path.Dir(pkg))
	if parent == "." || parent == "/" {
		return pkg
	}
	return parent + "/" + path.Base(pkg)
}

func browserPackageCaptions(packages []string) map[string]string {
	counts := make(map[string]int)
	for _, pkg := range packages {
		counts[shortPackage(pkg)]++
	}
	captions := make(map[string]string, len(packages))
	for _, pkg := range packages {
		caption := shortPackage(pkg)
		if counts[caption] > 1 {
			caption = pkg // Never disguise distinct modules with the same suffix.
		}
		captions[pkg] = caption
	}
	return captions
}
