package diagram

import (
	"bytes"
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/PranavJoshi2893/codebound/internal/calltree/model"
)

func browserExecutionFixture() *Graph {
	root := Function{Name: "main", Package: "example/app", Flow: &model.FunctionFlow{Scopes: []model.FlowScope{
		{ID: "body", Entry: 0, Blocks: []model.FlowBlock{
			{ID: 0, Label: "entry", Steps: []model.FlowStep{
				{ID: "s1", Reference: "B0.1", Kind: "call", Label: "prepare()"},
				{ID: "s2", Reference: "B0.2", Kind: "call", Label: "context.WithCancel(…)"},
				{ID: "s3", Reference: "B0.3", Kind: "defer", Label: "cancel()"},
				{ID: "s4", Reference: "B0.4", Kind: "call", Label: "callback()", Closure: "L1"},
			}, Exit: "branch", Condition: "ready && value < limit", Next: []model.FlowBranch{{To: 1, Label: "true"}, {To: 2, Label: "false"}}},
			{ID: 1, Label: "if then", Steps: []model.FlowStep{{ID: "s5", Reference: "B1.1", Kind: "go", Label: "Worker.Run(…)"}}, Exit: "jump", Next: []model.FlowBranch{{To: 3}}},
			{ID: 2, Label: "if else", Steps: []model.FlowStep{{ID: "s6", Reference: "B2.1", Kind: "call", Label: "Worker.Run(…)"}}, Exit: "jump", Next: []model.FlowBranch{{To: 3}}},
			{ID: 3, Label: "if done", Steps: []model.FlowStep{{ID: "s8", Reference: "B3.1", Kind: "run_defers", Label: "run deferred stack · last registered first"}}, Exit: "return"},
		}},
		{ID: "L1", Entry: 0, Blocks: []model.FlowBlock{{ID: 0, Label: "entry", Steps: []model.FlowStep{
			{ID: "s7", Reference: "L1/B0.1", Kind: "receive", Label: "receive from channel"},
		}, Exit: "return"}}},
	}}, Context: &ContextInfo{Bindings: []ContextBinding{{Parameter: "ctx", IDs: []string{"I1"}}}, Operations: []ContextOperation{
		{Site: "s2", Kind: "derive", Target: "context.WithCancel", IDs: []string{"C1"}, ParentIDs: []string{"I1"}},
		{Site: "s3", Kind: "cancel", IDs: []string{"C1"}, Execution: "deferred"},
		{Site: "s7", Kind: "observe", IDs: []string{"C1"}, Scope: "closure"},
	}}}
	worker := Function{Name: "Worker.Run", Package: "example/worker"}
	cleanup := Function{Name: "cleanup", Package: "example/cleanup"}
	return &Graph{Root: root, Calls: []Call{
		{Caller: root, Callee: worker, Site: "s5", Execution: "goroutine", Contexts: []ContextBinding{{Parameter: "ctx", IDs: []string{"C1"}}}},
		{Caller: root, Callee: worker, Site: "s6", Contexts: []ContextBinding{{Parameter: "ctx", IDs: []string{"I1"}}}},
		{Caller: root, Callee: cleanup, Site: "s3", Execution: "deferred"},
	}}
}

func TestBrowserExecutionBlocksAndCallSites(t *testing.T) {
	graph := browserExecutionFixture()
	var output bytes.Buffer
	if err := IndexGraph(graph).WriteBrowserDOT(&output, BrowserOptions{Contexts: true, Focus: -1}); err != nil {
		t.Fatal(err)
	}
	dot := output.String()
	for _, want := range []string{`label=<<TABLE`, "B0 · entry", "B1 · if then", "B2 · if else", "B3 · if done", "true → B1 | false → B2", "if ready &amp;&amp; value &lt; limit", "register defer · cancel()", "on exit: cancel/stop C1", "last registered first", "go · concurrent", "CLOSURE L1 · only when invoked", "observes Done: C1", `tailport="s5:e"`, `tailport="s6:e"`, `tailport="s3:e"`, `arrowtail="dot"`, `PORT="s2"`} {
		if !strings.Contains(dot, want) {
			t.Errorf("missing execution semantics %q:\n%s", want, dot)
		}
	}
	if strings.Index(dot, "prepare()") > strings.Index(dot, "C1 ← I1") {
		t.Fatal("context derivation must stay at its actual step, not move above earlier calls")
	}
	if strings.Count(dot, "n0 -> n1") != 2 || strings.Contains(dot, "at B1.1, B2.1") {
		t.Fatal("distinct call rows need their own visibly attached route, not references the reader must decode")
	}
	if !strings.Contains(dot, `go\nctx=C1`) || !strings.Contains(dot, `ctx=I1`) {
		t.Fatal("each call row must retain its own scheduling and context, not borrow another row's variants")
	}
	if strings.Contains(dot, "ready && value < limit") {
		t.Fatal("source expressions must be escaped in Graphviz HTML labels")
	}
	layout := renderDOTLayout(t, dot)
	checkLayoutTextSeparate(t, layout)
	checkLayoutRoutesAvoidText(t, layout)
}

func TestBrowserPanelsUseHorizontalSpaceWithoutReorderingExecution(t *testing.T) {
	graph := browserExecutionFixture()
	graph.Calls = nil // Panels without outgoing connectors should remain compact.
	index := IndexGraph(graph)
	render := func() contextLayout {
		t.Helper()
		var dot bytes.Buffer
		if err := index.WriteBrowserDOT(&dot, BrowserOptions{Contexts: true, Focus: -1}); err != nil {
			t.Fatal(err)
		}
		return renderDOTLayout(t, dot.String())
	}
	layout := render()
	checkLayoutTextSeparate(t, layout)
	checkLayoutRoutesAvoidText(t, layout)
	find := func(text string) layoutTextBounds {
		t.Helper()
		for _, drawing := range layout.Objects {
			for _, bounds := range drawing.textBounds() {
				if bounds.text == text {
					return bounds
				}
			}
		}
		t.Fatalf("rendered panel is missing %q", text)
		return layoutTextBounds{}
	}
	left, right, join := find("B1 · if then"), find("B2 · if else"), find("B3 · if done")
	if math.Abs(left.top-right.top) > 0.5 || right.left <= left.right || join.top >= min(left.bottom, right.bottom) {
		t.Fatal("actual Graphviz geometry must put alternatives beside each other and their join below")
	}
	entry := find("ENTRY → B0 · follow exits")
	closure := find("CLOSURE L1 · only when invoked · entry B0")
	if math.Abs(entry.top-closure.top) > 0.5 || closure.left <= entry.right {
		t.Fatalf("independent closure scopes should use the space beside the body, not add another vertical section: body=%+v closure=%+v", entry, closure)
	}
	dimensions := func(layout contextLayout) (float64, float64) {
		t.Helper()
		for _, drawing := range layout.Objects {
			if drawing.Name == "n0" {
				width, widthErr := strconv.ParseFloat(drawing.Width, 64)
				height, heightErr := strconv.ParseFloat(drawing.Height, 64)
				if widthErr != nil || heightErr != nil {
					t.Fatal("missing rendered function dimensions")
				}
				return width, height
			}
		}
		t.Fatal("missing rendered root")
		return 0, 0
	}
	// Compare with the same content in the previous one-column presentation.
	linear := flowLayout{columns: 1}
	for i, scope := range graph.Root.Flow.Scopes {
		section := flowScopeLayout{scope: scope, columns: 1}
		for block, data := range scope.Blocks {
			section.rows = append(section.rows, []flowPanel{{block: block, last: len(data.Steps)}})
		}
		linear.scopes = append(linear.scopes, section)
		linear.groups = append(linear.groups, []int{i})
	}
	index.layouts[0] = linear
	oldWidth, oldHeight := dimensions(render())
	width, height := dimensions(layout)
	if width <= oldWidth || height >= oldHeight {
		t.Fatalf("horizontal panels must actually reduce vertical space: new %gx%g, single-column %gx%g", width, height, oldWidth, oldHeight)
	}
}

func TestBrowserExecutionDoesNotMoveUnpositionedFindings(t *testing.T) {
	graph := browserExecutionFixture()
	graph.Root.Context.Operations = append(graph.Root.Context.Operations, ContextOperation{Kind: "create", IDs: []string{"C99"}})
	label := renderFlowLabel("main", graph.Root, true, true, nil, planFlowLayout(graph.Root.Flow), 0, nil)
	if !strings.Contains(label, "1 context findings not positioned") || strings.Contains(label, "new C99") {
		t.Fatal("findings without execution evidence must not be inserted at an invented position")
	}
	label = renderFlowLabel("<script>& source", graph.Root, false, false, nil, planFlowLayout(graph.Root.Flow), 0, nil)
	if strings.Contains(label, "<script>") || !strings.Contains(label, "&lt;script&gt;&amp; source") || strings.Contains(label, "C1 ← I1") {
		t.Fatal("flow labels must escape source names and hide context evidence in call-only mode")
	}
	if got := flowExitLabel(model.FlowBlock{Exit: "jump", Next: []model.FlowBranch{{To: 1, BackEdge: true}}}); got != "loop → B1" {
		t.Fatalf("loop-back control flow must be explicit: %s", got)
	}
	recoverID := 3
	graph.Root.Flow.Scopes[0].Recover = &recoverID
	label = renderFlowLabel("main", graph.Root, false, true, nil, planFlowLayout(graph.Root.Flow), 0, nil)
	if !strings.Contains(label, "if panic recovered → B3") {
		t.Fatal("a possible recovered-panic entry must be labeled, not put on the ordinary sequential path")
	}
}

func TestBrowserRoutingSeparatesForwardAndFeedbackPorts(t *testing.T) {
	index := IndexGraph(browserRoutingFixture())
	edges := index.browserEdges(nil, false)
	var backward, forward, local int
	for _, edge := range edges {
		var output bytes.Buffer
		if err := edge.writeDOT(&output, true); err != nil {
			t.Fatal(err)
		}
		dot := output.String()
		if edge.localRecursion() {
			local++
			if dot != "" {
				t.Fatal("positioned self-recursion should not add an exterior loop")
			}
			continue
		}
		if edge.backward {
			backward++
			if !strings.Contains(dot, `_w:w"`) || !strings.Contains(dot, "constraint=false") {
				t.Fatalf("feedback must leave the opposite side without forcing a cyclic rank: %s", dot)
			}
			if !strings.Contains(dot, `headport="entry:e"`) {
				t.Errorf("feedback needs an exterior header connection: %s", dot)
			}
		} else {
			forward++
			if !strings.Contains(dot, `:e"`) || !strings.Contains(dot, `headport="entry:w"`) || strings.Contains(dot, "constraint=false") {
				t.Errorf("forward calls must travel left-to-right between facing ports, not into the top of a tall card: %s", dot)
			}
		}
	}
	if backward != 2 || forward != 11 || local != 1 {
		t.Fatalf("routing must preserve all forward, cyclic and recursive calls: forward=%d backward=%d local=%d", forward, backward, local)
	}
	label := renderFlowLabel("main", index.Functions[0], true, true, nil, index.layouts[0], 0, nil)
	for _, port := range []string{`PORT="entry"`, `PORT="q0b0s1"`, `PORT="q0b0s1_w"`} {
		if !strings.Contains(label, port) {
			t.Errorf("card must offer exterior ports without changing execution row order: %s", port)
		}
	}
}

func TestBrowserRecursionKeepsPerSiteContextAndScheduling(t *testing.T) {
	function := Function{Name: "Worker.Run", Package: "example/worker", Flow: &model.FunctionFlow{Scopes: []model.FlowScope{{
		ID: "body", Entry: 5, Blocks: []model.FlowBlock{{ID: 5, Label: "entry", Exit: "return", Steps: []model.FlowStep{
			{ID: "s1", Reference: "B5.1", Kind: "go", Label: "worker.Run(…)"},
			{ID: "s2", Reference: "B5.2", Kind: "defer", Label: "worker.Run(…)"},
		}}},
	}}}}
	graph := &Graph{Root: function, Calls: []Call{
		{Caller: function, Callee: function, Site: "s1", ThroughInterface: true, Execution: "goroutine", Contexts: []ContextBinding{{Parameter: "ctx", IDs: []string{"C1"}}}},
		{Caller: function, Callee: function, Site: "s2", Execution: "deferred", Contexts: []ContextBinding{{Parameter: "ctx", IDs: []string{"C2"}}}},
	}}
	before, _ := json.Marshal(graph)
	index := IndexGraph(graph)
	edges := index.browserEdges(nil, false)
	notes := localRecursionNotes(edges, index.Functions, true)[0]
	if strings.Join(notes["s1"], "\n") != "↻ recursive call → entry B5 · possible interface target\nctx=C1" ||
		strings.Join(notes["s2"], "\n") != "↻ recursive call → entry B5\nctx=C2" {
		t.Fatalf("recursive sites must not merge context origins or invent an entry: %v", notes)
	}
	var output bytes.Buffer
	if err := index.WriteBrowserDOT(&output, BrowserOptions{Contexts: true, Focus: -1}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"go · concurrent", "register defer", "ctx=C1", "ctx=C2", "↻ recursive call"} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("local recursion must retain scheduling and bindings: %s", want)
		}
	}
	if strings.Contains(output.String(), "n0 -> n0") {
		t.Fatal("positioned recursion should not add a second exterior route")
	}
	after, _ := json.Marshal(graph)
	if !bytes.Equal(before, after) {
		t.Fatal("local recursion presentation must preserve exported analysis evidence")
	}
	plain := localRecursionNotes(edges, index.Functions, false)[0]
	if len(plain["s1"]) != 1 || len(plain["s2"]) != 1 {
		t.Fatal("call-only recursion markers should not reveal context bindings")
	}
	graph.Calls = append(graph.Calls, Call{Caller: function, Callee: function})
	if IndexGraph(graph).browserEdges(nil, false)[0].localRecursion() {
		t.Fatal("unpositioned recursive evidence must keep an explicit route rather than be assigned an invented row")
	}
	output.Reset()
	if err := IndexGraph(graph).WriteBrowserDOT(&output, BrowserOptions{Contexts: true, Focus: -1}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "n0 -> n0") || !strings.Contains(output.String(), "unpositioned sites") {
		t.Fatal("recursion without a complete position map must keep its disclosed fallback route")
	}
}
