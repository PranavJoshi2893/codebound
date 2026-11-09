package calltree

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHiddenClosuresAndBoundMethods(t *testing.T) {
	dir := fixture(t, map[string]string{"main.go": `package main
import "context"
type Worker struct{}
func (Worker) Run(ctx context.Context) { service(ctx) }
func service(ctx context.Context) { _ = ctx.Err() }
func unrelated() {}
func main() {
 ctx := context.Background()
 go func() { callback := (Worker{}).Run; callback(ctx) }()
 defer func() { service(ctx) }()
 unused := func() { unrelated() }; _ = unused
}
`})
	graph, err := New(dir).Analyze(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	var bound, closure bool
	for _, call := range graph.Calls {
		if call.Callee.Name == "unrelated" {
			t.Fatal("displayed an uninvoked closure's target")
		}
		if call.Caller.Name == "main" && call.Callee.Name == "Worker.Run" {
			bound = true
			if call.Path == nil || call.Execution != "goroutine" {
				t.Fatalf("lost goroutine/wrapper provenance: %+v", call)
			}
			checkPath(t, graph, call)
		}
		if call.Caller.Name == "main" && call.Callee.Name == "service" {
			closure = true
			if call.Path == nil || call.Execution != "deferred" {
				t.Fatalf("lost deferred closure: %+v", call)
			}
		}
		if len(call.Contexts) != 1 || call.Contexts[0].Unresolved || len(call.Contexts[0].IDs) != 1 {
			t.Fatalf("lost captured/forwarded context: %+v", call)
		}
	}
	if !bound || !closure {
		t.Fatalf("missing hidden paths: %+v", graph.Calls)
	}
}

func frameworkFixture(t *testing.T, source, framework string) *Analysis {
	t.Helper()
	dir := fixture(t, map[string]string{
		"main.go":                source,
		"framework/go.mod":       "module example.com/framework\n\ngo 1.24\n",
		"framework/framework.go": framework,
	})
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/project\n\ngo 1.24\n\nrequire example.com/framework v0.0.0\nreplace example.com/framework => ./framework\n"), 0644); err != nil {
		t.Fatal(err)
	}
	a, err := New(dir).Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestFrameworkHTTPDispatchReachesHandlerServices(t *testing.T) {
	a := frameworkFixture(t, `package main
import (
 "context"
 "net/http"
 "example.com/framework"
)
type Service struct{}
func (*Service) CreateItem(ctx context.Context) { _ = ctx.Err() }
func (*Service) GetItemByID(ctx context.Context) { _ = ctx.Err() }
func (*Service) ListItems(ctx context.Context) { _ = ctx.Err() }
type Handler struct { service *Service }
func NewHandler(s *Service) *Handler { return &Handler{s} }
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) { h.service.CreateItem(r.Context()) }
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) { h.service.GetItemByID(r.Context()) }
func (h *Handler) List(w http.ResponseWriter, r *http.Request) { h.service.ListItems(r.Context()) }
func (h *Handler) Unused(w http.ResponseWriter, r *http.Request) {}
func ItemRoutes(r *framework.Router, h *Handler) { r.Register(h.Create); r.Register(h.GetByID); r.Register(h.List) }
func Routes(h *Handler) *framework.Router { r := &framework.Router{}; r.Route(func(r *framework.Router) { ItemRoutes(r, h) }); return r }
func Run(ctx context.Context, r http.Handler) { go func() { server := &http.Server{Handler:r}; server.ListenAndServe() }(); _ = ctx.Done() }
func main() { h := NewHandler(&Service{}); r := Routes(h); Run(context.Background(), r) }
`, `package framework
import "net/http"
type Router struct { handlers []http.HandlerFunc }
func (r *Router) Route(configure func(*Router)) { configure(r) }
func (r *Router) Register(h http.HandlerFunc) { r.handlers = append(r.handlers, h) }
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) { for _, h := range r.handlers { h.ServeHTTP(w, req) } }
`)
	graph, err := a.Analyze(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"Handler.Create": "Service.CreateItem", "Handler.GetByID": "Service.GetItemByID", "Handler.List": "Service.ListItems"}
	dispatched, services := make(map[string]bool), make(map[string]bool)
	var registration bool
	for _, call := range graph.Calls {
		if strings.Contains(call.Callee.Package, "framework") || call.Callee.Name == "Handler.Unused" {
			t.Fatalf("unwanted visible function: %+v", call)
		}
		if call.Caller.Name == "Routes" && call.Callee.Name == "ItemRoutes" {
			registration = true
			if call.Path == nil || call.Path.HTTPDispatch {
				t.Fatal("route configuration confused with HTTP dispatch")
			}
		}
		if _, ok := want[call.Callee.Name]; ok {
			if call.Caller.Name != "Run" || call.Path == nil || !call.Path.HTTPDispatch || call.Execution != "goroutine" {
				t.Fatalf("handler has no genuine dispatch provenance: %+v", call)
			}
			dispatched[call.Callee.Name] = true
			checkPath(t, graph, call)
		}
		if target, ok := want[call.Caller.Name]; ok && call.Callee.Name == target {
			services[call.Caller.Name] = true
			if call.Path != nil || call.Site == "" {
				t.Fatal("direct service call lost its real site")
			}
			if len(call.Contexts) != 1 || !call.Contexts[0].Unresolved || len(call.Contexts[0].IDs) != 0 {
				t.Fatalf("invented startup/request context binding: %+v", call.Contexts)
			}
		}
		if call.Caller.Name == "NewHandler" {
			t.Fatal("constructor presented as dispatch")
		}
	}
	if !registration || len(dispatched) != 3 || len(services) != 3 {
		t.Fatalf("incomplete graph: registration=%v dispatch=%v services=%v", registration, dispatched, services)
	}
	first, _ := json.Marshal(graph)
	secondGraph, err := a.Analyze(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	second, _ := json.Marshal(secondGraph)
	if !bytes.Equal(first, second) {
		t.Fatal("hidden-path output is not deterministic")
	}
	for _, root := range []string{"Routes", "NewHandler"} {
		startup, err := a.Analyze(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		for _, call := range startup.Calls {
			if _, ok := want[call.Callee.Name]; ok {
				t.Fatalf("%s registration/construction invented a handler invocation: %+v", root, call)
			}
		}
	}
}

func TestFrameworkPathsPreserveAlternativeSitesAndContextUncertainty(t *testing.T) {
	a := frameworkFixture(t, `package main
import ("context"; "example.com/framework")
type Worker struct{}
func (Worker) Run(ctx context.Context) { service(ctx) }
func service(ctx context.Context) { _ = ctx.Err() }
func main() { framework.Execute(context.Background(), (Worker{}).Run) }
`, `package framework
import "context"
func Execute(ctx context.Context, callback func(context.Context)) { callback(ctx); callback(nil) }
`)
	graph, err := a.Analyze(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, call := range graph.Calls {
		if call.Callee.Name == "Worker.Run" {
			found = true
			checkPath(t, graph, call)
			if len(call.Contexts) != 1 || len(call.Contexts[0].IDs) != 1 || !call.Contexts[0].Unresolved {
				t.Fatalf("lost known + unknown origins through wrapper: %+v", call)
			}
		}
	}
	if !found {
		t.Fatal("missing bound callback")
	}
	var sites []string
	for _, edge := range graph.Evidence.Edges {
		if strings.HasSuffix(graph.Evidence.Nodes[edge.From].ID, "framework.Execute") && edge.Dynamic {
			sites = append(sites, edge.Instruction)
		}
	}
	if len(sites) != 2 || sites[0] == sites[1] {
		t.Fatalf("alternative same-line call sites were merged: %v", sites)
	}
}

func checkPath(t *testing.T, graph *Graph, call Call) {
	t.Helper()
	if call.Path == nil || graph.Evidence == nil {
		t.Fatal("missing indirect evidence")
	}
	entry, exit := graph.Evidence.Edges[call.Path.Entry], graph.Evidence.Edges[call.Path.Exit]
	if !graph.Evidence.Nodes[entry.From].Visible || !graph.Evidence.Nodes[exit.To].Visible || entry.Instruction == "" || exit.Instruction == "" {
		t.Fatalf("lost actual entry/exit identity: %+v %+v", entry, exit)
	}
	if call.Site != entry.Site {
		t.Fatal("call anchored to an invented source row")
	}
	// Find a real hidden-only corridor with the selected dispatch state.
	type state struct {
		node int
		http bool
	}
	pending := []state{{entry.To, entry.HTTPDispatch}}
	seen := make(map[state]bool)
	for len(pending) > 0 {
		current := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if seen[current] || graph.Evidence.Nodes[current.node].Visible {
			continue
		}
		seen[current] = true
		if current.node == exit.From && (current.http || exit.HTTPDispatch) == call.Path.HTTPDispatch {
			return
		}
		for _, edge := range graph.Evidence.Edges {
			if edge.From == current.node {
				pending = append(pending, state{edge.To, current.http || edge.HTTPDispatch})
			}
		}
	}
	t.Fatal("projected path has no supporting hidden corridor")
}

func TestHiddenFrameworkCyclesRetainCallsAndSourceOrder(t *testing.T) {
	a := frameworkFixture(t, `package main
import "example.com/framework"
func main() { framework.Execute(first, second) }
func first() {}
func second() {}
func unrelated() {}
`, `package framework
func Execute(a, b func()) { a(); repeat(a, b); b() }
func repeat(a, b func()) { Execute(a, b) }
`)
	graph, err := a.Analyze(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Calls) != 2 || graph.Calls[0].Callee.Name != "first" || graph.Calls[1].Callee.Name != "second" {
		t.Fatalf("cycle lost calls/order or added unrelated functions: %+v", graph.Calls)
	}
	var recursiveEvidence bool
	for _, edge := range graph.Evidence.Edges {
		from, to := graph.Evidence.Nodes[edge.From].ID, graph.Evidence.Nodes[edge.To].ID
		recursiveEvidence = recursiveEvidence || (strings.HasSuffix(from, ".repeat") && strings.HasSuffix(to, ".Execute"))
	}
	if !recursiveEvidence {
		t.Fatal("hidden cycle was removed from provenance")
	}
	for _, call := range graph.Calls {
		checkPath(t, graph, call)
	}
}

func TestHiddenContextCreationRemainsTraceable(t *testing.T) {
	a := frameworkFixture(t, `package main
import ("context"; "example.com/framework")
func main() { framework.Execute(service) }
func service(ctx context.Context) { _ = ctx.Err() }
`, `package framework
import "context"
func Execute(callback func(context.Context)) { callback(context.Background()) }
`)
	graph, err := a.Analyze(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Calls) != 1 || len(graph.Calls[0].Contexts) != 1 {
		t.Fatalf("missing framework context: %+v", graph.Calls)
	}
	binding := graph.Calls[0].Contexts[0]
	if binding.Unresolved || len(binding.IDs) != 1 {
		t.Fatalf("known hidden origin lost: %+v", binding)
	}
	for _, node := range graph.Evidence.Nodes {
		if node.Context == nil {
			continue
		}
		for _, operation := range node.Context.Operations {
			if operation.Kind == "create" && len(operation.IDs) == 1 && operation.IDs[0] == binding.IDs[0] && operation.Location != "" {
				return
			}
		}
	}
	t.Fatal("context ID has no exported creation-site provenance")
}

func TestFunctionValueInvocationIsMarkedDynamic(t *testing.T) {
	dir := fixture(t, map[string]string{"main.go": `package main
func main() { callback := choose(); callback() }
func choose() func() { return task }
func task() {}
`})
	graph, err := New(dir).Analyze(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, call := range graph.Calls {
		if call.Callee.Name != "task" {
			continue
		}
		found = true
		if !call.Dynamic || call.Path != nil || call.Site == "" {
			t.Fatalf("lost actual function-value invocation: %+v", call)
		}
	}
	if !found {
		t.Fatal("missing callback target")
	}
}
