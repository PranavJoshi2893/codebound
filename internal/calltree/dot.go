package calltree

import (
	"bufio"
	"fmt"
	"io"
)

type functionID struct {
	name, pkg, file string
	line            int
}

type dotCallID struct {
	from, to         int
	throughInterface bool
	violation        string
}

func (f Function) id() functionID {
	return functionID{f.Name, f.Package, f.File, f.Line}
}

// WriteDOT streams nodes and call edges, using identity indexes to combine
// repeated relationships. The analysis graph and its JSON findings stay intact.
func WriteDOT(writer io.Writer, graph *Graph) error {
	output := bufio.NewWriter(writer)
	if _, err := output.WriteString("digraph CallTree {\n  rankdir=TB;\n  splines=polyline;\n  node [shape=box];\n"); err != nil {
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
	writtenCalls := make(map[dotCallID]bool)
	for _, call := range graph.Calls {
		label := call.Violation
		if label == "" && call.ThroughInterface {
			label = "interface"
		}
		from, to := ids[call.Caller.id()], ids[call.Callee.id()]
		key := dotCallID{from: from, to: to, throughInterface: call.ThroughInterface, violation: call.Violation}
		if writtenCalls[key] {
			continue
		}
		writtenCalls[key] = true
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
	if err := writeContextDOT(output, graph, ids); err != nil {
		return err
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
	return label
}
