package calltree

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
)

func TestHybridContextViewKeepsUsageInsideFunctions(t *testing.T) {
	graph := contextLayoutFixture()
	dot := contextDOTOutput(t, graph)
	for _, expected := range []string{
		`ctx=C2`, `ctx=C3`, `ctx=C5`,
		`starts C2: NotifyContext`, `cancel/stop C2 [deferred]`,
		`ctx: C2`, `observes Done: C2`, `reads Err: C2`,
		`C2 · NotifyContext`, `START: main`, `CAN CANCEL WHEN:`, `cancel in main [deferred]`,
		`label="starts C2"`, `label="child of C2"`,
	} {
		if !strings.Contains(dot, expected) {
			t.Errorf("missing %q in %s", expected, dot)
		}
	}
	if strings.Contains(dot, "context input\"") || strings.Contains(dot, "shape=diamond") || strings.Contains(dot, "unmatched context target") {
		t.Fatal("parameters, reads, and forwarding must not create disconnected context nodes")
	}
	if strings.Count(dot, "shape=note") != 5 {
		t.Fatal("only the five creation/derivation sites should have separate markers")
	}
	if strings.Count(dot, "n1 -> n2 [") != 2 {
		t.Fatal("calls passing different contexts must stay separate, not merge into a misleading arrow")
	}
}

func TestHybridViewDoesNotInventMissingFlow(t *testing.T) {
	root := Function{Name: "Run", Package: "app", Context: &ContextInfo{Parameters: []string{"ctx"}}}
	callee := Function{Name: "Work", Package: "app", Context: &ContextInfo{Parameters: []string{"ctx"}}}
	dot := contextDOTOutput(t, &Graph{Root: root, Calls: []Call{{Caller: root, Callee: callee}}})
	if !strings.Contains(dot, "ctx: unresolved origin") || strings.Contains(dot, "shape=note") || strings.Contains(dot, "ctx=C") {
		t.Fatal("missing bindings must stay unresolved, not become new contexts or guessed call labels")
	}
}

func TestHybridViewPreservesWarningsAndUncertainty(t *testing.T) {
	call := Call{
		Violation: "handler must not call repository", ThroughInterface: true,
		Contexts: []ContextBinding{{Parameter: "ctx", IDs: []string{"C1", "C3"}, Unresolved: true}},
	}
	label := contextCallLabel(call)
	for _, expected := range []string{"handler must not call repository", "interface", "ctx=possible C1 / C3 / unknown"} {
		if !strings.Contains(label, expected) {
			t.Fatalf("lost %q in call label %q", expected, label)
		}
	}
}

func TestHybridViewQuotesLabels(t *testing.T) {
	operation := ContextOperation{Kind: "derive", Target: "context.WithTimeout", IDs: []string{"C1"}, Location: "quoted\"file.go:2:1", Detail: "timeout: 5s"}
	root := Function{Name: "Run", Package: "app", Context: &ContextInfo{Operations: []ContextOperation{operation}}}
	marker := contextMarker{owner: root, operation: operation}
	dot := contextDOTOutput(t, &Graph{Root: root})
	for _, expected := range []string{
		"label=" + strconv.Quote(functionLabel(root)),
		"label=" + strconv.Quote(marker.label()),
		"tooltip=" + strconv.Quote(contextOperationDetails(operation)),
	} {
		if !strings.Contains(dot, expected) {
			t.Fatalf("unsafe or missing label %q", expected)
		}
	}
}

func TestHybridViewPropagatesWriterErrors(t *testing.T) {
	root := Function{Name: "Run", Context: &ContextInfo{Parameters: []string{"ctx"}}}
	if err := writeContextDOT(failingWriter{}, &Graph{Root: root}, map[functionID]int{root.id(): 0}); err == nil {
		t.Fatal("context rendering write errors must be returned")
	}
}

func TestLifecycleDoesNotClaimInertParentCanCancel(t *testing.T) {
	markers := []contextMarker{
		{operation: ContextOperation{Target: "context.Background", IDs: []string{"C1"}}},
		{operation: ContextOperation{Target: "context.WithValue", IDs: []string{"C2"}, ParentIDs: []string{"C1"}}},
		{operation: ContextOperation{Target: "context.WithoutCancel", IDs: []string{"C3"}, ParentIDs: []string{"I1"}}},
	}
	byID := map[string]int{"C1": 0, "C2": 1, "C3": 2}
	for _, parent := range []string{"C1", "C2", "C3"} {
		if contextParentMayCancel(ContextOperation{ParentIDs: []string{parent}}, markers, byID) {
			t.Errorf("inert parent %s must not be advertised as a cancellation condition", parent)
		}
	}
	if !contextParentMayCancel(ContextOperation{ParentIDs: []string{"I1"}}, markers, byID) {
		t.Fatal("outside-tree parent cancellation cannot be ruled out")
	}
}

func contextDOTOutput(t *testing.T, graph *Graph) string {
	t.Helper()
	var output bytes.Buffer
	if err := WriteDOT(&output, graph); err != nil {
		t.Fatal(err)
	}
	return output.String()
}
