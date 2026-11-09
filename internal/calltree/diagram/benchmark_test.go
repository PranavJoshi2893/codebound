package diagram

import (
	"fmt"
	"testing"
)

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
