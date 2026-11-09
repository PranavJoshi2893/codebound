package calltree

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestExecutionFlowPreservesEvaluationAndControlFlow(t *testing.T) {
	dir := fixture(t, map[string]string{"main.go": `package main
import "context"
func first() int { return 1 }
func second(int) {}
func cleanupA() {}
func cleanupB() {}
func gate() bool { return true }
func gate2() bool { return false }
func action(context.Context) {}
func main() {
 ctx, cancel := context.WithCancel(context.Background())
 defer cleanupA()
 defer cleanupB()
 defer cancel()
 defer second(first())
 second(first())
 if gate() && gate2() { action(ctx) } else { second(2) }
 for i := 0; i < 3; i++ {
  action(ctx)
  if gate() { break }
 }
 go action(ctx)
 go second(first())
 done := func() {
  child, stop := context.WithCancel(ctx)
  defer stop()
  action(child)
 }
 go done()
 defer done()
 done()
}
`})
	analysis, err := New(dir).Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	graph, err := analysis.Analyze(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	flow := graph.Root.Flow
	if flow == nil || len(flow.Scopes) != 2 || flow.Scopes[0].ID != "body" || flow.Scopes[1].ID != "L1" {
		t.Fatalf("named body and closure must have independent entries: %+v", flow)
	}
	if flow.Scopes[0].Recover == nil {
		t.Fatal("a deferred function's possible recovered-panic entry must not be treated as the next sequential block")
	}
	entry := flow.Scopes[0].Blocks[flow.Scopes[0].Entry]
	var labels []string
	for _, step := range entry.Steps {
		labels = append(labels, step.Label)
	}
	joined := strings.Join(labels, "|")
	if !strings.Contains(joined, "cleanupA()|cleanupB()|cancel()|first()|second(…)") {
		t.Fatalf("SSA evaluation order, not source-token order, must be preserved: %s", joined)
	}
	var firstGate, secondGate = -1, -1
	var branches, loops, runDefers, concurrent, deferred int
	sites := make(map[string]FlowStep)
	for _, scope := range flow.Scopes {
		for _, block := range scope.Blocks {
			if block.Exit == "branch" {
				branches++
				if len(block.Next) != 2 || block.Next[0].Label != "true" || block.Next[1].Label != "false" {
					t.Fatalf("conditional paths must retain their alternatives: %+v", block)
				}
			}
			for _, next := range block.Next {
				if next.BackEdge {
					loops++
				}
			}
			for stepIndex, step := range block.Steps {
				if _, duplicate := sites[step.ID]; duplicate || step.ID == "" {
					t.Fatalf("steps need stable, scope-specific identities: %+v", step)
				}
				sites[step.ID] = step
				if scope.ID == "body" && step.Label == "gate()" && firstGate < 0 {
					firstGate = block.ID
				}
				if scope.ID == "body" && step.Label == "gate2()" {
					secondGate = block.ID
				}
				if step.Label == "second(…)" && (step.Kind == "defer" || step.Kind == "go") {
					if stepIndex == 0 || block.Steps[stepIndex-1].Label != "first()" || block.Steps[stepIndex-1].Kind != "call" {
						t.Fatal("defer/go arguments must be evaluated now, before registration or launch")
					}
				}
				switch step.Kind {
				case "run_defers":
					runDefers++
				case "go":
					concurrent++
				case "defer":
					deferred++
				}
			}
		}
	}
	if firstGate < 0 || secondGate < 0 || firstGate == secondGate || branches < 3 || loops == 0 {
		t.Fatalf("short-circuit, branch, and loop structure lost: gate blocks=%d,%d branches=%d loops=%d", firstGate, secondGate, branches, loops)
	}
	if runDefers == 0 || concurrent != 3 || deferred != 6 {
		t.Fatalf("deferred registration, stack execution and goroutine launches must stay distinct: run=%d go=%d defer=%d", runDefers, concurrent, deferred)
	}
	for _, call := range graph.Calls {
		found := false
		for _, scope := range call.Caller.Flow.Scopes {
			for _, block := range scope.Blocks {
				for _, step := range block.Steps {
					if step.ID == call.Site {
						found = true
						if call.Callee.Name == "cleanupA" && (call.Execution != "deferred" || step.Kind != "defer") {
							t.Fatal("non-context defers also need execution semantics")
						}
					}
				}
			}
		}
		if !found {
			t.Fatalf("call is not attached to its exact execution step: %+v", call)
		}
	}
	for _, operation := range graph.Root.Context.Operations {
		if _, exists := sites[operation.Site]; !exists {
			t.Fatalf("context action lost its execution position: %+v", operation)
		}
		if operation.Scope != "" && !strings.HasPrefix(operation.Site, "q1") {
			t.Fatal("closure context actions must not be placed in the declaration's sequential path")
		}
	}
	firstJSON, _ := json.Marshal(graph)
	again, err := analysis.Analyze(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	secondJSON, _ := json.Marshal(again)
	if string(firstJSON) != string(secondJSON) {
		t.Fatal("execution steps and site identities must be deterministic across queries")
	}
}

func TestExecutionFlowChannelsSwitchAndPanic(t *testing.T) {
	dir := fixture(t, map[string]string{"main.go": `package main
func main() { work(make(chan int, 1), 2) }
func work(ch chan int, n int) {
 ch <- n
 n = <-ch
 select { case n = <-ch: n++; case ch <- n: n--; default: n = 0 }
 switch n { case 0: return; case 1: panic("failed"); default: n++ }
again:
 n--
 if n > 0 { goto again }
}
`})
	graph, err := New(dir).Analyze(context.Background(), "work")
	if err != nil {
		t.Fatal(err)
	}
	counts := make(map[string]int)
	for _, block := range graph.Root.Flow.Scopes[0].Blocks {
		counts[block.Exit]++
		if block.Exit == "branch" && (block.Condition == "0" || block.Condition == "1") {
			t.Fatalf("a switch case value is not its boolean comparison: %+v", block)
		}
		for _, next := range block.Next {
			if next.BackEdge {
				counts["loop"]++
			}
		}
		for _, step := range block.Steps {
			counts[step.Kind]++
		}
	}
	for _, kind := range []string{"send", "receive", "select", "branch", "return", "panic", "loop"} {
		if counts[kind] == 0 {
			t.Errorf("missing execution structure %s: %v", kind, counts)
		}
	}
}
