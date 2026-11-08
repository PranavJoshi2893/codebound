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
	hasContexts := false
	addFunction := func(function Function) {
		if function.Context == nil || visited[function.id()] {
			return
		}
		visited[function.id()] = true
		descriptions = append(descriptions, function)
		hasContexts = true
		for _, operation := range function.Context.Operations {
			if operation.Kind != "create" && operation.Kind != "derive" && operation.Kind != "detach" {
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
	if !hasContexts {
		return nil
	}
	for _, function := range descriptions {
		if function.Context == nil {
			continue
		}
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
	for index, marker := range markers {
		marker.parentMayCancel = contextParentMayCancel(marker.operation, markers, byID)
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

// Background, TODO, and WithoutCancel cannot cancel their children. WithValue
// adds no cancellation of its own, so follow its parents instead of guessing.
func contextParentMayCancel(operation ContextOperation, markers []contextMarker, byID map[string]int) bool {
	if operation.ParentUnresolved || (operation.Parent != "" && len(operation.ParentIDs) == 0) {
		return true
	}
	pending := append([]string(nil), operation.ParentIDs...)
	seen := make(map[string]bool)
	for len(pending) > 0 {
		id := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if seen[id] {
			continue
		}
		seen[id] = true
		index, exists := byID[id]
		if !exists {
			return true // Outside-tree inputs can be cancellable.
		}
		parent := markers[index].operation
		switch parent.Target {
		case "context.Background", "context.TODO", "context.WithoutCancel":
		case "context.WithValue":
			if parent.ParentUnresolved || len(parent.ParentIDs) == 0 {
				return true
			}
			pending = append(pending, parent.ParentIDs...)
		default:
			return true
		}
	}
	return false
}

func contextFunctionSummary(info *ContextInfo) []string {
	if info == nil {
		return nil
	}
	var lines []string
	seen := make(map[string]bool)
	observed := make(map[string]bool)
	for _, operation := range info.Operations {
		if operation.Kind == "observe" {
			observed[operation.Context+"\x00"+operation.Scope] = true
		}
	}
	for index, parameter := range info.Parameters {
		label := "unresolved origin"
		if index < len(info.Bindings) {
			binding := info.Bindings[index]
			label = contextIDsLabel(binding.IDs, binding.Unresolved)
			if slices.ContainsFunc(binding.IDs, func(id string) bool { return strings.HasPrefix(id, "I") }) {
				label += " (origin outside tree)"
			}
		}
		line := parameter + ": " + label
		lines = append(lines, line)
		seen[line] = true
	}
	for _, operation := range info.Operations {
		ids := contextIDsLabel(operation.IDs, operation.Unresolved)
		var action string
		switch operation.Kind {
		case "create", "derive", "detach":
			action = "starts " + ids + ": " + shortContextTarget(operation.Target)
		case "forward":
			action = "passes " + ids + " to " + shortContextTarget(operation.Target)
		case "cancel":
			action = "cancel/stop " + ids
		case "observe":
			action = "observes Done: " + ids
		case "read":
			if operation.Target == "context.Context.Done" {
				// A receive is stronger evidence than merely obtaining a channel.
				if observed[operation.Context+"\x00"+operation.Scope] {
					continue
				}
				action = "gets Done channel: " + ids
			} else {
				action = "reads " + shortContextTarget(operation.Target) + ": " + ids
			}
		}
		if action != "" {
			action += contextScopeLabel(operation)
			if !seen[action] {
				seen[action] = true
				lines = append(lines, action)
			}
		}
	}
	return lines
}

func contextIDsLabel(ids []string, unresolved bool) string {
	if len(ids) == 0 {
		return "unresolved context"
	}
	label := strings.Join(ids, " / ")
	if len(ids) > 1 || unresolved {
		label = "possible " + label
	}
	if unresolved {
		label += " / unknown"
	}
	return label
}

func contextCallLabel(call Call) string {
	var labels []string
	if call.Violation != "" {
		labels = append(labels, call.Violation)
	}
	if call.ThroughInterface {
		labels = append(labels, "interface")
	}
	for _, binding := range call.Contexts {
		labels = append(labels, binding.Parameter+"="+contextIDsLabel(binding.IDs, binding.Unresolved))
	}
	if len(call.Contexts) > 0 && call.Execution != "" {
		labels = append(labels, "["+call.Execution+"]")
	}
	return strings.Join(labels, "\n")
}

func shortContextTarget(target string) string {
	if index := strings.LastIndex(target, "."); index >= 0 {
		return target[index+1:]
	}
	return target
}

func contextScopeLabel(operation ContextOperation) string {
	var labels []string
	if operation.Execution != "" {
		labels = append(labels, operation.Execution)
	}
	if operation.Scope != "" {
		scope := "closure"
		if strings.HasPrefix(operation.Scope, "goroutine ") {
			scope = "goroutine closure"
		} else if strings.HasPrefix(operation.Scope, "deferred ") {
			scope = "deferred closure"
		}
		labels = append(labels, scope)
	}
	if len(labels) == 0 {
		return ""
	}
	return " [" + strings.Join(labels, ", ") + "]"
}

func contextOperationDetails(operation ContextOperation) string {
	lines := []string{operation.Kind + ": " + operation.Target, "context: " + operation.Context}
	for _, detail := range []string{operation.Parent, operation.Detail, operation.Location, operation.Execution, operation.Scope} {
		if detail != "" {
			lines = append(lines, detail)
		}
	}
	return strings.Join(lines, "\n")
}
