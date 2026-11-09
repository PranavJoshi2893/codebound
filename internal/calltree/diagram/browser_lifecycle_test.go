package diagram

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/PranavJoshi2893/codebound/internal/calltree/model"
)

func lifecycleFixture() *model.Graph {
	// Misleading names are intentional: classification must use dispatch evidence.
	root := model.Function{Name: "ServeHTTP", Package: "example/cmd"}
	setup := model.Function{Name: "Handler.Create", Package: "example/item"}
	request := model.Function{Name: "NewHandler", Package: setup.Package}
	shared := model.Function{Name: "helper", Package: setup.Package}
	cycle := model.Function{Name: "cycle", Package: "example/helpers"}
	return &model.Graph{Root: root, Calls: []model.Call{
		{Caller: root, Callee: setup},
		{Caller: setup, Callee: request, Path: &model.CallPath{HTTPDispatch: true}},
		{Caller: root, Callee: shared},
		{Caller: request, Callee: shared},
		{Caller: request, Callee: cycle},
		{Caller: cycle, Callee: request},
	}}
}

func TestBrowserLifecycleUsesDispatchReachabilityNotNames(t *testing.T) {
	graph := lifecycleFixture()
	before, _ := json.Marshal(graph)
	index := IndexGraph(graph)
	want := []browserPhase{phaseEntry, phaseEntry, phaseHTTP, phaseEntry | phaseHTTP, phaseHTTP}
	if !reflect.DeepEqual(index.phases, want) {
		t.Fatalf("unexpected lifecycle reachability: %v", index.phases)
	}
	groups := index.browserGroups([]int{0, 1, 2, 3, 4})
	var itemGroups []browserGroup
	for _, group := range groups {
		if group.pkg == "example/item" {
			itemGroups = append(itemGroups, group)
		}
	}
	if len(itemGroups) != 3 {
		t.Fatalf("setup, request and shared functions must not share one package rectangle: %+v", groups)
	}
	for _, contexts := range []bool{false, true} {
		for _, focus := range []int{-1, 2} {
			var dot bytes.Buffer
			if err := index.WriteBrowserDOT(&dot, BrowserOptions{Focus: focus, Contexts: contexts}); err != nil {
				t.Fatal(err)
			}
			for _, caption := range []string{"ENTRY / LIFECYCLE", "HTTP REQUEST", "SHARED · ENTRY + HTTP"} {
				if !strings.Contains(dot.String(), caption) {
					t.Fatalf("missing lifecycle caption %q: %s", caption, dot.String())
				}
			}
		}
	}
	after, _ := json.Marshal(graph)
	if !bytes.Equal(before, after) {
		t.Fatal("layout classification changed analysis evidence")
	}
}

func TestBrowserLifecycleRetainsMixedPathsAndOrdinaryGraphs(t *testing.T) {
	graph := lifecycleFixture()
	// Same target can be invoked during setup and after dispatch.
	graph.Calls = append(graph.Calls, model.Call{Caller: graph.Root, Callee: graph.Calls[1].Callee})
	index := IndexGraph(graph)
	if index.phase(2) != phaseEntry|phaseHTTP || index.phase(4) != phaseEntry|phaseHTTP {
		t.Fatal("mixed reachability was discarded")
	}
	for i := range graph.Calls {
		graph.Calls[i].Path = nil
	}
	index = IndexGraph(graph)
	if index.phases != nil {
		t.Fatal("HTTP-like function names introduced an invented lifecycle")
	}
}
