package calltree

import (
	"context"
	"go/token"
	"sort"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/ssa"
)

// Raw reachability is independent of presentation. In particular a dependency,
// closure, or synthetic wrapper is never a traversal barrier.
type reachableCalls struct {
	functions []*ssa.Function
	out       map[*ssa.Function][]*callgraph.Edge
}

func walkCalls(ctx context.Context, graph *callgraph.Graph, root *ssa.Function) (*reachableCalls, error) {
	raw := &reachableCalls{out: make(map[*ssa.Function][]*callgraph.Edge)}
	pending := []*ssa.Function{root}
	seen := make(map[*ssa.Function]bool)
	for len(pending) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		fn := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if seen[fn] {
			continue
		}
		seen[fn] = true
		raw.functions = append(raw.functions, fn)
		edges := sortedEdges(graph.Nodes[fn])
		raw.out[fn] = edges
		for i := len(edges) - 1; i >= 0; i-- {
			pending = append(pending, edges[i].Callee.Func)
		}
	}
	return raw, nil
}

func (a *Analysis) collectCalls(ctx context.Context, graph *callgraph.Graph, root *ssa.Function) (*Graph, error) {
	raw, err := walkCalls(ctx, graph, root)
	if err != nil {
		return nil, err
	}
	return a.projectCalls(ctx, graph, raw, root)
}

// Preserve source order, with a stable tie-breaker for multiple possible targets.
func sortedEdges(node *callgraph.Node) []*callgraph.Edge {
	if node == nil {
		return nil
	}
	edges := append([]*callgraph.Edge(nil), node.Out...)
	sort.SliceStable(edges, func(i, j int) bool {
		left, right := callPosition(edges[i]), callPosition(edges[j])
		if left != right {
			return left < right
		}
		return edges[i].Callee.Func.String() < edges[j].Callee.Func.String()
	})
	return edges
}

func callPosition(edge *callgraph.Edge) token.Pos {
	if edge.Site == nil {
		return token.NoPos
	}
	return edge.Site.Pos()
}
