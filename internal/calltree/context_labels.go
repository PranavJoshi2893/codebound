package calltree

import (
	"slices"
	"strings"
)

type contextObservation struct {
	origin, scope string
}

func contextFunctionSummary(info *ContextInfo) []string {
	if info == nil {
		return nil
	}
	var lines []string
	seen := make(map[string]bool)
	observed := make(map[contextObservation]bool)
	for _, operation := range info.Operations {
		if operation.Kind == "observe" {
			observed[contextObservation{operation.Context, operation.Scope}] = true
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
				if observed[contextObservation{operation.Context, operation.Scope}] {
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
