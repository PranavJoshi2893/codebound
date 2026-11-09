package calltree

import (
	"context"

	"github.com/PranavJoshi2893/codebound/internal/calltree/model"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/ssa"
)

// Retain only corridors that can return to a visible declaration. External
// operations (including SQL) remain execution rows, not expanded library cards.
func (raw *reachableCalls) returningTo(visible map[*ssa.Function]analyzedFunction) map[*ssa.Function]bool {
	incoming := make(map[*ssa.Function][]*ssa.Function)
	for _, fn := range raw.functions {
		for _, edge := range raw.out[fn] {
			incoming[edge.Callee.Func] = append(incoming[edge.Callee.Func], fn)
		}
	}
	useful := make(map[*ssa.Function]bool)
	var pending []*ssa.Function
	for _, fn := range raw.functions {
		if _, ok := visible[fn]; ok {
			useful[fn] = true
			pending = append(pending, fn)
		}
	}
	for len(pending) > 0 {
		fn := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		for _, caller := range incoming[fn] {
			if !useful[caller] {
				useful[caller] = true
				pending = append(pending, caller)
			}
		}
	}
	return useful
}

type projectedCall struct {
	entry, exit *callgraph.Edge
	path        *model.CallPath
}

func (a *Analysis) projectCalls(ctx context.Context, graph *callgraph.Graph, raw *reachableCalls, root *ssa.Function) (*Graph, error) {
	descriptions := make(map[*ssa.Function]analyzedFunction)
	for _, fn := range raw.functions {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if _, ok := a.projectDeclaration(fn); ok {
			descriptions[fn] = a.analyzeFunction(fn)
		}
	}
	useful := raw.returningTo(descriptions)
	evidence, edgeIDs := a.callEvidence(raw, useful, descriptions)
	result := &Graph{Root: descriptions[root].description, Calls: []Call{}}
	var contextCalls []contextCall
	order := []*ssa.Function{root}
	seen := map[*ssa.Function]bool{root: true}
	type frame struct {
		function *ssa.Function
		calls    []projectedCall
		next     int
	}
	stack := []frame{{function: root, calls: projectOutgoing(raw, root, descriptions, useful, edgeIDs)}}
	for len(stack) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		f := &stack[len(stack)-1]
		if f.next == len(f.calls) {
			stack = stack[:len(stack)-1]
			continue
		}
		candidate := f.calls[f.next]
		f.next++
		calleeFn := candidate.exit.Callee.Func
		caller, callee := descriptions[f.function], descriptions[calleeFn]
		throughInterface := candidate.exit.Site != nil && candidate.exit.Site.Common().IsInvoke()
		call := Call{Caller: caller.description, Callee: callee.description, Path: candidate.path, ThroughInterface: throughInterface}
		if candidate.path == nil && candidate.exit.Site != nil {
			call.Dynamic = candidate.exit.Site.Common().StaticCallee() == nil && !throughInterface
		}
		// Architecture checks describe actual calls, never a collapsed corridor.
		if call.Path == nil {
			call.Violation = boundaryViolation(call.Caller, call.Callee, throughInterface)
		}
		if candidate.entry.Site != nil {
			call.Site = caller.sites[candidate.entry.Site]
			call.Execution = contextExecution(candidate.entry.Site)
		}
		result.Calls = append(result.Calls, call)
		contextCalls = append(contextCalls, contextCall{edge: candidate.exit, index: len(result.Calls) - 1})
		if call.Path != nil {
			result.Evidence = evidence
		}
		if !seen[calleeFn] {
			seen[calleeFn] = true
			order = append(order, calleeFn)
			stack = append(stack, frame{function: calleeFn, calls: projectOutgoing(raw, calleeFn, descriptions, useful, edgeIDs)})
		}
	}
	if err := a.annotateReachableContexts(ctx, graph, raw, useful, result, descriptions, order, contextCalls); err != nil {
		return nil, err
	}
	if err := a.inspectRegistrations(ctx, root, descriptions, result); err != nil {
		return nil, err
	}
	return result, nil
}

// For each actual entry call, find exit calls through hidden nodes only. Each
// (hidden function, dispatch state) is expanded once: cycles terminate and shared
// framework paths don't multiply exponentially. Evidence retains all raw edges.
func projectOutgoing(raw *reachableCalls, caller *ssa.Function, visible map[*ssa.Function]analyzedFunction, useful map[*ssa.Function]bool, ids map[*callgraph.Edge]int) []projectedCall {
	var result []projectedCall
	for _, entry := range raw.out[caller] {
		if !useful[entry.Callee.Func] {
			continue
		}
		if _, ok := visible[entry.Callee.Func]; ok {
			result = append(result, projectedCall{entry: entry, exit: entry})
			continue
		}
		type state struct {
			fn   *ssa.Function
			http bool
		}
		type step struct {
			edge *callgraph.Edge
			http bool
		}
		pending := []step{{entry, httpDispatch(entry)}}
		seen := make(map[state]bool)
		type exitID struct {
			edge *callgraph.Edge
			http bool
		}
		exits := make(map[exitID]bool)
		for len(pending) > 0 {
			current := pending[len(pending)-1]
			pending = pending[:len(pending)-1]
			fn := current.edge.Callee.Func
			if !useful[fn] {
				continue
			}
			if _, ok := visible[fn]; ok {
				key := exitID{current.edge, current.http}
				if !exits[key] {
					exits[key] = true
					result = append(result, projectedCall{entry, current.edge, &model.CallPath{Entry: ids[entry], Exit: ids[current.edge], HTTPDispatch: current.http}})
				}
				continue
			}
			key := state{fn, current.http}
			if seen[key] {
				continue
			}
			seen[key] = true
			edges := raw.out[fn]
			for i := len(edges) - 1; i >= 0; i-- {
				edge := edges[i]
				pending = append(pending, step{edge, current.http || httpDispatch(edge)})
			}
		}
	}
	return result
}
