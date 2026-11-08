package calltree

import (
	"context"
	"go/token"
	"sort"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/ssa"
)

// A frame remembers the next call to visit for one function, avoiding recursion.
type traversalFrame struct {
	function  *ssa.Function
	edges     []*callgraph.Edge
	nextIndex int
}

func (a *Analysis) collectCalls(ctx context.Context, graph *callgraph.Graph, root *ssa.Function) (*Graph, error) {
	result := &Graph{Root: a.describeFunction(root), Calls: []Call{}}
	// A cached description also marks a function as discovered. Each function
	// is converted and expanded once, even when many callers share it.
	descriptions := map[*ssa.Function]Function{root: result.Root}
	stack := []traversalFrame{{function: root, edges: sortedEdges(graph.Nodes[root])}}
	for len(stack) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		frame := &stack[len(stack)-1]
		edge, ok := frame.nextEdge()
		if !ok {
			stack[len(stack)-1] = traversalFrame{}
			stack = stack[:len(stack)-1]
			continue
		}

		calleeFunction := edge.Callee.Func
		if _, ok := a.projectDeclaration(calleeFunction); !ok {
			continue
		}
		callee, discovered := descriptions[calleeFunction]
		if !discovered {
			callee = a.describeFunction(calleeFunction)
			descriptions[calleeFunction] = callee
		}

		throughInterface := edge.Site != nil && edge.Site.Common().IsInvoke()
		caller := descriptions[frame.function]
		result.Calls = append(result.Calls, Call{
			Caller:           caller,
			Callee:           callee,
			ThroughInterface: throughInterface,
			Violation:        boundaryViolation(caller, callee, throughInterface),
		})
		// Keep calls to already discovered functions (including recursion),
		// but do not expand their bodies again.
		if !discovered {
			stack = append(stack, traversalFrame{
				function: calleeFunction,
				edges:    sortedEdges(graph.Nodes[calleeFunction]),
			})
		}
	}
	return result, nil
}

func (f *traversalFrame) nextEdge() (*callgraph.Edge, bool) {
	if f.nextIndex == len(f.edges) {
		return nil, false
	}
	edge := f.edges[f.nextIndex]
	f.nextIndex++
	return edge, true
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
