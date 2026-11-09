package diagram

import (
	"bytes"
	"strings"
	"testing"

	"github.com/PranavJoshi2893/codebound/internal/calltree/model"
)

func TestIndirectPathsAreNotDirectOrInlineRecursion(t *testing.T) {
	fn := model.Function{Name: "Run", Package: "example", Flow: &model.FunctionFlow{Scopes: []model.FlowScope{{ID: "body", Entry: 0, Blocks: []model.FlowBlock{{ID: 0, Exit: "return", Steps: []model.FlowStep{{ID: "s1", Kind: "go", Label: "closure()"}}}}}}}}
	graph := &model.Graph{Root: fn, Calls: []model.Call{{Caller: fn, Callee: fn, Site: "s1", Path: &model.CallPath{HTTPDispatch: true}}}}
	edges := IndexGraph(graph).browserEdges(nil, false)
	if len(edges) != 1 || edges[0].localRecursion() {
		t.Fatal("hidden re-entry falsely marked as a direct recursive invocation")
	}
	var browser bytes.Buffer
	if err := edges[0].writeDOT(&browser, false); err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"possible HTTP dispatch", "indirect", `style="dashed"`, "recursive"} {
		if !strings.Contains(browser.String(), label) {
			t.Fatalf("missing %q: %s", label, browser.String())
		}
	}
	var dot bytes.Buffer
	if err := WriteDOT(&dot, graph); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dot.String(), "possible HTTP dispatch") {
		t.Fatalf("DOT discarded path semantics: %s", dot.String())
	}
}
