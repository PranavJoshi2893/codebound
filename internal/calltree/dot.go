package calltree

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type functionID struct {
	name, pkg, file string
	line            int
}

func (f Function) id() functionID {
	return functionID{f.Name, f.Package, f.File, f.Line}
}

// WriteDOT streams plain DOT with only a node-ID index, not a second graph of
// copied nodes and edges. Ordering and quoting match the original exporter.
func WriteDOT(writer io.Writer, graph *Graph) error {
	output := bufio.NewWriter(writer)
	if _, err := output.WriteString("digraph CallTree {\n  rankdir=TB;\n  node [shape=box];\n"); err != nil {
		return err
	}
	ids := make(map[functionID]int)
	addNode := func(function Function) error {
		key := function.id()
		if _, exists := ids[key]; exists {
			return nil
		}
		id := len(ids)
		ids[key] = id
		_, err := fmt.Fprintf(output, "  n%d [label=%q];\n", id, functionLabel(function))
		return err
	}
	if err := addNode(graph.Root); err != nil {
		return err
	}
	for _, call := range graph.Calls {
		if err := addNode(call.Caller); err != nil {
			return err
		}
		if err := addNode(call.Callee); err != nil {
			return err
		}
	}
	for _, call := range graph.Calls {
		label := call.Violation
		if label == "" && call.ThroughInterface {
			label = "interface"
		}
		from, to := ids[call.Caller.id()], ids[call.Callee.id()]
		var err error
		if label == "" {
			_, err = fmt.Fprintf(output, "  n%d -> n%d;\n", from, to)
		} else {
			_, err = fmt.Fprintf(output, "  n%d -> n%d [label=%q];\n", from, to, label)
		}
		if err != nil {
			return err
		}
	}
	if _, err := output.WriteString("}\n"); err != nil {
		return err
	}
	return output.Flush()
}

func functionLabel(function Function) string {
	label := fmt.Sprintf("%s\n%s\n%s:%d", function.Name, function.Package, function.File, function.Line)
	if function.Layer != "" {
		label += "\n" + function.Layer
	}
	if function.Context != nil {
		if len(function.Context.Parameters) > 0 {
			label += "\ncontext inputs: " + strings.Join(function.Context.Parameters, ", ")
		}
		for _, operation := range function.Context.Operations {
			label += "\n" + contextOperationLabel(operation)
		}
	}
	return label
}

func contextOperationLabel(operation ContextOperation) string {
	label := operation.Kind + ": " + operation.Context
	switch operation.Kind {
	case "create", "derive", "detach":
		if operation.Parent != "" {
			label += " (parent: " + operation.Parent + ")"
		}
	case "forward":
		label += " -> " + operation.Target
	case "read", "observe":
		label += " via " + operation.Target
	}
	if operation.Detail != "" {
		label += " [" + operation.Detail + "]"
	}
	if operation.Execution != "" {
		label += " [" + operation.Execution + "]"
	}
	if operation.Scope != "" {
		label += " [" + operation.Scope + "]"
	}
	return label
}
