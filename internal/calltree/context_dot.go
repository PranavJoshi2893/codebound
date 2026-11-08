package calltree

import (
	"fmt"
	"io"
	"strings"
)

// Context origins are local to a function. Unresolved origins also need their
// call site: two unrelated "unresolved" values must not become one context.
type contextDOTNode struct {
	owner                   functionID
	source, scope, location string
}

// Group only arrows with identical endpoints, direction, and cancellation kind.
// Red cancellation arrows must remain separate from ordinary context flow.
type contextDOTArrowID struct {
	from, to     string
	cancellation bool
}

type contextDOTArrow struct {
	id          contextDOTArrowID
	labels      []string
	details     []string
	seenLabels  map[string]bool
	seenDetails map[string]bool
}

type contextDOT struct {
	output       io.Writer
	functions    map[functionID]int
	targets      map[string]int
	contexts     map[contextDOTNode]string
	otherTargets map[contextDOTNode]string
	written      []bool
	arrows       []contextDOTArrow
	arrowIDs     map[contextDOTArrowID]int
}

func writeContextDOT(output io.Writer, graph *Graph, functions map[functionID]int) error {
	hasContexts := graph.Root.Context != nil
	for _, call := range graph.Calls {
		if hasContexts {
			break
		}
		hasContexts = call.Caller.Context != nil || call.Callee.Context != nil
	}
	if !hasContexts {
		return nil
	}
	// Let dot reserve space for context nodes and arrow labels instead of
	// forcing them onto their owner's row, where parallel labels collide.
	if _, err := fmt.Fprintln(output, "  graph [nodesep=0.6, ranksep=0.9, pad=0.3];"); err != nil {
		return err
	}
	renderer := contextDOT{
		output: output, functions: functions,
		targets: make(map[string]int), contexts: make(map[contextDOTNode]string),
		otherTargets: make(map[contextDOTNode]string), written: make([]bool, len(functions)),
		arrowIDs: make(map[contextDOTArrowID]int),
	}
	// Index targets before drawing arrows, including calls to functions that
	// appear later in the depth-first call tree.
	renderer.indexTarget(graph.Root)
	for _, call := range graph.Calls {
		renderer.indexTarget(call.Caller)
		renderer.indexTarget(call.Callee)
	}
	if err := renderer.writeFunction(graph.Root); err != nil {
		return err
	}
	for _, call := range graph.Calls {
		if err := renderer.writeFunction(call.Caller); err != nil {
			return err
		}
		if err := renderer.writeFunction(call.Callee); err != nil {
			return err
		}
	}
	if err := renderer.writeArrows(); err != nil {
		return err
	}
	if len(renderer.contexts) > 0 {
		_, err := fmt.Fprintln(output, `  graph [label="Solid arrows: function calls | dashed arrows: context flow", labelloc=b, fontsize=10];`)
		return err
	}
	return nil
}

func (d *contextDOT) indexTarget(function Function) {
	id := d.functions[function.id()]
	names := []string{function.Package + "." + function.Name}
	if receiver, method, ok := strings.Cut(function.Name, "."); ok {
		// SSA prints pointer/value method calls differently from source names.
		names = append(names, "(*"+function.Package+"."+receiver+")."+method, "("+function.Package+"."+receiver+")."+method)
	}
	for _, name := range names {
		if previous, exists := d.targets[name]; exists && previous != id {
			d.targets[name] = -1 // Never guess between ambiguous declarations.
		} else {
			d.targets[name] = id
		}
	}
}

func (d *contextDOT) writeFunction(function Function) error {
	id := d.functions[function.id()]
	if function.Context == nil || d.written[id] {
		return nil
	}
	d.written[id] = true
	functionNode := fmt.Sprintf("n%d", id)
	for _, parameter := range function.Context.Parameters {
		origin := "parameter " + parameter
		node, err := d.sourceNode(function, origin, "", "")
		if err != nil {
			return err
		}
		d.addArrow(node, functionNode, "receives", "Context parameter: "+parameter, false)
	}
	// Register all creation sites first, so references inside closures and
	// branch merges can reuse a context even if it is mentioned earlier.
	for _, operation := range function.Context.Operations {
		switch operation.Kind {
		case "create", "derive", "detach":
			key := contextDOTNode{owner: function.id(), source: operation.Context}
			if _, err := d.contextNode(key, contextNodeLabel(operation), contextOperationDetails(operation), false); err != nil {
				return err
			}
		}
	}
	for _, operation := range function.Context.Operations {
		if err := d.writeOperation(function, operation); err != nil {
			return err
		}
	}
	return nil
}

func (d *contextDOT) writeOperation(function Function, operation ContextOperation) error {
	contextNode, err := d.sourceNode(function, operation.Context, operation.Scope, operation.Location)
	if err != nil {
		return err
	}
	functionNode := fmt.Sprintf("n%d", d.functions[function.id()])
	details := contextOperationDetails(operation)
	switch operation.Kind {
	case "create", "derive", "detach":
		d.addArrow(functionNode, contextNode, contextArrowLabel("creates", operation), details, false)
		if operation.Parent != "" {
			parent, err := d.sourceNode(function, operation.Parent, operation.Scope, operation.Location)
			if err != nil {
				return err
			}
			label := "parent"
			if operation.Kind == "detach" {
				label = "values only\nno cancellation/deadline"
			}
			d.addArrow(parent, contextNode, label, details, false)
		}
	case "forward":
		target, err := d.targetNode(function, operation)
		if err != nil {
			return err
		}
		label := contextArrowLabel("passes context\nvia "+function.Name, operation)
		d.addArrow(contextNode, target, label, details, false)
	case "read":
		label := "reads " + strings.TrimPrefix(operation.Target, "context.Context.")
		if operation.Target == "context.Context.Done" {
			label = "gets Done channel"
		}
		d.addArrow(contextNode, functionNode, contextArrowLabel(label, operation), details, false)
	case "observe":
		d.addArrow(contextNode, functionNode, contextArrowLabel("observes Done", operation), details, false)
	case "cancel":
		d.addArrow(functionNode, contextNode, contextArrowLabel("cancels", operation), details, true)
	}
	return nil
}

func (d *contextDOT) sourceNode(owner Function, origin, scope, location string) (string, error) {
	key := contextDOTNode{owner: owner.id(), source: origin}
	if node, exists := d.contexts[key]; exists {
		return node, nil
	}
	if strings.HasPrefix(origin, "parameter ") && !strings.Contains(origin, " or ") {
		return d.contextNode(key, strings.TrimPrefix(origin, "parameter ")+"\ncontext input", origin, false)
	}
	key.scope, key.location = scope, location
	if scope == "" && location == "" {
		key.location = fmt.Sprintf("unknown site %d", len(d.contexts))
	}
	label := "possible context source\n" + origin
	if origin == "unresolved" || origin == "" {
		label = "unresolved context"
	} else if origin == "nil" {
		label = "nil context"
	}
	return d.contextNode(key, label, origin+"\n"+location+"\n"+scope, true)
}

func (d *contextDOT) contextNode(key contextDOTNode, label, details string, uncertain bool) (string, error) {
	if node, exists := d.contexts[key]; exists {
		return node, nil
	}
	node := fmt.Sprintf("c%d", len(d.contexts))
	d.contexts[key] = node
	shape := "ellipse"
	if uncertain {
		shape = "diamond"
	}
	_, err := fmt.Fprintf(d.output, "  %s [shape=%s, color=\"#2563eb\", label=%q, tooltip=%q];\n", node, shape, label, details)
	return node, err
}

func (d *contextDOT) targetNode(owner Function, operation ContextOperation) (string, error) {
	if id, exists := d.targets[operation.Target]; exists && id >= 0 {
		return fmt.Sprintf("n%d", id), nil
	}
	// Keep unmatched endpoints distinct from actual project declarations.
	key := contextDOTNode{owner: owner.id(), source: operation.Target, scope: operation.Scope}
	if operation.Target == "unresolved callback" || operation.Target == "" {
		key.location = operation.Location
		if key.location == "" {
			key.location = fmt.Sprintf("unknown site %d", len(d.otherTargets))
		}
	}
	if node, exists := d.otherTargets[key]; exists {
		return node, nil
	}
	node := fmt.Sprintf("t%d", len(d.otherTargets))
	d.otherTargets[key] = node
	label := "unmatched context target\n" + operation.Target
	_, err := fmt.Fprintf(d.output, "  %s [shape=note, style=dashed, label=%q];\n", node, label)
	return node, err
}

func (d *contextDOT) addArrow(from, to, label, details string, cancellation bool) {
	id := contextDOTArrowID{from: from, to: to, cancellation: cancellation}
	index, exists := d.arrowIDs[id]
	if !exists {
		index = len(d.arrows)
		d.arrowIDs[id] = index
		d.arrows = append(d.arrows, contextDOTArrow{
			id: id, seenLabels: make(map[string]bool), seenDetails: make(map[string]bool),
		})
	}
	arrow := &d.arrows[index]
	if !arrow.seenLabels[label] {
		arrow.seenLabels[label] = true
		arrow.labels = append(arrow.labels, label)
	}
	if details != "" && !arrow.seenDetails[details] {
		arrow.seenDetails[details] = true
		arrow.details = append(arrow.details, details)
	}
}

func (d *contextDOT) writeArrows() error {
	for _, arrow := range d.arrows {
		color := "#2563eb"
		if arrow.id.cancellation {
			color = "#b91c1c"
		}
		// Keep stable source order, unique visible actions, and all distinct
		// source details, without drawing one parallel line per operation.
		label := strings.Join(arrow.labels, "\n")
		details := strings.Join(arrow.details, "\n\n")
		_, err := fmt.Fprintf(d.output, "  %s -> %s [style=dashed, weight=0, color=%q, label=%q, fontsize=11, tooltip=%q];\n", arrow.id.from, arrow.id.to, color, label, details)
		if err != nil {
			return err
		}
	}
	return nil
}

func contextNodeLabel(operation ContextOperation) string {
	label := operation.Target
	switch {
	case operation.Kind == "create":
		label += "\nindependent"
	case operation.Kind == "detach":
		label += "\ncancellation detached"
	case strings.HasPrefix(operation.Detail, "timeout:"), strings.HasPrefix(operation.Detail, "deadline:"):
		label += "\n" + operation.Detail
	}
	if operation.Location != "" {
		label += "\n" + operation.Location
	}
	return label
}

func contextArrowLabel(action string, operation ContextOperation) string {
	if operation.Execution != "" {
		action += " [" + operation.Execution + "]"
	}
	if operation.Scope != "" {
		scope := "closure"
		if strings.HasPrefix(operation.Scope, "goroutine ") {
			scope = "goroutine closure"
		} else if strings.HasPrefix(operation.Scope, "deferred ") {
			scope = "deferred closure"
		}
		action += " [" + scope + "]"
	}
	return action
}

func contextOperationDetails(operation ContextOperation) string {
	lines := []string{operation.Kind + ": " + operation.Target, "context: " + operation.Context}
	if operation.Parent != "" {
		lines = append(lines, "parent: "+operation.Parent)
	}
	for _, detail := range []string{operation.Detail, operation.Location, operation.Execution, operation.Scope} {
		if detail != "" {
			lines = append(lines, detail)
		}
	}
	return strings.Join(lines, "\n")
}
