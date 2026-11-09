package diagram

import "github.com/PranavJoshi2893/codebound/internal/calltree/model"

// Compute inherited cancellation once for all creation sites. Only WithValue
// depends entirely on its parents; other cancellable constructors have their
// own cancellation source. Detached contexts never inherit cancellation.
func contextParentCancellation(operations []*model.ContextOperation, byID map[string]int) []bool {
	canCancel := make([]bool, len(operations))
	dependents := make([][]int, len(operations))
	var pending []int
	for index, operation := range operations {
		switch operation.Target {
		case "context.Background", "context.TODO", "context.WithoutCancel":
		case "context.WithValue":
			canCancel[index] = operation.ParentUnresolved || len(operation.ParentIDs) == 0
			for _, parent := range operation.ParentIDs {
				if parentIndex, exists := byID[parent]; exists {
					dependents[parentIndex] = append(dependents[parentIndex], index)
				} else {
					canCancel[index] = true // An outside-tree input may cancel.
				}
			}
		default:
			canCancel[index] = true
		}
		if canCancel[index] {
			pending = append(pending, index)
		}
	}
	for len(pending) > 0 {
		last := len(pending) - 1
		index := pending[last]
		pending = pending[:last]
		for _, child := range dependents[index] {
			if !canCancel[child] {
				canCancel[child] = true
				pending = append(pending, child)
			}
		}
	}

	parents := make([]bool, len(operations))
	for index, operation := range operations {
		parents[index] = operation.ParentUnresolved || (operation.Parent != "" && len(operation.ParentIDs) == 0)
		for _, parent := range operation.ParentIDs {
			parentIndex, exists := byID[parent]
			if !exists || canCancel[parentIndex] {
				parents[index] = true
			}
		}
	}
	return parents
}
