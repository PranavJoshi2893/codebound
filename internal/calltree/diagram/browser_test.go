package diagram

import (
	"bytes"
	"encoding/json"
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
	for _, want := range []string{`label="ROOT · main\nexample/app"`, `label="Worker.Run\nexample/service"`, "Layer: service", `URL="function:1"`, `label="go"`, "Possible interface target", `style="dotted"`, `label="defer\nrecursive"`, "Architecture warning (naming heuristic)", `color="#b91c1c"`, `graph [label=""]`} {
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
	for _, verbose := range []string{`label="calls"`, `label="starts goroutine`, `label="defers call`, `\nLayer: service", color=`} {
		if strings.Contains(dot, verbose) {
			t.Errorf("verbose text should not be drawn: %s", verbose)
		}
	}
	if !strings.Contains(dot, `n0 -> n3 [label=""`) {
		t.Fatal("ordinary calls need an arrow, not a repeated caption")
	}
	output.Reset()
	if err := indexed.WriteBrowserDOT(&output, BrowserOptions{Contexts: true, Focus: -1}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `new C1 · Background`) || !strings.Contains(output.String(), `label="go\nctx=possible C1 / unknown", style="dotted", color="#2563eb"`) {
		t.Fatalf("context view lost inline creation, passing, or uncertainty:\n%s", output.String())
	}
	if strings.Contains(output.String(), "shape=note") || strings.Contains(output.String(), "style=dashed") {
		t.Fatal("browser contexts must use the existing component-to-component route, not a second graph")
	}
	output.Reset()
	if err := indexed.WriteBrowserDOT(&output, BrowserOptions{Focus: 1}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "Helper") || !strings.Contains(output.String(), "other call relationships hidden") || !strings.Contains(output.String(), `color="#b45309"`) || !strings.Contains(output.String(), `\n+1 links"`) {
		t.Fatalf("focused graph must hide unrelated calls and disclose omissions:\n%s", output.String())
	}
	output.Reset()
	if err := indexed.WriteBrowserDOT(&output, BrowserOptions{Focus: 2}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "ROOT ·") || strings.Contains(output.String(), "n0 ->") {
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

func TestBrowserDOTPackageGrouping(t *testing.T) {
	root := Function{Name: "main", Package: "example/app"}
	run := Function{Name: "run", Package: root.Package}
	worker := Function{Name: "Run", Package: "example/worker"}
	helper := Function{Name: "helper", Package: worker.Package}
	leaf := Function{Name: "Get", Package: "example/repo"}
	index := IndexGraph(&Graph{Root: root, Calls: []Call{
		{Caller: root, Callee: run}, {Caller: run, Callee: worker},
		{Caller: worker, Callee: helper}, {Caller: helper, Callee: leaf},
	}})
	var output bytes.Buffer
	if err := index.WriteBrowserDOT(&output, BrowserOptions{Focus: -1}); err != nil {
		t.Fatal(err)
	}
	dot := output.String()
	if strings.Count(dot, "subgraph cluster_package_") != 2 {
		t.Fatalf("only multi-function packages should have a flat group:\n%s", dot)
	}
	for _, want := range []string{`label="ROOT · main\nexample/app"`, `n1 [label="run"`, `n2 [label="Run\nexample/worker"`, `n3 [label="helper"`, `n4 [label="Get\nexample/repo"`} {
		if !strings.Contains(dot, want) {
			t.Errorf("missing compact package presentation %s", want)
		}
	}
	if strings.Contains(dot, `label="run\nexample/app"`) || strings.Count(dot, " -> ") != 4 || strings.Count(dot, `URL="function:`) != 5 {
		t.Fatal("grouping must remove repeated subtitles without losing functions or calls")
	}
	output.Reset()
	if err := index.WriteBrowserDOT(&output, BrowserOptions{Focus: 4}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "subgraph cluster_") || strings.Contains(output.String(), "example/app") {
		t.Fatal("focused grouping must use only visible functions, not pull in the rest of a package")
	}
	output.Reset()
	if err := index.WriteBrowserDOT(&output, BrowserOptions{Focus: -1}); err != nil || output.String() != dot {
		t.Fatal("package ordering and function identities must stay deterministic across layouts")
	}
}

func TestBrowserDOTPreservesDistinctCallKinds(t *testing.T) {
	root := Function{Name: "main", Package: "example/app"}
	leaf := Function{Name: "Run", Package: "example/worker"}
	index := IndexGraph(&Graph{Root: root, Calls: []Call{
		{Caller: root, Callee: leaf},
		{Caller: root, Callee: leaf, ThroughInterface: true},
		{Caller: root, Callee: leaf, Execution: "goroutine"},
		{Caller: root, Callee: leaf, Execution: "deferred"},
	}})
	var output bytes.Buffer
	if err := index.WriteBrowserDOT(&output, BrowserOptions{Focus: -1}); err != nil {
		t.Fatal(err)
	}
	if strings.Count(output.String(), "n0 -> n1") != 1 || !strings.Contains(output.String(), `label="call / go / defer\ndirect / interface"`) {
		t.Fatal("one relationship arrow must disclose all execution and dispatch variants")
	}
}

func TestBrowserContextOmitsRepeatedEvidence(t *testing.T) {
	graph := contextLayoutFixture()
	var output bytes.Buffer
	if err := IndexGraph(graph).WriteBrowserDOT(&output, BrowserOptions{Contexts: true, Focus: -1}); err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(output.String(), "\n") {
		label, _, _ := strings.Cut(line, ", tooltip=")
		for _, verbose := range []string{"START:", "CAN CANCEL WHEN:", "CANCELLATION:", "main.go:", "passes C", "starts C"} {
			if strings.Contains(label, verbose) {
				t.Errorf("browser labels should not repeat lifecycle detail %q: %s", verbose, label)
			}
		}
	}
	if strings.Contains(output.String(), "[shape=note") || strings.Count(output.String(), " -> ") != 2 {
		t.Fatal("context mode should not add new nodes or parallel routes")
	}
	for _, want := range []string{"new C1 · Background", "C2 ← C1 · NotifyContext", "C3 ← C2 · WithTimeout", "new C4 · Background", "C5 ← C4 · WithTimeout", "+3 actions in details", "ctx=possible C3 / C5"} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("missing inline lifecycle or call flow %q:\n%s", want, output.String())
		}
	}
	exported := contextDOTOutput(t, graph)
	if !strings.Contains(exported, "CAN CANCEL WHEN:") || !strings.Contains(exported, "starts C2") {
		t.Fatal("detailed DOT export must remain unchanged")
	}
}

func TestBrowserContextBoundaryPackages(t *testing.T) {
	root := Function{Name: "main", Package: "example/app"}
	run := Function{Name: "run", Package: root.Package}
	worker := Function{Name: "Run", Package: "example/worker"}
	helper := Function{Name: "handle", Package: worker.Package}
	index := IndexGraph(&Graph{Root: root, Calls: []Call{
		{Caller: root, Callee: run}, {Caller: root, Callee: helper},
		{Caller: run, Callee: worker, Contexts: []ContextBinding{{Parameter: "ctx", IDs: []string{"C1"}}}},
		{Caller: worker, Callee: helper},
	}})
	for _, focus := range []int{-1, 3} {
		var output bytes.Buffer
		if err := index.WriteBrowserDOT(&output, BrowserOptions{Contexts: true, Focus: focus}); err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{`label="run\nexample/app`, `label="Run\nexample/worker`, `ctx=C1`} {
			if !strings.Contains(output.String(), want) {
				t.Errorf("package names must identify both ends of a boundary even inside a group: %s", want)
			}
		}
	}
}

func TestBrowserContextMergesVariantsWithoutChangingEvidence(t *testing.T) {
	root := Function{Name: "main", Package: "example/app"}
	worker := Function{Name: "Run", Package: "example/worker"}
	graph := &Graph{Root: root, Calls: []Call{
		{Caller: root, Callee: worker, Execution: "goroutine", ThroughInterface: true, Contexts: []ContextBinding{{Parameter: "ctx", IDs: []string{"C1"}}}},
		{Caller: root, Callee: worker, Contexts: []ContextBinding{{Parameter: "ctx", IDs: []string{"C2"}, Unresolved: true}}},
		{Caller: root, Callee: worker, Execution: "deferred", Violation: "a naming-based finding"},
	}}
	before, err := json.Marshal(graph)
	if err != nil {
		t.Fatal(err)
	}
	exportBefore := contextDOTOutput(t, graph)
	var output bytes.Buffer
	if err := IndexGraph(graph).WriteBrowserDOT(&output, BrowserOptions{Contexts: true, Focus: -1}); err != nil {
		t.Fatal(err)
	}
	dot := output.String()
	if strings.Count(dot, " -> ") != 1 {
		t.Fatal("context, execution, and dispatch variants must not add parallel routes")
	}
	for _, want := range []string{"go / call / defer", "direct / interface", "ctx=possible C1 / C2 / unknown (partial)", `style="solid"`, `color="#b91c1c"`, `fontcolor="#2563eb"`, `penwidth=2.0`} {
		if !strings.Contains(dot, want) {
			t.Errorf("merged arrow must retain its variant evidence %s:\n%s", want, dot)
		}
	}
	after, err := json.Marshal(graph)
	if err != nil || !bytes.Equal(before, after) || contextDOTOutput(t, graph) != exportBefore {
		t.Fatal("browser summaries must not mutate or aggregate exported analysis evidence")
	}
}

func TestBrowserContextBindingVariants(t *testing.T) {
	for _, test := range []struct {
		name  string
		calls []Call
		want  []string
	}{
		{"unknown variant", []Call{
			{Contexts: []ContextBinding{{Parameter: "ctx", IDs: []string{"C1"}}}},
			{Contexts: []ContextBinding{{Parameter: "ctx"}}},
		}, []string{"ctx=possible C1 / unknown"}},
		{"repeated evidence", []Call{
			{Contexts: []ContextBinding{{Parameter: "ctx", IDs: []string{"C1"}}}},
			{Contexts: []ContextBinding{{Parameter: "ctx", IDs: []string{"C1"}}}},
		}, []string{"ctx=C1"}},
		{"missing binding evidence", []Call{
			{Contexts: []ContextBinding{{Parameter: "ctx", IDs: []string{"C1"}}}},
			{},
		}, []string{"ctx=possible C1 / unknown (partial)"}},
		{"blank parameters", []Call{
			{Contexts: []ContextBinding{{Parameter: "_", IDs: []string{"C1"}}, {Parameter: "_", IDs: []string{"C2"}}}},
		}, []string{"_=C1", "_=C2"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			edge := browserEdge{}
			for i := range test.calls {
				edge.calls = append(edge.calls, &test.calls[i])
			}
			if got := edge.contextLabels(); strings.Join(got, "\n") != strings.Join(test.want, "\n") {
				t.Fatalf("context origins must stay conservative and parameter-specific: got %v want %v", got, test.want)
			}
		})
	}
}

func TestBrowserContextKeepsOriginsInTheirOwner(t *testing.T) {
	info := &ContextInfo{Bindings: []ContextBinding{{Parameter: "ctx", IDs: []string{"I1"}}}, Operations: []ContextOperation{
		{Kind: "create", Target: "context.Background", IDs: []string{"C1"}},
		{Kind: "derive", Target: "context.WithTimeout", IDs: []string{"C2"}, ParentIDs: []string{"C1", "I1"}, ParentUnresolved: true},
		{Kind: "detach", Target: "context.WithoutCancel", IDs: []string{"C3"}, ParentIDs: []string{"C2"}},
		{Kind: "derive", Target: "context.WithValue", IDs: []string{"C4"}},
	}}
	lines := strings.Join(browserContextSummary(info, true), "\n")
	for _, want := range []string{"new C1 · Background", "C2 ← possible C1 / I1 / unknown · WithTimeout", "C3 ← C2 · WithoutCancel", "C4 ← unresolved context · WithValue", "input ctx=I1"} {
		if !strings.Contains(lines, want) {
			t.Errorf("origins must not be omitted or moved outside their owner: %s", want)
		}
	}
	if strings.Contains(strings.Join(browserContextSummary(info, false), "\n"), "input ctx=") {
		t.Fatal("only the original root may introduce outside-tree input bindings")
	}
}

func TestBrowserPackageCaptionsDisambiguateModules(t *testing.T) {
	packages := []string{"example.com/api/internal/server", "example.com/jobs/internal/server", "example.com/api/config"}
	captions := browserPackageCaptions(packages)
	if captions[packages[0]] != packages[0] || captions[packages[1]] != packages[1] || captions[packages[2]] != "api/config" {
		t.Fatalf("short package captions must not make distinct modules indistinguishable: %v", captions)
	}
}
