package calltree

import (
	_ "embed"
	"fmt"
	"io"
	"strconv"
	"text/template"
)

//go:embed templates/graph.dot.tmpl
var dotTemplateText string

var dotTemplate = template.Must(template.New("dot").Funcs(template.FuncMap{
	"quote": strconv.Quote,
}).Parse(dotTemplateText))

type dotGraph struct {
	Nodes []dotNode
	Edges []dotEdge
}

type dotNode struct {
	ID    string
	Label string
}

type dotEdge struct {
	From  string
	To    string
	Label string
}

// WriteDOT writes plain Graphviz DOT without invoking external tools.
func WriteDOT(writer io.Writer, graph *Graph) error {
	view := dotGraph{}
	type functionID struct {
		name, pkg, file string
		line            int
	}
	ids := make(map[functionID]string)
	addNode := func(function Function) string {
		key := functionID{function.Name, function.Package, function.File, function.Line}
		if id := ids[key]; id != "" {
			return id
		}
		id := fmt.Sprintf("n%d", len(ids))
		ids[key] = id
		view.Nodes = append(view.Nodes, dotNode{ID: id, Label: functionLabel(function)})
		return id
	}
	addNode(graph.Root)
	for _, call := range graph.Calls {
		label := call.Violation
		if label == "" && call.ThroughInterface {
			label = "interface"
		}
		view.Edges = append(view.Edges, dotEdge{
			From: addNode(call.Caller), To: addNode(call.Callee), Label: label,
		})
	}
	return dotTemplate.Execute(writer, view)
}

func functionLabel(function Function) string {
	label := fmt.Sprintf("%s\n%s\n%s:%d", function.Name, function.Package, function.File, function.Line)
	if function.Layer != "" {
		label += "\n" + function.Layer
	}
	return label
}
