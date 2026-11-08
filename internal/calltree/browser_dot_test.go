package calltree

import (
	"bytes"
	"io"
	"strconv"
	"strings"
	"testing"
)

func TestBrowserDOT(t *testing.T) {
	root := Function{Name: "main", Package: "example/app", File: "/project/main.go", Line: 1,
		Context: &ContextInfo{Operations: []ContextOperation{{Kind: "create", Target: "context.Background", IDs: []string{"C1"}}}}}
	worker := Function{Name: "Worker.Run", Package: "example/service", Layer: "service", File: "/project/worker.go", Line: 5}
	leaf := Function{Name: "Get", Package: "example/repo", File: "/project/repo.go", Line: 10}
	helper := Function{Name: "Helper", Package: "example/helpers"}
	graph := &Graph{Root: root, Calls: []Call{
		{Caller: root, Callee: worker, Execution: "goroutine", ThroughInterface: true, Contexts: []ContextBinding{{Parameter: "ctx", IDs: []string{"C1"}, Unresolved: true}}},
		{Caller: root, Callee: worker, Execution: "goroutine", ThroughInterface: true, Contexts: []ContextBinding{{Parameter: "ctx", IDs: []string{"C1"}, Unresolved: true}}},
		{Caller: worker, Callee: leaf, Violation: "a very long architecture diagnosis"},
		{Caller: root, Callee: helper},
		{Caller: worker, Callee: worker, Execution: "deferred"},
	}}
	indexed := IndexGraph(graph)
	var output bytes.Buffer
	if err := indexed.WriteBrowserDOT(&output, BrowserOptions{Focus: -1}); err != nil {
		t.Fatal(err)
	}
	dot := output.String()
	for _, want := range []string{"START HERE", "Worker.Run", "Layer: service", `URL="function:1"`, "starts goroutine", "possible interface target", `style="dotted"`, "defers call until return (recursive)", "architecture warning", `color="#b91c1c"`, `graph [label=""]`} {
		if !strings.Contains(dot, want) {
			t.Errorf("missing visual explanation %q:\n%s", want, dot)
		}
	}
	if strings.Contains(dot, "c0 [") || strings.Contains(dot, "passes ctx=") || strings.Contains(dot, "/project/main.go\\n") {
		t.Fatal("simple view must not contain context markers/bindings or full paths in labels")
	}
	if strings.Count(dot, "n0 -> n1") != 1 {
		t.Fatal("equivalent repeated calls should be drawn once")
	}
	output.Reset()
	if err := indexed.WriteBrowserDOT(&output, BrowserOptions{Contexts: true, Focus: -1}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "c0 [shape=note") || !strings.Contains(output.String(), "passes ctx=possible C1 / unknown") {
		t.Fatalf("context view lost lifecycle markers or uncertainty:\n%s", output.String())
	}
	output.Reset()
	if err := indexed.WriteBrowserDOT(&output, BrowserOptions{Focus: 1}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "Helper") || !strings.Contains(output.String(), "other call relationships hidden") || !strings.Contains(output.String(), "FOCUS\\n") {
		t.Fatalf("focused graph must hide unrelated calls and disclose omissions:\n%s", output.String())
	}
	output.Reset()
	if err := indexed.WriteBrowserDOT(&output, BrowserOptions{Focus: 2}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "START HERE") || strings.Contains(output.String(), "n0 ->") {
		t.Fatal("a leaf-focused graph must not reintroduce the original root")
	}
	if err := indexed.WriteBrowserDOT(io.Discard, BrowserOptions{Focus: 99}); err == nil {
		t.Fatal("invalid focus must be rejected")
	}
	if err := indexed.WriteBrowserDOT(failingWriter{}, BrowserOptions{Focus: -1}); err == nil {
		t.Fatal("writer errors must be returned")
	}
	if graph.Root.Context == nil || len(graph.Calls[0].Contexts) != 1 || len(graph.Calls) != 5 {
		t.Fatal("presentation must not change analysis/export data")
	}
}

func TestBrowserDOTLongEscapedLabels(t *testing.T) {
	root := Function{Name: "main", Package: "example/app"}
	leaf := Function{Name: strings.Repeat(`Long\"Name`, 1000), Package: "example/worker"}
	index := IndexGraph(&Graph{Root: root, Calls: []Call{{Caller: root, Callee: leaf}}})
	var output bytes.Buffer
	if err := index.WriteBrowserDOT(&output, BrowserOptions{Focus: -1}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "label="+strconv.Quote(leaf.Name+"\nexample/worker")) || !strings.Contains(output.String(), "n0 -> n1") {
		t.Fatal("buffer growth must preserve long escaped labels and following edges")
	}
}
