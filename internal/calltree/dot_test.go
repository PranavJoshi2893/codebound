package calltree

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestWriteDOT(t *testing.T) {
	root := Function{Name: "main", Package: "app", File: "main.go", Line: 1}
	callee := Function{Name: "Handle", Package: "app/handler", File: "handler.go", Line: 2, Layer: "handler"}
	graph := &Graph{Root: root, Calls: []Call{{Caller: root, Callee: callee, ThroughInterface: true}}}
	var output bytes.Buffer
	if err := WriteDOT(&output, graph); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"digraph CallTree", `label="main\napp\nmain.go:1"`, "n0 -> n1", `label="interface"`} {
		if !strings.Contains(output.String(), expected) {
			t.Errorf("missing %q in %s", expected, output.String())
		}
	}
	if strings.Contains(output.String(), "\x1b[") || strings.Contains(output.String(), "color=") {
		t.Fatal("DOT output contains terminal colors")
	}
}

func TestDOTDistinguishesSameLineDeclarations(t *testing.T) {
	root := Function{Name: "First", Package: "app", File: "app.go", Line: 1}
	callee := root
	callee.Name = "Second"
	graph := &Graph{Root: root, Calls: []Call{{Caller: root, Callee: callee}}}
	var output bytes.Buffer
	if err := WriteDOT(&output, graph); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "n0 -> n1") {
		t.Fatal("different functions on the same source line collapsed into one node")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestDOTReturnsWriteErrors(t *testing.T) {
	if err := WriteDOT(failingWriter{}, &Graph{}); err == nil {
		t.Fatal("writer error was ignored")
	}
}
