package diagram

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/PranavJoshi2893/codebound/internal/calltree/model"
)

func compactLayoutFixture() *model.Graph {
	function := func(name, pkg string, blocks int) model.Function {
		flow := &model.FunctionFlow{Scopes: []model.FlowScope{{ID: "body", Entry: 0}}}
		for i := 0; i < blocks; i++ {
			block := model.FlowBlock{ID: i, Exit: "return", Steps: []model.FlowStep{{ID: fmt.Sprintf("s%d", i), Reference: fmt.Sprintf("B%d.1", i), Kind: "call", Label: "operation()"}}}
			if i+1 < blocks {
				block.Exit = "jump"
				block.Next = []model.FlowBranch{{To: i + 1}}
			}
			flow.Scopes[0].Blocks = append(flow.Scopes[0].Blocks, block)
		}
		return model.Function{Name: name, Package: pkg, Flow: flow}
	}
	root := function("root", "example/app", 5)
	heavy := function("terminal", "example/externalop", 18)
	branch := function("branch", "example/server", 12)
	leaf := function("leaf", "example/config", 2)
	other := function("other", "example/setup", 1)
	sink := function("sink", "example/request", 2)
	return &model.Graph{Root: root, Calls: []model.Call{{Caller: root, Callee: heavy}, {Caller: root, Callee: branch}, {Caller: root, Callee: leaf}, {Caller: root, Callee: other}, {Caller: branch, Callee: sink}}}
}

func TestBrowserCompactsTallTerminalFanout(t *testing.T) {
	index := IndexGraph(compactLayoutFixture())
	edges := index.browserEdges(nil, false)
	index.compactBrowserFanout(edges, []int{0, 1, 2, 3, 4, 5})
	if edges[0].minlen != 2 {
		t.Fatal("tall singleton leaf was not moved out of the crowded first rank")
	}
	for _, edge := range edges[1:] {
		if edge.minlen != 0 {
			t.Fatal("call backbone or short leaves must not be staggered")
		}
	}
	var dot bytes.Buffer
	if err := index.WriteBrowserDOT(&dot, BrowserOptions{Focus: -1}); err != nil {
		t.Fatal(err)
	}
	compact := renderDOTLayout(t, dot.String())
	baseline := renderDOTLayout(t, strings.ReplaceAll(dot.String(), ", minlen=2", ""))
	checkLayoutTextSeparate(t, compact)
	checkLayoutRoutesAvoidText(t, compact)
	if len(compact.Edges) != len(baseline.Edges) {
		t.Fatal("compaction hid relationships")
	}
	_, height := layoutExtent(t, compact)
	_, oldHeight := layoutExtent(t, baseline)
	if height >= oldHeight {
		t.Fatalf("packing did not reduce the tall fanout: %v >= %v", height, oldHeight)
	}
}

func TestBrowserDoesNotStaggerSharedGroupedOrCyclicTargets(t *testing.T) {
	for _, kind := range []string{"shared", "grouped", "feedback", "recursive", "dispatch"} {
		t.Run(kind, func(t *testing.T) {
			graph := compactLayoutFixture()
			heavy := graph.Calls[0].Callee
			switch kind {
			case "shared":
				graph.Calls = append(graph.Calls, model.Call{Caller: graph.Calls[1].Callee, Callee: heavy})
			case "grouped":
				helper := model.Function{Name: "peer", Package: heavy.Package}
				graph.Calls = append(graph.Calls, model.Call{Caller: graph.Root, Callee: helper})
			case "feedback":
				graph.Calls = append(graph.Calls, model.Call{Caller: heavy, Callee: graph.Root})
			case "recursive":
				graph.Calls = append(graph.Calls, model.Call{Caller: heavy, Callee: heavy})
			case "dispatch":
				graph.Calls[0].Path = &model.CallPath{HTTPDispatch: true}
			}
			index := IndexGraph(graph)
			edges := index.browserEdges(nil, false)
			visible := make([]int, len(index.Functions))
			for i := range visible {
				visible[i] = i
			}
			index.compactBrowserFanout(edges, visible)
			if edges[0].minlen != 0 {
				t.Fatalf("%s target must not move", kind)
			}
		})
	}
}

func layoutExtent(t *testing.T, layout contextLayout) (float64, float64) {
	t.Helper()
	var x0, y0, x1, y1 float64
	if _, err := fmt.Sscanf(layout.Bounds, "%f,%f,%f,%f", &x0, &y0, &x1, &y1); err != nil {
		t.Fatal(err)
	}
	return x1 - x0, y1 - y0
}

func TestBrowserCompactionRespectsHiddenNeighbors(t *testing.T) {
	graph := compactLayoutFixture()
	heavy := graph.Calls[0].Callee
	graph.Calls = append(graph.Calls, model.Call{Caller: heavy, Callee: graph.Calls[4].Callee})
	index := IndexGraph(graph)
	calls := index.incident[index.offsets[0]:index.offsets[1]]
	edges := index.browserEdges(calls, true)
	index.compactBrowserFanout(edges, []int{0, 1, 2, 3, 4})
	if edges[0].minlen != 0 {
		t.Fatal("a focused view mistook an omitted outgoing relationship for a terminal")
	}
}
