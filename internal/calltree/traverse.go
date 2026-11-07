package calltree

import (
	"context"
	"go/ast"
	"go/token"
	"path/filepath"
	"sort"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/ssa"
)

type traversalFrame struct {
	function *ssa.Function
	edges    []*callgraph.Edge
	next     int
}

func collectCalls(ctx context.Context, program *ssa.Program, graph *callgraph.Graph, project map[string]bool, root *ssa.Function, dir string) (*Graph, error) {
	result := &Graph{Root: describeFunction(program, root, dir), Calls: []Call{}}
	// A cached description also marks a function as discovered. Each function
	// is converted and expanded once, even when many callers share it.
	descriptions := map[*ssa.Function]Function{root: result.Root}
	stack := []traversalFrame{{function: root, edges: sortedEdges(graph.Nodes[root])}}
	for len(stack) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		frame := &stack[len(stack)-1]
		if frame.next == len(frame.edges) {
			stack[len(stack)-1] = traversalFrame{}
			stack = stack[:len(stack)-1]
			continue
		}
		edge := frame.edges[frame.next]
		frame.next++
		called := edge.Callee.Func
		if _, ok := projectDeclaration(called, project); !ok {
			continue
		}
		callee, discovered := descriptions[called]
		if !discovered {
			callee = describeFunction(program, called, dir)
			descriptions[called] = callee
		}
		throughInterface := edge.Site != nil && edge.Site.Common().IsInvoke()
		caller := descriptions[frame.function]
		result.Calls = append(result.Calls, Call{
			Caller:           caller,
			Callee:           callee,
			ThroughInterface: throughInterface,
			Violation:        boundaryViolation(caller, callee, throughInterface),
		})
		if !discovered {
			stack = append(stack, traversalFrame{function: called, edges: sortedEdges(graph.Nodes[called])})
		}
	}
	return result, nil
}

// Only projectDeclaration-checked functions reach this conversion.
func describeFunction(program *ssa.Program, function *ssa.Function, dir string) Function {
	declaration := function.Syntax().(*ast.FuncDecl)
	position := program.Fset.Position(declaration.Pos())
	file := position.Filename
	if relative, err := filepath.Rel(dir, file); err == nil {
		file = relative
	}
	result := Function{
		Name:    functionName(declaration),
		Package: function.Pkg.Pkg.Path(),
		File:    file,
		Line:    position.Line,
	}
	result.Layer = inferLayer(result.Package, result.File)
	return result
}

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
