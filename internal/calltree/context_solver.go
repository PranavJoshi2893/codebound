package calltree

import (
	"context"

	"golang.org/x/tools/go/ssa"
)

type contextValueID struct {
	inspector *contextInspector
	value     ssa.Value
}

// Separate immutable creation sites from parameters whose bindings can grow.
// Local SSA aliases are walked once; solver iterations only merge known sets.
type contextExpression struct {
	fixed      contextIDs
	parameters []*ssa.Parameter
}

func (flow *contextFlow) valueExpression(inspector *contextInspector, value ssa.Value) contextExpression {
	if value == nil {
		return contextExpression{fixed: contextIDs{unresolved: true}}
	}
	key := contextValueID{inspector: inspector, value: value}
	if expression, exists := flow.expressions[key]; exists {
		return expression
	}
	expression := flow.compileSources(inspector, inspector.valueSources(value))
	if flow.expressions == nil {
		flow.expressions = make(map[contextValueID]contextExpression)
	}
	flow.expressions[key] = expression
	return expression
}

func (flow *contextFlow) compileSources(inspector *contextInspector, sources []ssa.Value) contextExpression {
	expression := contextExpression{fixed: contextIDs{unresolved: len(sources) == 0}}
	seen := make(map[*ssa.Parameter]bool)
	for _, source := range sources {
		if parameter, ok := source.(*ssa.Parameter); ok && flow.parameters[parameter] != nil {
			if !seen[parameter] {
				seen[parameter] = true
				expression.parameters = append(expression.parameters, parameter)
			}
			continue
		}
		call, ok := source.(*ssa.Call)
		if extract, isExtract := source.(*ssa.Extract); isExtract && (extract.Index == 0 || extract.Index == 1) {
			call, ok = extract.Tuple.(*ssa.Call)
		}
		if ok {
			origin := inspector.createdContext(call.Common(), call.Pos())
			if id := flow.sources[contextSourceID{owner: inspector.owner, origin: origin}]; id != "" {
				if expression.fixed.ids == nil {
					expression.fixed.ids = make(map[string]bool)
				}
				expression.fixed.ids[id] = true
				continue
			}
		}
		expression.fixed.unresolved = true
	}
	return expression
}

func (expression contextExpression) evaluate(parameters map[*ssa.Parameter]*contextIDs) contextIDs {
	var values contextIDs
	values.merge(expression.fixed)
	for _, parameter := range expression.parameters {
		values.merge(*parameters[parameter])
	}
	return values
}

type contextRule struct {
	parameter  *ssa.Parameter
	expression contextExpression
}

type contextPropagation struct {
	parameters map[*ssa.Parameter]*contextIDs
	rules      []contextRule
	dependents map[*ssa.Parameter][]int
	pending    []int
	queued     []bool
}

func (flow *contextFlow) compileArguments(ctx context.Context) (contextPropagation, error) {
	propagation := contextPropagation{
		parameters: flow.parameters, rules: make([]contextRule, 0, len(flow.arguments)),
		dependents: make(map[*ssa.Parameter][]int), queued: make([]bool, len(flow.arguments)),
		pending: make([]int, 0, len(flow.arguments)),
	}
	for index, argument := range flow.arguments {
		if err := ctx.Err(); err != nil {
			return contextPropagation{}, err
		}
		expression := flow.valueExpression(argument.inspector, argument.value)
		propagation.rules = append(propagation.rules, contextRule{parameter: argument.parameter, expression: expression})
		for _, source := range expression.parameters {
			propagation.dependents[source] = append(propagation.dependents[source], index)
		}
	}
	// A stack keeps pending storage bounded by the number of rules. Seed it in
	// reverse order so the initial visit follows the existing source order.
	for index := len(propagation.rules) - 1; index >= 0; index-- {
		propagation.enqueue(index)
	}
	return propagation, nil
}

func (propagation *contextPropagation) enqueue(index int) {
	if !propagation.queued[index] {
		propagation.queued[index] = true
		propagation.pending = append(propagation.pending, index)
	}
}

func (propagation *contextPropagation) enqueueDependents(parameter *ssa.Parameter) {
	for _, index := range propagation.dependents[parameter] {
		propagation.enqueue(index)
	}
}

func (propagation *contextPropagation) run(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for len(propagation.pending) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		last := len(propagation.pending) - 1
		index := propagation.pending[last]
		propagation.pending = propagation.pending[:last]
		propagation.queued[index] = false
		rule := propagation.rules[index]
		target := propagation.parameters[rule.parameter]
		changed := target.merge(rule.expression.fixed)
		for _, source := range rule.expression.parameters {
			if target.merge(*propagation.parameters[source]) {
				changed = true
			}
		}
		if changed {
			propagation.enqueueDependents(rule.parameter)
		}
	}
	return nil
}
