package calltree

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
)

func TestContextDOTDrawsSeparateNodesAndArrows(t *testing.T) {
	root := Function{Name: "main", Package: "app", File: "main.go", Line: 1, Context: &ContextInfo{
		Operations: []ContextOperation{
			{Kind: "create", Context: "background", Target: "context.Background", Location: "main.go:2:1"},
			{Kind: "derive", Context: "signal", Parent: "background", Target: "signal.NotifyContext", Location: "main.go:3:1"},
			{Kind: "cancel", Context: "signal", Target: "cancel", Execution: "deferred", Location: "main.go:4:1"},
			{Kind: "forward", Context: "signal", Target: "app.run", Location: "main.go:5:1"},
		},
	}}
	run := Function{Name: "run", Package: "app", File: "main.go", Line: 10, Context: &ContextInfo{
		Parameters: []string{"ctx"},
		Operations: []ContextOperation{
			{Kind: "derive", Context: "timeout", Parent: "parameter ctx", Target: "context.WithTimeout", Detail: "timeout: 5s", Location: "main.go:11:1"},
			{Kind: "detach", Context: "detached", Parent: "parameter ctx", Target: "context.WithoutCancel", Location: "main.go:12:1"},
			{Kind: "observe", Context: "parameter ctx", Target: "context.Context.Done", Location: "main.go:13:1"},
			{Kind: "read", Context: "parameter ctx", Target: "context.Context.Err", Location: "main.go:14:1"},
		},
	}}
	graph := &Graph{Root: root, Calls: []Call{{Caller: root, Callee: run}, {Caller: root, Callee: run}}}
	dot := contextDOTOutput(t, graph)
	if label := functionLabel(root); label != "main\napp\nmain.go:1" {
		t.Fatalf("context details must not appear inside the function box: %q", label)
	}
	for _, expected := range []string{
		`n0 -> n1;`,
		`c0 [shape=ellipse`,
		`n0 -> c0 [style=dashed`,
		`c0 -> c1 [style=dashed`,
		`n0 -> c1 [style=dashed, weight=0, color="#b91c1c", label="cancels [deferred]"`,
		`c1 -> n1 [style=dashed`,
		`label="passes context\nvia main"`,
		`c2 -> n1 [style=dashed, weight=0, color="#2563eb", label="receives"`,
		`c2 -> c3 [style=dashed`,
		`label="context.WithTimeout\ntimeout: 5s\nmain.go:11:1"`,
		`c2 -> c4 [style=dashed`,
		`label="values only\nno cancellation/deadline"`,
		`label="observes Done"`,
		`label="reads Err"`,
		`Solid arrows: function calls | dashed arrows: context flow`,
	} {
		if !strings.Contains(dot, expected) {
			t.Errorf("missing %q in %s", expected, dot)
		}
	}
	if strings.Count(dot, "  c1 [") != 1 || strings.Count(dot, `label="cancels [deferred]"`) != 1 {
		t.Fatal("repeated callers must not duplicate context nodes or actions")
	}
	if strings.Contains(dot, "rank=same") || strings.Contains(dot, "constraint=false") {
		t.Fatal("context nodes and labels must be allowed to reserve layout space")
	}
}

func TestContextDOTDoesNotMergeUnrelatedSources(t *testing.T) {
	root := Function{Name: "Run", Package: "app", Context: &ContextInfo{
		Parameters: []string{"ctx"},
		Operations: []ContextOperation{
			{Kind: "read", Context: "unresolved", Target: "context.Context.Err", Location: "app.go:2:1"},
			{Kind: "read", Context: "unresolved", Target: "context.Context.Err", Location: "app.go:3:1"},
		},
	}}
	callee := Function{Name: "Work", Package: "app", Context: &ContextInfo{Parameters: []string{"ctx"}}}
	dot := contextDOTOutput(t, &Graph{Root: root, Calls: []Call{{Caller: root, Callee: callee}}})
	if strings.Count(dot, `label="ctx\ncontext input"`) != 2 {
		t.Fatal("identically named parameters in different functions must be separate contexts")
	}
	if strings.Count(dot, `shape=diamond`) != 2 {
		t.Fatal("unresolved contexts from different call sites must not share an identity")
	}
	root.Context.Operations[0].Location = ""
	root.Context.Operations[1].Location = ""
	dot = contextDOTOutput(t, &Graph{Root: root})
	if strings.Count(dot, `shape=diamond`) != 2 {
		t.Fatal("missing source locations must not merge unrelated unresolved contexts")
	}
}

func TestContextDOTMatchesMethodTargets(t *testing.T) {
	for _, target := range []string{"app.Worker.Run", "(*app.Worker).Run", "(app.Worker).Run"} {
		t.Run(target, func(t *testing.T) {
			root := Function{Name: "Start", Package: "app", Context: &ContextInfo{
				Parameters: []string{"ctx"},
				Operations: []ContextOperation{{Kind: "forward", Context: "parameter ctx", Target: target}},
			}}
			callee := Function{Name: "Worker.Run", Package: "app"}
			dot := contextDOTOutput(t, &Graph{Root: root, Calls: []Call{{Caller: root, Callee: callee}}})
			if !strings.Contains(dot, "c0 -> n1 [style=dashed") || strings.Contains(dot, "  t0 [") {
				t.Fatalf("context arrow did not reach the method declaration: %s", dot)
			}
		})
	}
}

func TestContextDOTDoesNotGuessIndirectTargets(t *testing.T) {
	root := Function{Name: "Start", Package: "app", Context: &ContextInfo{
		Parameters: []string{"ctx"},
		Operations: []ContextOperation{{Kind: "forward", Context: "parameter ctx", Target: "(app.Service).Run"}},
	}}
	callee := Function{Name: "Worker.Run", Package: "app"}
	dot := contextDOTOutput(t, &Graph{Root: root, Calls: []Call{{Caller: root, Callee: callee, ThroughInterface: true}}})
	for _, expected := range []string{`n0 -> n1 [label="interface"]`, `c0 -> t0 [style=dashed`, `unmatched context target`} {
		if !strings.Contains(dot, expected) {
			t.Fatalf("missing %q in %s", expected, dot)
		}
	}
	if strings.Contains(dot, "c0 -> n1 [style=dashed") {
		t.Fatal("an interface contract must not be equated to a guessed concrete declaration")
	}
}

func TestContextDOTQuotesContextLabelsAndDetails(t *testing.T) {
	operation := ContextOperation{
		Kind: "derive", Context: "quoted\"context", Target: "context.WithTimeout", Parent: "unresolved",
		Detail: "timeout: quoted\"value", Location: "path\\with\ncontrols.go:1:1", Scope: "closure at quoted\"file",
	}
	root := Function{Name: "Run", Package: "app", Context: &ContextInfo{Operations: []ContextOperation{operation}}}
	dot := contextDOTOutput(t, &Graph{Root: root})
	for _, expected := range []string{
		"label=" + strconv.Quote(contextNodeLabel(operation)),
		"tooltip=" + strconv.Quote(contextOperationDetails(operation)),
	} {
		if !strings.Contains(dot, expected) {
			t.Fatalf("context data was not safely quoted: %s", dot)
		}
	}
}

func TestContextDOTPropagatesWriterErrors(t *testing.T) {
	root := Function{Name: "Run", Context: &ContextInfo{Parameters: []string{"ctx"}}}
	if err := writeContextDOT(failingWriter{}, &Graph{Root: root}, map[functionID]int{root.id(): 0}); err == nil {
		t.Fatal("context node write errors must be returned")
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
