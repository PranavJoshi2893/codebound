package calltree

import (
	"context"
	"fmt"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/ssa"
)

type contextCall struct {
	edge  *callgraph.Edge
	index int
}

type contextSourceID struct {
	owner  functionID
	origin string
}

type contextArgument struct {
	parameter *ssa.Parameter
	value     ssa.Value
	inspector *contextInspector
}

// This pass connects existing local findings through actual SSA call arguments.
// It never uses function-name matching or treats a parameter as a new creation.
type contextFlow struct {
	sources     map[contextSourceID]string
	parameters  map[*ssa.Parameter]*contextIDs
	arguments   []contextArgument
	functions   []*ssa.Function
	owners      map[*ssa.Function]*contextInspector
	expressions map[contextValueID]contextExpression
}

func annotateContextFlow(ctx context.Context, callGraph *callgraph.Graph, graph *Graph, descriptions map[*ssa.Function]analyzedFunction, order []*ssa.Function, calls []contextCall) error {
	flow := newContextFlow(descriptions, order)
	if len(flow.owners) == 0 {
		return nil
	}
	flow.seedRootInputs(order[0])
	flow.collectArguments(callGraph)
	if err := flow.solve(ctx); err != nil {
		return err
	}
	flow.annotateFunctions(descriptions, order)
	flow.annotateCalls(graph, calls)
	return ctx.Err()
}

func newContextFlow(descriptions map[*ssa.Function]analyzedFunction, order []*ssa.Function) *contextFlow {
	flow := &contextFlow{
		sources: make(map[contextSourceID]string), parameters: make(map[*ssa.Parameter]*contextIDs),
		owners: make(map[*ssa.Function]*contextInspector),
	}
	for _, function := range order {
		info := descriptions[function]
		if info.contexts == nil {
			continue
		}
		flow.registerFunction(function, info.contexts)
		for _, operation := range info.contexts.info.Operations {
			if !operation.createsContext() {
				continue
			}
			key := contextSourceID{owner: info.description.id(), origin: operation.Context}
			if _, exists := flow.sources[key]; !exists {
				flow.sources[key] = fmt.Sprintf("C%d", len(flow.sources)+1)
			}
		}
	}
	return flow
}

func (flow *contextFlow) seedRootInputs(root *ssa.Function) {
	input := 0
	for _, parameter := range root.Params {
		if values := flow.parameters[parameter]; values != nil {
			input++
			values.ids[fmt.Sprintf("I%d", input)] = true
		}
	}
}

func (flow *contextFlow) collectArguments(callGraph *callgraph.Graph) {
	for _, function := range flow.functions {
		inspector := flow.owners[function]
		if node := callGraph.Nodes[function]; node != nil {
			for _, edge := range sortedEdges(node) {
				if flow.owners[edge.Callee.Func] != nil {
					flow.addArguments(edge, inspector)
				}
			}
		}
	}
}

func (flow *contextFlow) solve(ctx context.Context) error {
	propagation, err := flow.compileArguments(ctx)
	if err != nil {
		return err
	}
	if err := propagation.run(ctx); err != nil {
		return err
	}
	// A cycle or an unsupported invocation may have no known starting value.
	// Mark that uncertainty, then carry it through downstream calls too.
	for parameter, values := range flow.parameters {
		if len(values.ids) == 0 && !values.unresolved {
			values.unresolved = true
			propagation.enqueueDependents(parameter)
		}
	}
	return propagation.run(ctx)
}

func (flow *contextFlow) annotateFunctions(descriptions map[*ssa.Function]analyzedFunction, order []*ssa.Function) {
	for _, function := range order {
		info := descriptions[function]
		if info.contexts == nil {
			continue
		}
		for _, parameter := range function.Params {
			if values := flow.parameters[parameter]; values != nil {
				info.contexts.info.Bindings = append(info.contexts.info.Bindings, values.binding(parameter.Name()))
			}
		}
		for index := range info.contexts.info.Operations {
			operation := &info.contexts.info.Operations[index]
			values := flow.resolveOrigin(info.contexts, operation.Context)
			operation.IDs, operation.Unresolved = values.names(), values.unresolved || len(values.ids) == 0
			if operation.Parent != "" {
				parent := flow.resolveOrigin(info.contexts, operation.Parent)
				operation.ParentIDs, operation.ParentUnresolved = parent.names(), parent.unresolved || len(parent.ids) == 0
			}
		}
	}
}

func (flow *contextFlow) annotateCalls(graph *Graph, calls []contextCall) {
	for _, call := range calls {
		inspector := flow.owners[call.edge.Caller.Func]
		if inspector == nil {
			continue
		}
		arguments := contextArguments(call.edge)
		for index, parameter := range call.edge.Callee.Func.Params {
			if !isContextType(parameter.Type()) {
				continue
			}
			var value ssa.Value
			if index < len(arguments) {
				value = arguments[index]
			}
			values := flow.resolveValue(inspector, value)
			graph.Calls[call.index].Contexts = append(graph.Calls[call.index].Contexts, values.binding(parameter.Name()))
		}
		if len(graph.Calls[call.index].Contexts) > 0 && call.edge.Site != nil {
			graph.Calls[call.index].Execution = contextExecution(call.edge.Site)
		}
	}
}

func (flow *contextFlow) registerFunction(function *ssa.Function, inspector *contextInspector) {
	if flow.owners[function] != nil {
		return
	}
	flow.owners[function] = inspector
	flow.functions = append(flow.functions, function)
	for _, parameter := range function.Params {
		if isContextType(parameter.Type()) {
			flow.parameters[parameter] = &contextIDs{ids: make(map[string]bool)}
		}
	}
	for _, closure := range function.AnonFuncs {
		flow.registerFunction(closure, inspector)
	}
}

func (flow *contextFlow) addArguments(edge *callgraph.Edge, inspector *contextInspector) {
	arguments := contextArguments(edge)
	for index, parameter := range edge.Callee.Func.Params {
		if flow.parameters[parameter] == nil {
			continue
		}
		var value ssa.Value
		if index < len(arguments) {
			value = arguments[index]
		}
		flow.arguments = append(flow.arguments, contextArgument{parameter: parameter, value: value, inspector: inspector})
	}
}

func contextArguments(edge *callgraph.Edge) []ssa.Value {
	if edge.Site == nil {
		return nil
	}
	call := edge.Site.Common()
	if call.IsInvoke() {
		// Invoke arguments omit the receiver; concrete method parameters don't.
		return append([]ssa.Value{call.Value}, call.Args...)
	}
	return call.Args
}

func (flow *contextFlow) resolveOrigin(inspector *contextInspector, origin string) contextIDs {
	if id := flow.sources[contextSourceID{owner: inspector.owner, origin: origin}]; id != "" {
		return contextIDs{ids: map[string]bool{id: true}}
	}
	sources, exists := inspector.origins[origin]
	if !exists {
		return contextIDs{unresolved: true}
	}
	return flow.compileSources(inspector, sources).evaluate(flow.parameters)
}

func (flow *contextFlow) resolveValue(inspector *contextInspector, value ssa.Value) contextIDs {
	return flow.valueExpression(inspector, value).evaluate(flow.parameters)
}
