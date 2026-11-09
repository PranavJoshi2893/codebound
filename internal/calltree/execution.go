package calltree

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"strings"

	"github.com/PranavJoshi2893/codebound/internal/calltree/model"
	"golang.org/x/tools/go/ssa"
)

// Preserve the compiler's evaluation order and CFG instead of sorting source
// lines: f(g()) executes g first, and short-circuit expressions have branches.
// Value bookkeeping is omitted; calls, channel actions, and all CFG exits remain.
func (a *Analysis) inspectExecution(function *ssa.Function) (*model.FunctionFlow, map[ssa.Instruction]string) {
	sites := make(map[ssa.Instruction]string)
	if len(function.Blocks) == 0 {
		return nil, sites
	}
	source := executionSource{calls: make(map[token.Pos]*ast.CallExpr), expressions: make(map[token.Pos]ast.Expr)}
	source.index(function.Syntax())
	functions := []*ssa.Function{function}
	ids := map[*ssa.Function]string{function: "body"}
	for i := 0; i < len(functions); i++ {
		for _, closure := range functions[i].AnonFuncs {
			ids[closure] = fmt.Sprintf("L%d", len(functions))
			functions = append(functions, closure)
		}
	}
	flow := &model.FunctionFlow{}
	for scopeIndex, fn := range functions {
		if len(fn.Blocks) == 0 {
			continue
		}
		scope := model.FlowScope{ID: ids[fn], Entry: fn.Blocks[0].Index, Location: a.sourceLocation(fn.Pos())}
		if fn.Recover != nil {
			id := fn.Recover.Index
			scope.Recover = &id
		}
		backEdges := executionBackEdges(fn)
		for _, block := range fn.Blocks {
			view := model.FlowBlock{ID: block.Index, Label: strings.ReplaceAll(block.Comment, ".", " "), Exit: "stop"}
			for _, instruction := range block.Instrs {
				step := model.FlowStep{Location: a.sourceLocation(instruction.Pos())}
				switch instruction := instruction.(type) {
				case ssa.CallInstruction:
					step.Kind, step.Label = "call", source.callLabel(instruction)
					switch instruction.(type) {
					case *ssa.Go:
						step.Kind = "go"
					case *ssa.Defer:
						step.Kind = "defer"
					}
					step.Closure = ids[instruction.Common().StaticCallee()]
					if step.Closure == "body" {
						step.Closure = "" // Recursion re-enters the function, not a closure.
					}
				case *ssa.RunDefers:
					step.Kind, step.Label = "run_defers", "run deferred stack · last registered first"
				case *ssa.UnOp:
					if instruction.Op == token.ARROW {
						step.Kind, step.Label = "receive", "receive from channel"
					}
				case *ssa.Send:
					step.Kind, step.Label = "send", "send to channel"
				case *ssa.Select:
					step.Kind, step.Label = "select", "select · one ready case"
					if !instruction.Blocking {
						step.Label += " or default"
					}
				case *ssa.If:
					view.Exit, view.Condition = "branch", source.condition(instruction.Cond)
				case *ssa.Jump:
					view.Exit = "jump"
				case *ssa.Return:
					view.Exit = "return"
				case *ssa.Panic:
					view.Exit = "panic"
				}
				if step.Kind == "" {
					continue
				}
				step.ID = fmt.Sprintf("q%db%ds%d", scopeIndex, block.Index, len(view.Steps)+1)
				step.Reference = fmt.Sprintf("B%d.%d", block.Index, len(view.Steps)+1)
				if scopeIndex > 0 {
					step.Reference = scope.ID + "/" + step.Reference
				}
				sites[instruction] = step.ID
				view.Steps = append(view.Steps, step)
			}
			for i, successor := range block.Succs {
				branch := model.FlowBranch{To: successor.Index, BackEdge: backEdges[[2]int{block.Index, successor.Index}]}
				if view.Exit == "branch" {
					branch.Label = "true"
					if i == 1 {
						branch.Label = "false"
					}
				}
				view.Next = append(view.Next, branch)
			}
			scope.Blocks = append(scope.Blocks, view)
		}
		flow.Scopes = append(flow.Scopes, scope)
	}
	return flow, sites
}

// DFS back edges identify cycles, including goto loops that are not natural
// dominator loops. Disconnected recovery blocks retain their alternative entry.
func executionBackEdges(function *ssa.Function) map[[2]int]bool {
	back := make(map[[2]int]bool)
	colors := make(map[*ssa.BasicBlock]uint8)
	type frame struct {
		block *ssa.BasicBlock
		next  int
	}
	for _, entry := range function.Blocks {
		if colors[entry] != 0 {
			continue
		}
		colors[entry] = 1
		stack := []frame{{block: entry}}
		for len(stack) > 0 {
			current := &stack[len(stack)-1]
			if current.next == len(current.block.Succs) {
				colors[current.block] = 2
				stack = stack[:len(stack)-1]
				continue
			}
			next := current.block.Succs[current.next]
			current.next++
			if colors[next] == 1 {
				back[[2]int{current.block.Index, next.Index}] = true
			} else if colors[next] == 0 {
				colors[next] = 1
				stack = append(stack, frame{block: next})
			}
		}
	}
	return back
}

type executionSource struct {
	calls       map[token.Pos]*ast.CallExpr
	expressions map[token.Pos]ast.Expr
}

func (source executionSource) index(node ast.Node) {
	if node == nil {
		return
	}
	ast.Inspect(node, func(node ast.Node) bool {
		switch node := node.(type) {
		case ast.Expr:
			source.expressions[node.Pos()] = node
		}
		switch node := node.(type) {
		case *ast.CallExpr:
			source.calls[node.Lparen] = node
		case *ast.GoStmt:
			source.calls[node.Go] = node.Call
		case *ast.DeferStmt:
			source.calls[node.Defer] = node.Call
		case *ast.BinaryExpr:
			source.expressions[node.OpPos] = node
		case *ast.UnaryExpr:
			source.expressions[node.OpPos] = node
		}
		return true
	})
}

func (source executionSource) callLabel(site ssa.CallInstruction) string {
	if call := source.calls[site.Pos()]; call != nil {
		name := "closure"
		if _, closure := call.Fun.(*ast.FuncLit); !closure {
			name = executionText(call.Fun)
		}
		if len(call.Args) > 0 {
			return name + "(…)"
		}
		return name + "()"
	}
	if function := site.Common().StaticCallee(); function != nil {
		if _, closure := function.Syntax().(*ast.FuncLit); closure {
			return "closure(…)"
		}
		return function.Name() + "(…)"
	}
	if site.Common().IsInvoke() {
		return site.Common().Method.Name() + "(…)"
	}
	switch value := site.Common().Value.(type) {
	case *ssa.Parameter:
		return value.Name() + "(…)"
	case *ssa.Builtin:
		return value.Name() + "(…)"
	}
	return "callback(…)"
}

func (source executionSource) condition(value ssa.Value) string {
	return shortExecutionText(source.valueLabel(value, 0))
}

func (source executionSource) valueLabel(value ssa.Value, depth int) string {
	if value == nil || depth > 8 {
		return "value"
	}
	if call, ok := value.(ssa.CallInstruction); ok {
		return "result of " + source.callLabel(call)
	}
	switch value := value.(type) {
	case *ssa.BinOp:
		if expression, ok := source.expressions[value.Pos()].(*ast.BinaryExpr); ok && expression.Op == value.Op {
			return executionText(expression)
		}
		return source.valueLabel(value.X, depth+1) + " " + value.Op.String() + " " + source.valueLabel(value.Y, depth+1)
	case *ssa.UnOp:
		if value.Op == token.ARROW {
			return "received value"
		}
		if expression, ok := source.expressions[value.Pos()].(*ast.UnaryExpr); ok && expression.Op == value.Op {
			return executionText(expression)
		}
		return value.Op.String() + source.valueLabel(value.X, depth+1)
	case *ssa.Const:
		if value.Value != nil {
			return value.Value.ExactString()
		}
		return "nil"
	case *ssa.Parameter:
		return value.Name()
	case *ssa.Extract:
		if _, selectCase := value.Tuple.(*ssa.Select); selectCase && value.Index == 0 {
			return "selected case"
		}
	}
	if expression := source.expressions[value.Pos()]; expression != nil {
		return executionText(expression)
	}
	return "value"
}

func executionText(node ast.Node) string {
	var text bytes.Buffer
	if err := format.Node(&text, token.NewFileSet(), node); err != nil {
		return "expression"
	}
	return shortExecutionText(text.String())
}

func shortExecutionText(text string) string {
	result := strings.Join(strings.Fields(text), " ")
	runes := []rune(result)
	if len(runes) > 64 {
		result = string(runes[:61]) + "…"
	}
	return result
}
