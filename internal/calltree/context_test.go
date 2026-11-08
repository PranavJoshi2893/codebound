package calltree

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestContextFlowThroughProject(t *testing.T) {
	dir := fixture(t, map[string]string{"main.go": `package main
import (
 "context"
 "os/signal"
 "syscall"
 "time"
)
func main() {
 ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT)
 defer stop()
 run(ctx)
}
func run(ctx context.Context) {
 child, cancel := context.WithTimeout(ctx, 5*time.Second)
 defer cancel()
 alias := child
 go consume(alias)
 select { case <-ctx.Done(): default: }
 _ = ctx.Err()
 _, _ = ctx.Deadline()
 shutdown, finish := context.WithTimeout(context.Background(), time.Second)
 defer finish()
 consume(shutdown)
}
func consume(ctx context.Context) {}
`})
	analysis, err := New(dir).Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	graph, err := analysis.Analyze(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	main := graph.Root.Context
	if main == nil {
		t.Fatal("missing context facts for main")
	}
	signal := contextOperation(t, main, "derive", "signal.NotifyContext")
	if !strings.HasPrefix(signal.Parent, "context.Background at ") {
		t.Fatalf("wrong signal context parent: %+v", signal)
	}
	stop := contextOperation(t, main, "cancel", "cancel")
	if stop.Context != signal.Context || stop.Execution != "deferred" {
		t.Fatalf("stop must cancel the signal context on return: %+v", stop)
	}
	forward := contextOperation(t, main, "forward", "example.com/project.run")
	if forward.Context != signal.Context {
		t.Fatalf("main forwarded the wrong context: %+v", forward)
	}

	run := functionContext(t, graph, "run")
	if !reflect.DeepEqual(run.Parameters, []string{"ctx"}) {
		t.Fatalf("wrong context parameters: %v", run.Parameters)
	}
	child := contextOperation(t, run, "derive", "context.WithTimeout")
	if child.Parent != "parameter ctx" || child.Detail != "timeout: 5s" {
		t.Fatalf("wrong child context: %+v", child)
	}
	forward = contextOperation(t, run, "forward", "example.com/project.consume")
	if forward.Context != child.Context || forward.Execution != "goroutine" {
		t.Fatalf("context alias was not forwarded to the goroutine: %+v", forward)
	}
	observed := contextOperation(t, run, "observe", "context.Context.Done")
	if observed.Context != "parameter ctx" || observed.Detail != "select receive case" {
		t.Fatalf("wrong cancellation observation: %+v", observed)
	}
	contextOperation(t, run, "read", "context.Context.Err")
	contextOperation(t, run, "read", "context.Context.Deadline")
	var independent bool
	for _, operation := range run.Operations {
		if operation.Target == "context.WithTimeout" && operation.Detail == "timeout: 1s" {
			independent = strings.HasPrefix(operation.Parent, "context.Background at ")
		}
	}
	if !independent {
		t.Fatal("shutdown context must not inherit the incoming context")
	}
	consume := functionContext(t, graph, "consume")
	if len(consume.Operations) != 0 || !reflect.DeepEqual(consume.Parameters, []string{"ctx"}) {
		t.Fatalf("receiving a context must not imply observing it: %+v", consume)
	}

	second, err := analysis.Analyze(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, _ := json.Marshal(graph)
	secondJSON, _ := json.Marshal(second)
	if !bytes.Equal(firstJSON, secondJSON) || !bytes.Contains(firstJSON, []byte(`"context":`)) {
		t.Fatal("context JSON is missing or not deterministic")
	}
	var dot bytes.Buffer
	if err := WriteDOT(&dot, graph); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`label="ctx\ncontext input"`, "timeout: 5s", `observes Done`, "[deferred]", "[goroutine]"} {
		if !strings.Contains(dot.String(), expected) {
			t.Errorf("missing %q in DOT output", expected)
		}
	}
}

func TestContextAliasesAndClosures(t *testing.T) {
	graph := analyzeContextFixture(t, `package app
import c "context"
type RequestContext = c.Context
func Run(ctx RequestContext) {
 child, cancel := c.WithCancel(ctx)
 var stop func() = cancel
 defer stop()
 go func() {
  <-child.Done()
  consume(child)
  func() { _ = ctx.Err() }()
 }()
 defer func() { cancel() }()
}
func consume(ctx RequestContext) {}
`)
	info := graph.Root.Context
	if info == nil || !reflect.DeepEqual(info.Parameters, []string{"ctx"}) {
		t.Fatalf("type alias was not recognized: %+v", info)
	}
	child := contextOperation(t, info, "derive", "context.WithCancel")
	stop := contextOperation(t, info, "cancel", "cancel")
	if stop.Context != child.Context || stop.Execution != "deferred" {
		t.Fatalf("converted cancellation alias lost its context: %+v", stop)
	}
	observed := contextOperation(t, info, "observe", "context.Context.Done")
	if observed.Context != child.Context || !strings.Contains(observed.Scope, "goroutine closure at ") {
		t.Fatalf("captured context lost its origin or closure scope: %+v", observed)
	}
	read := contextOperation(t, info, "read", "context.Context.Err")
	if read.Context != "parameter ctx" || !strings.Contains(read.Scope, " > direct closure at ") {
		t.Fatalf("nested closure lost its origin or scope: %+v", read)
	}
	var deferredClosure bool
	for _, operation := range info.Operations {
		if operation.Kind == "cancel" && strings.HasPrefix(operation.Scope, "deferred closure at ") {
			deferredClosure = operation.Context == child.Context
		}
	}
	if !deferredClosure {
		t.Fatal("missing cancellation inside deferred closure")
	}
}

func TestContextMethodValues(t *testing.T) {
	graph := analyzeContextFixture(t, `package app
import "context"
func Run(ctx context.Context) {
 done := ctx.Done
 <-done()
 readError := ctx.Err
 _ = readError()
 deadline := context.Context.Deadline
 _, _ = deadline(ctx)
}
`)
	info := graph.Root.Context
	for _, method := range []string{"Done", "Err", "Deadline"} {
		operation := contextOperation(t, info, "read", "context.Context."+method)
		if operation.Context != "parameter ctx" {
			t.Fatalf("method value lost its receiver: %+v", operation)
		}
	}
	contextOperation(t, info, "observe", "context.Context.Done")
}

func TestContextClosureParametersKeepTheirOwnIdentity(t *testing.T) {
	graph := analyzeContextFixture(t, `package app
import "context"
func Run(ctx context.Context) {
 go func(ctx context.Context) { <-ctx.Done() }(context.Background())
 go func() { <-ctx.Done() }()
}
`)
	var captured, local bool
	for _, operation := range graph.Root.Context.Operations {
		if operation.Kind != "observe" {
			continue
		}
		captured = captured || operation.Context == "parameter ctx"
		local = local || strings.HasPrefix(operation.Context, "parameter ctx (closure at ")
	}
	if !captured || !local {
		t.Fatal("a shadowed closure parameter must not be mistaken for the outer context")
	}
	dot := contextDOTOutput(t, graph)
	if strings.Count(dot, `\ncontext input"`) != 2 {
		t.Fatal("outer and shadowed closure parameters must have separate context nodes")
	}
}

func TestContextConstructorsAndStatusReads(t *testing.T) {
	graph := analyzeContextFixture(t, `package app
import (
 "context"
 "time"
)
func Run(ctx context.Context, delay time.Duration) {
 a, cancelA := context.WithCancelCause(ctx)
 defer cancelA(nil)
 b, cancelB := context.WithTimeoutCause(a, delay, nil)
 defer cancelB()
 c, cancelC := context.WithDeadline(b, time.Now())
 defer cancelC()
 d, cancelD := context.WithDeadlineCause(c, time.Now(), nil)
 defer cancelD()
 values := context.WithValue(d, "key", "private-value")
 detached := context.WithoutCancel(values)
 _ = detached.Err()
 _ = context.Cause(detached)
 _ = values.Value("key")
 _ = ctx.Done()
 consume(context.TODO())
}
func consume(ctx context.Context) {}
`)
	info := graph.Root.Context
	for _, target := range []string{"context.WithCancelCause", "context.WithTimeoutCause", "context.WithDeadline", "context.WithDeadlineCause", "context.WithValue"} {
		contextOperation(t, info, "derive", target)
	}
	timeout := contextOperation(t, info, "derive", "context.WithTimeoutCause")
	if timeout.Detail != "timeout: dynamic" {
		t.Fatalf("a runtime timeout must not be guessed: %+v", timeout)
	}
	detached := contextOperation(t, info, "detach", "context.WithoutCancel")
	if !strings.Contains(detached.Detail, "not inherited") || !strings.HasPrefix(detached.Parent, "context.WithValue at ") {
		t.Fatalf("wrong detached context: %+v", detached)
	}
	contextOperation(t, info, "read", "context.Cause")
	contextOperation(t, info, "read", "context.Context.Value")
	contextOperation(t, info, "create", "context.TODO")
	for _, operation := range info.Operations {
		if operation.Kind == "observe" {
			t.Fatalf("calling Done without receiving must not count as observing cancellation: %+v", operation)
		}
	}
	encoded, _ := json.Marshal(info)
	if bytes.Contains(encoded, []byte("private-value")) {
		t.Fatal("context values should not be exported")
	}
}

func TestContextBranchOriginsAndUnresolvedSources(t *testing.T) {
	for _, tt := range []struct {
		name   string
		source string
		origin string
	}{
		{
			name: "branch merge",
			source: `package app
import "context"
func Run(ctx, other context.Context, choose bool) {
 selected := ctx
 if choose { selected = other }
 consume(selected)
}
func consume(ctx context.Context) {}
`,
			origin: "parameter ctx or parameter other",
		},
		{
			name: "loop merge",
			source: `package app
import "context"
func Run(ctx context.Context, again bool) {
 selected := ctx
 for again { consume(selected); again = false }
 consume(selected)
}
func consume(ctx context.Context) {}
`,
			origin: "parameter ctx",
		},
		{
			name: "field",
			source: `package app
import "context"
type holder struct { ctx context.Context }
func Run(h holder) { consume(h.ctx) }
func consume(ctx context.Context) {}
`,
			origin: "unresolved",
		},
		{
			name: "global",
			source: `package app
import "context"
var ctx = context.Background()
func Run() { consume(ctx) }
func consume(ctx context.Context) {}
`,
			origin: "unresolved",
		},
		{
			name: "custom wrapper",
			source: `package app
import "context"
func wrapped(ctx context.Context) context.Context { return ctx }
func Run(ctx context.Context) { consume(wrapped(ctx)) }
func consume(ctx context.Context) {}
`,
			origin: "unresolved return from example.com/project.wrapped",
		},
		{
			name: "escaped pointer",
			source: `package app
import "context"
func change(ctx *context.Context) { *ctx = context.Background() }
func Run(ctx context.Context) { change(&ctx); consume(ctx) }
func consume(ctx context.Context) {}
`,
			origin: "parameter ctx or unresolved",
		},
		{
			name: "escaped captured pointer",
			source: `package app
import "context"
func change(ctx *context.Context) { *ctx = context.Background() }
func Run(ctx context.Context) {
 func() { change(&ctx) }()
 consume(ctx)
}
func consume(ctx context.Context) {}
`,
			origin: "parameter ctx or unresolved",
		},
		{
			name: "nil",
			source: `package app
import "context"
func Run() { consume(nil) }
func consume(ctx context.Context) {}
`,
			origin: "nil",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			graph := analyzeContextFixture(t, tt.source)
			operation := contextOperation(t, graph.Root.Context, "forward", "example.com/project.consume")
			if operation.Context != tt.origin {
				t.Fatalf("got origin %q, want %q", operation.Context, tt.origin)
			}
		})
	}
}

func TestContextNamesAreNotEnough(t *testing.T) {
	graph := analyzeContextFixture(t, `package app
import "context"
func WithCancel(ctx context.Context) (context.Context, context.CancelFunc) { return context.WithCancel(ctx) }
func Run(ctx context.Context) {
 child, cancel := WithCancel(ctx)
 defer cancel()
 consume(child)
}
func consume(ctx context.Context) {}
`)
	info := graph.Root.Context
	for _, operation := range info.Operations {
		if operation.Kind == "derive" || operation.Kind == "create" {
			t.Fatalf("a custom wrapper must not be mistaken for the standard API: %+v", operation)
		}
	}
	cancel := contextOperation(t, info, "cancel", "cancel")
	if cancel.Context != "unresolved" {
		t.Fatalf("custom cancellation return should remain unresolved: %+v", cancel)
	}
	forward := contextOperation(t, info, "forward", "example.com/project.consume")
	if forward.Context != "unresolved" {
		t.Fatalf("custom tuple return should remain unresolved: %+v", forward)
	}
}

func TestContextBuiltinsAreNotCallbacks(t *testing.T) {
	graph := analyzeContextFixture(t, `package app
import "context"
func Run(ctx context.Context) { println(ctx) }
`)
	if info := graph.Root.Context; info == nil || len(info.Operations) != 0 {
		t.Fatalf("builtin usage must not be labeled as an unresolved callback: %+v", info)
	}
}

func TestContextMetadataOmittedWhenUnused(t *testing.T) {
	graph := analyzeContextFixture(t, "package app\nfunc Run() {}\n")
	if graph.Root.Context != nil {
		t.Fatal("function without context should have no context metadata")
	}
	encoded, _ := json.Marshal(graph)
	if bytes.Contains(encoded, []byte(`"context":`)) {
		t.Fatal("empty context metadata should not change existing JSON")
	}
}

func analyzeContextFixture(t *testing.T, source string) *Graph {
	t.Helper()
	dir := fixture(t, map[string]string{"app.go": source})
	graph, err := New(dir).Analyze(context.Background(), "Run")
	if err != nil {
		t.Fatal(err)
	}
	return graph
}

func functionContext(t *testing.T, graph *Graph, name string) *ContextInfo {
	t.Helper()
	if graph.Root.Name == name && graph.Root.Context != nil {
		return graph.Root.Context
	}
	for _, call := range graph.Calls {
		if call.Callee.Name == name && call.Callee.Context != nil {
			return call.Callee.Context
		}
	}
	t.Fatalf("missing context info for function %q", name)
	return nil
}

func contextOperation(t *testing.T, info *ContextInfo, kind, target string) ContextOperation {
	t.Helper()
	if info != nil {
		for _, operation := range info.Operations {
			if operation.Kind == kind && operation.Target == target {
				return operation
			}
		}
	}
	t.Fatalf("missing %s operation for %s in %+v", kind, target, info)
	return ContextOperation{}
}
