package calltree

import (
	"fmt"
	"io"
	"slices"
	"strings"
)

// The hybrid view keeps usage in function boxes. Only real creation sites get
// separate markers; parameters and context reads never create extra nodes.
type contextMarker struct {
	owner           Function
	operation       ContextOperation
	cancels         []string
	parentMayCancel bool
}

func writeContextDOT(output io.Writer, graph *Graph, functions map[functionID]int) error {
	var markers []contextMarker
	byID := make(map[string]int)
	visited := make(map[functionID]bool)
	var descriptions []Function
	addFunction := func(function Function) {
		if function.Context == nil || visited[function.id()] {
			return
		}
		visited[function.id()] = true
		descriptions = append(descriptions, function)
		for _, operation := range function.Context.Operations {
			if !operation.createsContext() {
				continue
			}
			if len(operation.IDs) == 1 {
				if _, exists := byID[operation.IDs[0]]; exists {
					continue
				}
				byID[operation.IDs[0]] = len(markers)
			}
			markers = append(markers, contextMarker{owner: function, operation: operation})
		}
	}
	addFunction(graph.Root)
	for _, call := range graph.Calls {
		addFunction(call.Caller)
		addFunction(call.Callee)
	}
	if len(descriptions) == 0 {
		return nil
	}
	for _, function := range descriptions {
		for _, operation := range function.Context.Operations {
			if operation.Kind != "cancel" {
				continue
			}
			condition := "cancel in " + function.Name + contextScopeLabel(operation)
			if len(operation.IDs) > 1 || operation.Unresolved {
				condition += " (possible target)"
			}
			for _, id := range operation.IDs {
				if index, exists := byID[id]; exists && !slices.Contains(markers[index].cancels, condition) {
					markers[index].cancels = append(markers[index].cancels, condition)
				}
			}
		}
	}
	if _, err := fmt.Fprintln(output, "  graph [nodesep=0.6, ranksep=0.9, pad=0.3];"); err != nil {
		return err
	}
	operations := make([]*ContextOperation, len(markers))
	for index := range markers {
		operations[index] = &markers[index].operation
	}
	parentCancellation := contextParentCancellation(operations, byID)
	for index, marker := range markers {
		marker.parentMayCancel = parentCancellation[index]
		if _, err := fmt.Fprintf(output, "  c%d [shape=note, color=\"#2563eb\", label=%q, tooltip=%q];\n", index, marker.label(), contextOperationDetails(marker.operation)); err != nil {
			return err
		}
		id := contextIDsLabel(marker.operation.IDs, marker.operation.Unresolved)
		if _, err := fmt.Fprintf(output, "  n%d -> c%d [style=dashed, weight=0, color=\"#2563eb\", label=%q, fontsize=11];\n", functions[marker.owner.id()], index, "starts "+id+contextScopeLabel(marker.operation)); err != nil {
			return err
		}
		for _, parent := range marker.operation.ParentIDs {
			if parentIndex, exists := byID[parent]; exists && parentIndex != index {
				label := "child of " + parent
				if marker.operation.Kind == "detach" {
					label = "values only; cancellation detached"
				}
				if marker.operation.ParentUnresolved || len(marker.operation.ParentIDs) > 1 {
					label = "possible " + label
				}
				if _, err := fmt.Fprintf(output, "  c%d -> c%d [style=dashed, weight=0, color=\"#2563eb\", label=%q, fontsize=11];\n", parentIndex, index, label); err != nil {
					return err
				}
			}
		}
	}
	_, err := fmt.Fprintln(output, `  graph [label="C: creation site | I: outside-tree input\nCall arrows carry context IDs; cancellation conditions are not runtime end points", labelloc=b, fontsize=10];`)
	return err
}

func (marker contextMarker) label() string {
	operation := marker.operation
	start := marker.owner.Name
	if operation.Scope != "" {
		start = "closure in " + start
	}
	lines := []string{contextIDsLabel(operation.IDs, operation.Unresolved) + " · " + shortContextTarget(operation.Target), "START: " + start, operation.Location}
	if operation.Parent != "" {
		parent := contextIDsLabel(operation.ParentIDs, operation.ParentUnresolved)
		if len(operation.ParentIDs) == 0 {
			parent = "unresolved"
		}
		lines = append(lines, "Parent: "+parent)
		if len(operation.IDs) == 1 && slices.Contains(operation.ParentIDs, operation.IDs[0]) {
			lines = append(lines, "same-site parent: earlier invocation/iteration")
		}
	}
	switch operation.Target {
	case "context.Background", "context.TODO":
		lines = append(lines, "CANCELLATION: none; no deadline")
	case "context.WithoutCancel":
		lines = append(lines, "CANCELLATION: detached; no deadline")
	case "context.WithValue":
		if marker.parentMayCancel {
			lines = append(lines, "CANCELLATION: follows parent")
		} else {
			lines = append(lines, "CANCELLATION: none from parent; no deadline")
		}
	default:
		lines = append(lines, "CAN CANCEL WHEN:")
		if strings.HasPrefix(operation.Detail, "timeout:") || strings.HasPrefix(operation.Detail, "deadline:") {
			lines = append(lines, operation.Detail)
		}
		if operation.Target == "signal.NotifyContext" {
			lines = append(lines, "configured signal arrives")
		}
		if marker.parentMayCancel {
			lines = append(lines, "parent is cancelled (if cancellable)")
		}
		if len(marker.cancels) == 0 {
			lines = append(lines, "cancel/stop function (call not resolved here)")
		} else {
			lines = append(lines, marker.cancels...)
		}
	}
	return strings.Join(lines, "\n")
}
