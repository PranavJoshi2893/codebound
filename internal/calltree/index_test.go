package calltree

import (
	"reflect"
	"testing"
)

func TestGraphIndex(t *testing.T) {
	root := Function{Name: "main", Package: "app", File: "app.go", Line: 1}
	other := root
	other.Name = "Other" // Different declaration, even on the same line.
	leaf := Function{Name: "Leaf", Package: "app"}
	graph := &Graph{Root: root, Calls: []Call{
		{Caller: root, Callee: other},
		{Caller: root, Callee: other}, // Repeated call sites do not inflate relationship counts.
		{Caller: other, Callee: other},
		{Caller: other, Callee: leaf},
		{Caller: leaf, Callee: other}, // The reverse direction is a separate relationship.
	}}
	index := IndexGraph(graph)
	if len(index.Functions) != 3 || index.Functions[0].Name != "main" || index.Functions[1].Name != "Other" {
		t.Fatalf("lost root-first declaration identity: %+v", index.Functions)
	}
	if index.Calls[2].From != 1 || index.Calls[2].To != 1 || index.Calls[2].Call != &graph.Calls[2] {
		t.Fatal("index must preserve recursive calls and reference original evidence")
	}
	if !reflect.DeepEqual(index.relations, []int{1, 4, 2}) {
		t.Fatalf("incorrect distinct relationship counts: %v", index.relations)
	}
	for id, want := range [][]int{{0, 1}, {0, 1, 2, 3, 4}, {3, 4}} {
		got := index.incident[index.offsets[id]:index.offsets[id+1]]
		if !reflect.DeepEqual(got, want) {
			t.Errorf("function %d: incident calls %v, want %v", id, got, want)
		}
	}
	if len(IndexGraph(nil).Functions) != 0 || len(IndexGraph(&Graph{Root: root}).Functions) != 1 {
		t.Fatal("empty and root-only graphs must remain valid")
	}
}
