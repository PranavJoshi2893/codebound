package calltree

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestAnalyzeCallsAndBoundaries(t *testing.T) {
	dir := fixture(t, map[string]string{
		"main.go": `package app
import "example.com/project/handler"
type runner interface { Run() }
type worker struct{}
func (worker) Run() {}
func invoke(r runner) { r.Run() }
func execute(callback func()) { callback() }
func task() {}
func cycle() { cycle() }
func main() { invoke(worker{}); execute(task); handler.Handle(); cycle() }
`,
		"handler/handler.go": `package handler
import (
 "example.com/project/repository"
 "example.com/project/service"
)
func Handle() { service.Do(); repository.Get() }
`,
		"service/service.go": `package service
import "example.com/project/repository"
func Do() { repository.Get() }
`,
		"repository/repository.go": "package repository\nfunc Get() {}\n",
	})
	analyzer := New(dir)
	analyzer.Pattern = "./..."
	graph, err := analyzer.Analyze(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{graph.Root.Name: true}
	var violation, interfaceCall, recursion bool
	for _, call := range graph.Calls {
		names[call.Callee.Name] = true
		violation = violation || call.Violation == "handler must not call repository"
		interfaceCall = interfaceCall || (call.Callee.Name == "worker.Run" && call.ThroughInterface)
		recursion = recursion || (call.Caller.Name == "cycle" && call.Callee.Name == "cycle")
		if filepath.IsAbs(call.Callee.File) {
			t.Errorf("expected relative source path, got %q", call.Callee.File)
		}
	}
	for _, name := range []string{"main", "invoke", "worker.Run", "execute", "task", "Handle", "Do", "Get", "cycle"} {
		if !names[name] {
			t.Errorf("missing function %q", name)
		}
	}
	if !violation || !interfaceCall || !recursion {
		t.Fatalf("missing graph behavior: violation=%t interface=%t recursion=%t", violation, interfaceCall, recursion)
	}
	second, err := analyzer.Analyze(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, _ := json.Marshal(graph)
	secondJSON, _ := json.Marshal(second)
	if !bytes.Equal(firstJSON, secondJSON) {
		t.Fatal("analysis output is not deterministic")
	}
}

func TestAnalyzeSignalCleanupDoesNotCallMain(t *testing.T) {
	dir := fixture(t, map[string]string{"main.go": `package main
import (
 "context"
 "os/signal"
 "syscall"
)
func main() {
 _, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
 defer stop()
 run()
}
func run() {}
`})
	graph, err := New(dir).Analyze(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	// stop and main both have type func(), but stop never points to main.
	// The only displayed call should be the explicit call to run.
	if len(graph.Calls) != 1 || graph.Calls[0].Caller.Name != "main" || graph.Calls[0].Callee.Name != "run" {
		t.Fatalf("expected only main -> run, got %+v", graph.Calls)
	}
}

func TestAnalyzeCallbackTargets(t *testing.T) {
	for _, tt := range []struct {
		name   string
		source string
		want   []string
	}{
		{
			name: "passed as an argument",
			source: `package main
func main() { execute(task) }
func execute(callback func()) { callback() }
func task() {}
func unrelated() {}
`,
			want: []string{"main -> execute", "execute -> task"},
		},
		{
			name: "returned for deferred cleanup",
			source: `package main
func main() { stop := cleanup(); defer stop() }
func cleanup() func() { return task }
func task() {}
func unrelated() {}
`,
			want: []string{"main -> cleanup", "main -> task"},
		},
		{
			name: "both branches remain possible",
			source: `package main
func main() { callback := choose(true); callback() }
func choose(first bool) func() {
 if first { return taskA }
 return taskB
}
func taskA() {}
func taskB() {}
func unrelated() {}
`,
			want: []string{"main -> choose", "main -> taskA", "main -> taskB"},
		},
		{
			name: "real recursion through a callback",
			source: `package main
var callback func()
func init() { callback = main }
func main() { callback() }
func unrelated() {}
`,
			want: []string{"main -> main"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := fixture(t, map[string]string{"main.go": tt.source})
			graph, err := New(dir).Analyze(context.Background(), "main")
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, call := range graph.Calls {
				got = append(got, call.Caller.Name+" -> "+call.Callee.Name)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAnalysisReusesCallGraph(t *testing.T) {
	dir := fixture(t, map[string]string{"main.go": "package main\nfunc main() { task() }\nfunc task() {}\n"})
	analysis, err := New(dir).Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	first := analysis.callGraph()
	for _, root := range []string{"main", "task"} {
		if _, err := analysis.Analyze(context.Background(), root); err != nil {
			t.Fatal(err)
		}
		if analysis.callGraph() != first {
			t.Fatal("analysis rebuilt its call graph")
		}
	}
}

func TestConcurrentQueriesShareLoadedAnalysis(t *testing.T) {
	dir := fixture(t, map[string]string{"main.go": `package main
import "context"
func main() {
 ctx, cancel := context.WithCancel(context.Background())
 defer cancel()
 task(ctx)
}
func task(ctx context.Context) { _ = ctx.Err() }
`})
	analysis, err := New(dir).Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	const queries = 8
	results := make(chan error, queries)
	var workers sync.WaitGroup
	for i := 0; i < queries; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			graph, err := analysis.Analyze(context.Background(), "main")
			if err == nil && len(graph.Calls) != 1 {
				err = fmt.Errorf("unexpected call count: %d", len(graph.Calls))
			}
			if err == nil && (graph.Root.Context == nil || graph.Calls[0].Callee.Context == nil) {
				err = errors.New("missing context facts in concurrent query")
			}
			results <- err
		}()
	}
	workers.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestTraversalPreservesDepthFirstOrder(t *testing.T) {
	dir := fixture(t, map[string]string{"main.go": `package main
func main() { first(); second() }
func first() { shared() }
func second() { shared() }
func shared() { main() }
`})
	graph, err := New(dir).Analyze(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, call := range graph.Calls {
		got = append(got, call.Caller.Name+" -> "+call.Callee.Name)
	}
	want := []string{"main -> first", "first -> shared", "shared -> main", "main -> second", "second -> shared"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("call ordering changed: got %v, want %v", got, want)
	}
}

func TestDeepCallChain(t *testing.T) {
	const depth = 5000
	dir := fixture(t, map[string]string{"main.go": callChainSource(depth)})
	graph, err := New(dir).Analyze(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Calls) != depth {
		t.Fatalf("incomplete deep graph: got %d calls, want %d", len(graph.Calls), depth)
	}
}

func callChainSource(depth int) string {
	var source strings.Builder
	source.WriteString("package main\nfunc main() { task0() }\n")
	for i := 0; i < depth; i++ {
		fmt.Fprintf(&source, "func task%d() {", i)
		if i+1 < depth {
			fmt.Fprintf(&source, "task%d()", i+1)
		}
		source.WriteString("}\n")
	}
	return source.String()
}

func TestRootResolution(t *testing.T) {
	dir := fixture(t, map[string]string{
		"main.go":        "package app\nimport \"example.com/project/other\"\nfunc Run() { other.Run() }\n",
		"other/other.go": "package other\nfunc Run() {}\ntype Worker struct{}\nfunc (*Worker) Start() {}\n",
	})
	analyzer := New(dir)
	analyzer.Pattern = "./..."
	for _, name := range []string{"example.com/project.Run", "Worker.Start", "example.com/project/other.Worker.Start"} {
		if _, err := analyzer.Analyze(context.Background(), name); err != nil {
			t.Errorf("root %q: %v", name, err)
		}
	}
	for name, expected := range map[string]string{"Run": "ambiguous", "Missing": "not found"} {
		if _, err := analyzer.Analyze(context.Background(), name); err == nil || !strings.Contains(err.Error(), expected) {
			t.Errorf("root %q: expected %s, got %v", name, expected, err)
		}
	}
}

func TestAnalyzeErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := New(".").Analyze(ctx, "main"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	dir := fixture(t, map[string]string{"main.go": "package app\nfunc Broken( {}"})
	if _, err := New(dir).Analyze(context.Background(), "Broken"); err == nil {
		t.Fatal("invalid source was accepted")
	}
}

func fixture(t *testing.T, files map[string]string) string {
	t.Helper()
	t.Setenv("GOWORK", "off")
	dir := t.TempDir()
	files["go.mod"] = "module example.com/project\n\ngo 1.24\n"
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}
