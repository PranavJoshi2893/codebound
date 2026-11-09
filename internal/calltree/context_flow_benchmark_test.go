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
