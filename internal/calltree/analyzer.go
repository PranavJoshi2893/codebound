package calltree

import (
	"context"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/vta"
	"golang.org/x/tools/go/ssa"
)

// Analyzer selects the directory and package pattern to analyze.
type Analyzer struct {
	Dir     string
	Pattern string
}

// Analysis holds loaded source so root discovery and graph construction share
// one package load and SSA build.
type Analysis struct {
	dir             string
	program         *ssa.Program
	projectPackages map[string]bool
	functions       map[*ssa.Function]bool
	callGraph       func() *callgraph.Graph
}

func New(dir string) *Analyzer {
	return &Analyzer{Dir: dir, Pattern: "."}
}

// Analyze builds a reachable, module-local graph using Variable Type Analysis
// (VTA). VTA tracks which values can reach a variable to resolve callback and
// interface calls. These are still possible calls, not a runtime trace.
func (a *Analyzer) Analyze(ctx context.Context, rootName string) (*Graph, error) {
	analysis, err := a.Load(ctx)
	if err != nil {
		return nil, err
	}
	return analysis.Analyze(ctx, rootName)
}

// Analyze accepts an explicit root, or discovers a single executable main when
// rootName is empty. Ambiguous discovery returns an EntryPointError.
func (a *Analysis) Analyze(ctx context.Context, rootName string) (*Graph, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	root, err := a.resolveRoot(rootName)
	if err != nil {
		return nil, err
	}
	graph := a.callGraph()
	return a.collectCalls(ctx, graph, root)
}

func (a *Analysis) buildCallGraph() *callgraph.Graph {
	// With nil as the starting graph, VTA first finds possible targets by type,
	// then narrows them by tracking how values flow. This keeps defer stop()
	// from targeting main just because both are func().
	// Dependencies must participate too: they may return callbacks such as stop.
	return vta.CallGraph(a.functions, nil)
}
