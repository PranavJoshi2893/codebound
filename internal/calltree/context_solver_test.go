package calltree

import (
	"context"
	"errors"
	"math/rand"
	"reflect"
	"testing"

	"golang.org/x/tools/go/ssa"
)

func TestContextSolverMatchesFullRescan(t *testing.T) {
	random := rand.New(rand.NewSource(42))
	for scenario := 0; scenario < 30; scenario++ {
		parameters := make([]ssa.Parameter, 32)
		inspector := &contextInspector{}
		var arguments []contextArgument
		for index := 0; index < 100; index++ {
			var value ssa.Value
			if random.Intn(5) != 0 {
				value = &parameters[random.Intn(len(parameters))]
			}
			arguments = append(arguments, contextArgument{
				parameter: &parameters[random.Intn(len(parameters))], value: value, inspector: inspector,
			})
		}
		newFlow := func() *contextFlow {
			flow := &contextFlow{parameters: make(map[*ssa.Parameter]*contextIDs), arguments: arguments}
			for index := range parameters {
				flow.parameters[&parameters[index]] = &contextIDs{}
			}
			flow.parameters[&parameters[0]].ids = map[string]bool{"I1": true}
			flow.parameters[&parameters[1]].ids = map[string]bool{"C1": true}
			return flow
		}
		worklist, reference := newFlow(), newFlow()
		if err := worklist.solve(context.Background()); err != nil {
			t.Fatal(err)
		}
		solveByFullRescan(reference)
		for parameter, got := range worklist.parameters {
			want := reference.parameters[parameter]
			if !reflect.DeepEqual(got.names(), want.names()) || got.unresolved != want.unresolved {
				t.Fatalf("scenario %d: worklist=%+v, reference=%+v", scenario, got, want)
			}
		}
	}
}

// Deliberately retain the simpler old algorithm only as an independent oracle.
func solveByFullRescan(flow *contextFlow) {
	propagate := func() {
		for changed := true; changed; {
			changed = false
			for _, argument := range flow.arguments {
				if flow.parameters[argument.parameter].merge(flow.resolveValue(argument.inspector, argument.value)) {
					changed = true
				}
			}
		}
	}
	propagate()
	for _, values := range flow.parameters {
		if len(values.ids) == 0 {
			values.unresolved = true
		}
	}
	propagate()
}

func TestContextSolverReverseChain(t *testing.T) {
	flow, last := contextFlowChain(5000, true)
	if err := flow.solve(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(last.names(), []string{"I1"}) || last.unresolved {
		t.Fatalf("lost input context across reversed dependencies: %+v", last)
	}
}

func TestContextSolverHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, size := range []int{0, 1000} {
		flow, _ := contextFlowChain(size, false)
		if err := flow.solve(ctx); !errors.Is(err, context.Canceled) {
			t.Fatalf("size %d: expected cancellation, got %v", size, err)
		}
	}
}

func TestContextExpressionCacheReadsCurrentBindings(t *testing.T) {
	flow, _ := contextFlowChain(1, false)
	argument := flow.arguments[0]
	first := flow.resolveValue(argument.inspector, argument.value)
	parameter := argument.value.(*ssa.Parameter)
	flow.parameters[parameter].merge(contextIDs{ids: map[string]bool{"C2": true}, unresolved: true})
	second := flow.resolveValue(argument.inspector, argument.value)
	if len(first.ids) != 1 || !reflect.DeepEqual(second.names(), []string{"C2", "I1"}) || !second.unresolved {
		t.Fatal("cached SSA expressions must not freeze or alias mutable parameter bindings")
	}
}
