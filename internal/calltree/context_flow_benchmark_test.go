package calltree

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"golang.org/x/tools/go/ssa"
)

// Reverse-ordered rules are intentionally hostile to whole-graph rescanning:
// a single input can advance only one hop per pass in that implementation.
func BenchmarkContextFlow(b *testing.B) {
	for _, size := range []int{100, 1000} {
		for _, reverse := range []bool{false, true} {
			b.Run(fmt.Sprintf("size=%d/reverse=%t", size, reverse), func(b *testing.B) {
				b.ReportAllocs()
				for index := 0; index < b.N; index++ {
					b.StopTimer()
					flow, last := contextFlowChain(size, reverse)
					b.StartTimer()
					if err := flow.solve(context.Background()); err != nil {
						b.Fatal(err)
					}
					if !last.ids["I1"] || last.unresolved {
						b.Fatal("context input did not reach the final parameter")
					}
				}
			})
		}
	}
}

func contextFlowChain(size int, reverse bool) (*contextFlow, *contextIDs) {
	parameters := make([]ssa.Parameter, size+1)
	flow := &contextFlow{parameters: make(map[*ssa.Parameter]*contextIDs)}
	inspector := &contextInspector{}
	for index := range parameters {
		flow.parameters[&parameters[index]] = &contextIDs{ids: make(map[string]bool)}
		if index > 0 {
			flow.arguments = append(flow.arguments, contextArgument{
				parameter: &parameters[index], value: &parameters[index-1], inspector: inspector,
			})
		}
	}
	flow.parameters[&parameters[0]].ids["I1"] = true
	if reverse {
		slices.Reverse(flow.arguments)
	}
	return flow, flow.parameters[&parameters[size]]
}

func BenchmarkContextParents(b *testing.B) {
	for _, size := range []int{100, 1000} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			operations, byID := contextParentChain(size)
			b.ReportAllocs()
			b.ResetTimer()
			for index := 0; index < b.N; index++ {
				for _, mayCancel := range contextParentCancellation(operations, byID) {
					if mayCancel {
						b.Fatal("WithValue must not introduce cancellation above Background")
					}
				}
			}
		})
	}
}

func contextParentChain(size int) ([]*ContextOperation, map[string]int) {
	operations := make([]*ContextOperation, size+1)
	byID := make(map[string]int)
	for index := range operations {
		id := fmt.Sprintf("C%d", index)
		byID[id] = index
		operation := ContextOperation{Target: "context.Background", IDs: []string{id}}
		if index > 0 {
			operation.Target = "context.WithValue"
			operation.ParentIDs = []string{fmt.Sprintf("C%d", index-1)}
		}
		operations[index] = &operation
	}
	return operations, byID
}
