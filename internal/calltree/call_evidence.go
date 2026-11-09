package calltree

import (
	"fmt"
	"go/types"

	"github.com/PranavJoshi2893/codebound/internal/calltree/model"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/ssa"
)

func (a *Analysis) callEvidence(raw *reachableCalls, useful map[*ssa.Function]bool, visible map[*ssa.Function]analyzedFunction) (*model.CallEvidence, map[*callgraph.Edge]int) {
	evidence := &model.CallEvidence{}
	nodes := make(map[*ssa.Function]int)
	owners := make(map[ssa.Instruction]string)
	for _, fn := range raw.functions {
		if !useful[fn] {
			continue
		}
		info, shown := visible[fn]
		nodes[fn] = len(evidence.Nodes)
		evidence.Nodes = append(evidence.Nodes, model.EvidenceFunction{ID: fn.String(), Visible: shown})
		for instruction, site := range info.sites {
			owners[instruction] = site
		}
	}
	ids := make(map[*callgraph.Edge]int)
	for _, fn := range raw.functions {
		if !useful[fn] {
			continue
		}
		instructions := make(map[ssa.Instruction]string)
		for _, block := range fn.Blocks {
			for i, instruction := range block.Instrs {
				instructions[instruction] = fmt.Sprintf("b%di%d", block.Index, i)
			}
		}
		for _, edge := range raw.out[fn] {
			if !useful[edge.Callee.Func] {
				continue
			}
			item := model.EvidenceCall{From: nodes[fn], To: nodes[edge.Callee.Func], HTTPDispatch: httpDispatch(edge)}
			if edge.Site != nil {
				item.Instruction, item.Location, item.Site = instructions[edge.Site], a.sourceLocation(edge.Site.Pos()), owners[edge.Site]
				item.Execution = contextExecution(edge.Site)
				item.ThroughInterface = edge.Site.Common().IsInvoke()
				item.Dynamic = edge.Site.Common().StaticCallee() == nil && !item.ThroughInterface
			}
			ids[edge] = len(evidence.Edges)
			evidence.Edges = append(evidence.Edges, item)
		}
	}
	return evidence, ids
}

// Recognize the actual standard-library server-to-handler invocation by package,
// receiver type, and interface identity. Never infer dispatch from project names
// or the mere presence of a handler argument during route registration.
func httpDispatch(edge *callgraph.Edge) bool {
	fn := edge.Caller.Func
	if fn.Pkg == nil || fn.Pkg.Pkg.Path() != "net/http" || fn.Name() != "ServeHTTP" || fn.Signature.Recv() == nil || edge.Site == nil {
		return false
	}
	receiver, ok := fn.Signature.Recv().Type().(*types.Named)
	if !ok || receiver.Obj().Pkg() == nil || receiver.Obj().Pkg().Path() != "net/http" || receiver.Obj().Name() != "serverHandler" {
		return false
	}
	call := edge.Site.Common()
	if !call.IsInvoke() || call.Method.Name() != "ServeHTTP" {
		return false
	}
	handler, ok := call.Value.Type().(*types.Named)
	return ok && handler.Obj().Pkg() != nil && handler.Obj().Pkg().Path() == "net/http" && handler.Obj().Name() == "Handler"
}
