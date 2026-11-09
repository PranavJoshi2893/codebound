package calltree

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/PranavJoshi2893/codebound/internal/calltree/diagram"
)

func TestContextIDsContinueThroughCalls(t *testing.T) {
	graph := analyzeContextFixture(t, `package app
import "context"
func Run() {
 ctx, cancel := context.WithCancel(context.Background())
 defer cancel()
 first(ctx)
}
func first(ctx context.Context) { second(ctx) }
func second(ctx context.Context) { <-ctx.Done() }
`)
	created := contextOperation(t, graph.Root.Context, "derive", "context.WithCancel")
	if !reflect.DeepEqual(created.IDs, []string{"C2"}) {
		t.Fatalf("expected one creation-site ID, got %v", created.IDs)
	}
	for _, call := range graph.Calls {
		if len(call.Contexts) != 1 || !reflect.DeepEqual(call.Contexts[0].IDs, created.IDs) || call.Contexts[0].Unresolved {
			t.Fatalf("context identity changed between %s and %s: %+v", call.Caller.Name, call.Callee.Name, call.Contexts)
		}
		if !reflect.DeepEqual(call.Callee.Context.Bindings[0].IDs, created.IDs) {
			t.Fatalf("callee lost incoming context identity: %+v", call.Callee.Context.Bindings)
		}
	}
	observed := contextOperation(t, functionContext(t, graph, "second"), "observe", "context.Context.Done")
	if !reflect.DeepEqual(observed.IDs, created.IDs) || observed.Unresolved {
		t.Fatalf("cancellation observation is not attached to the original context: %+v", observed)
	}
	dot := contextDOTOutput(t, graph)
	if strings.Count(dot, `label="ctx=C2"`) != 2 || strings.Count(dot, "shape=note") != 2 {
		t.Fatal("the same C2 must follow both calls, with no new parameter-context nodes")
	}
}

func TestContextArgumentsMatchPositionsNotNames(t *testing.T) {
	graph := analyzeContextFixture(t, `package app
import "context"
func Run(first, second context.Context) { swap(second, first) }
func swap(left, right context.Context) {}
`)
	want := []ContextBinding{{Parameter: "left", IDs: []string{"I2"}}, {Parameter: "right", IDs: []string{"I1"}}}
	if len(graph.Calls) != 1 || !reflect.DeepEqual(graph.Calls[0].Contexts, want) {
		t.Fatalf("arguments were not matched to the actual parameter positions: %+v", graph.Calls)
	}
	if strings.Contains(contextDOTOutput(t, graph), "shape=note") {
		t.Fatal("outside inputs must not be drawn as creations inside the tree")
	}
}

func TestContextArgumentsAccountForInterfaceReceiver(t *testing.T) {
	graph := analyzeContextFixture(t, `package app
import "context"
type runner interface { Start(context.Context) }
type worker struct{}
func (worker) Start(request context.Context) { _ = request.Err() }
func Run(ctx context.Context) { var service runner = worker{}; service.Start(ctx) }
`)
	if len(graph.Calls) != 1 || !graph.Calls[0].ThroughInterface {
		t.Fatalf("missing interface call: %+v", graph.Calls)
	}
	want := []ContextBinding{{Parameter: "request", IDs: []string{"I1"}}}
	if !reflect.DeepEqual(graph.Calls[0].Contexts, want) {
		t.Fatalf("receiver shifted the context argument: %+v", graph.Calls[0].Contexts)
	}
}

func TestContextBlankParametersRemainDistinct(t *testing.T) {
	graph := analyzeContextFixture(t, `package app
import "context"
func Run(first, second context.Context) { ignore(first, second) }
func ignore(_, _ context.Context) {}
`)
	if len(graph.Calls) != 1 {
		t.Fatal("missing ignore call")
	}
	label := contextDOTOutput(t, graph)
	if !strings.Contains(label, "_: I1") || !strings.Contains(label, "_: I2") {
		t.Fatalf("blank parameter names must not merge distinct argument positions: %s", label)
	}
}

func contextDOTOutput(t *testing.T, graph *Graph) string {
	t.Helper()
	var output bytes.Buffer
	if err := diagram.WriteDOT(&output, graph); err != nil {
		t.Fatal(err)
	}
	return output.String()
}

func TestContextFlowPreservesBranchesAndUnknowns(t *testing.T) {
	for _, tt := range []struct {
		name       string
		body       string
		ids        []string
		unresolved bool
	}{
		{"two inputs", "selected := first; if choose { selected = second }; work(selected)", []string{"I1", "I2"}, false},
		{"nil branch", "selected := first; if choose { selected = nil }; work(selected)", []string{"I1"}, true},
		{"escaped pointer", "change(&first); work(first)", []string{"I1"}, true},
		{"wrapper return", "work(wrap(first))", nil, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			graph := analyzeContextFixture(t, `package app
import "context"
func Run(first, second context.Context, choose bool) { `+tt.body+` }
func work(ctx context.Context) { _ = ctx.Err() }
func change(ctx *context.Context) { *ctx = context.Background() }
func wrap(ctx context.Context) context.Context { return ctx }
`)
			for _, call := range graph.Calls {
				if call.Callee.Name != "work" {
					continue
				}
				got := call.Contexts[0]
				if !reflect.DeepEqual(got.IDs, tt.ids) || got.Unresolved != tt.unresolved {
					t.Fatalf("wrong context possibilities: %+v", got)
				}
				used := contextOperation(t, call.Callee.Context, "read", "context.Context.Err")
				if !reflect.DeepEqual(used.IDs, tt.ids) || used.Unresolved != tt.unresolved {
					t.Fatalf("callee did not retain uncertainty: %+v", used)
				}
				return
			}
			t.Fatal("missing work call")
		})
	}
}

func TestDerivedContextHasItsOwnIdentityAndParent(t *testing.T) {
	graph := analyzeContextFixture(t, `package app
import (
 "context"
 "time"
)
func Run(ctx context.Context) {
 child, cancel := context.WithTimeout(ctx, 5*time.Second)
 defer cancel()
 detached := context.WithoutCancel(child)
 work(detached)
}
func work(ctx context.Context) {}
`)
	child := contextOperation(t, graph.Root.Context, "derive", "context.WithTimeout")
	detached := contextOperation(t, graph.Root.Context, "detach", "context.WithoutCancel")
	if !reflect.DeepEqual(child.ParentIDs, []string{"I1"}) || !reflect.DeepEqual(child.IDs, []string{"C1"}) {
		t.Fatalf("wrong derived context identity: %+v", child)
	}
	if !reflect.DeepEqual(detached.IDs, []string{"C2"}) || !reflect.DeepEqual(detached.ParentIDs, child.IDs) {
		t.Fatalf("wrong detached context identity: %+v", detached)
	}
	if !reflect.DeepEqual(graph.Calls[0].Contexts[0].IDs, detached.IDs) {
		t.Fatal("work should receive the detached context, not its parent")
	}
	dot := contextDOTOutput(t, graph)
	for _, expected := range []string{"CANCELLATION: detached; no deadline", "values only; cancellation detached", "timeout: 5s", "cancel in Run [deferred]"} {
		if !strings.Contains(dot, expected) {
			t.Errorf("missing lifecycle condition %q", expected)
		}
	}
}

func TestContextFlowTerminatesOnRecursion(t *testing.T) {
	graph := analyzeContextFixture(t, `package app
import "context"
func Run(ctx context.Context, again bool) {
 child, cancel := context.WithCancel(ctx)
 defer cancel()
 if again { Run(child, false) }
}
`)
	if !reflect.DeepEqual(graph.Root.Context.Bindings[0].IDs, []string{"C1", "I1"}) {
		t.Fatalf("recursive input possibilities were not retained: %+v", graph.Root.Context.Bindings)
	}
	if !reflect.DeepEqual(graph.Calls[0].Contexts[0].IDs, []string{"C1"}) {
		t.Fatal("recursive call must pass the child creation site")
	}
}

func TestContextFlowAcrossLongCallChain(t *testing.T) {
	var source strings.Builder
	source.WriteString("package app\nimport \"context\"\nfunc Run(ctx context.Context) { step0(ctx) }\n")
	const depth = 200
	for index := 0; index < depth; index++ {
		fmt.Fprintf(&source, "func step%d(ctx context.Context) { step%d(ctx) }\n", index, index+1)
	}
	fmt.Fprintf(&source, "func step%d(ctx context.Context) { _ = ctx.Err() }\n", depth)
	graph := analyzeContextFixture(t, source.String())
	last := functionContext(t, graph, fmt.Sprintf("step%d", depth))
	if !reflect.DeepEqual(last.Bindings[0].IDs, []string{"I1"}) || last.Bindings[0].Unresolved {
		t.Fatalf("input context was lost in a long chain: %+v", last.Bindings)
	}
}

func TestContextFlowRetainsGenericForwardingEvidence(t *testing.T) {
	graph := analyzeContextFixture(t, `package app
import "context"
func Run(first, second context.Context) { forward[int](first); forward[string](second) }
func forward[T any](ctx context.Context) { _ = ctx.Err() }
`)
	var forwarded [][]string
	for _, operation := range graph.Root.Context.Operations {
		if operation.Kind == "forward" {
			forwarded = append(forwarded, operation.IDs)
		}
	}
	if !reflect.DeepEqual(forwarded, [][]string{{"I1"}, {"I2"}}) {
		t.Fatalf("generic forwarding must keep its evidence even when synthetic wrappers aren't displayed: %v", forwarded)
	}
}
