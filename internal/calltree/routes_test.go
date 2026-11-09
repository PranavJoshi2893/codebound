package calltree

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/ssa"
)

func TestServeMuxRegistrationsAreNotCalls(t *testing.T) {
	dir := fixture(t, map[string]string{"main.go": `package main
import (web "net/http"; "os")
type Handler struct{}
func (*Handler) Get(w web.ResponseWriter, r *web.Request) {}
func (*Handler) ServeHTTP(w web.ResponseWriter, r *web.Request) {}
func list(w web.ResponseWriter, r *web.Request) {}
func configure(m *web.ServeMux, h *Handler) {
 m.HandleFunc("GET example.com/items/{id}", h.Get)
 m.Handle("POST /items", h)
 m.Handle("/adapted/{$}", web.HandlerFunc(list))
 m.HandleFunc("/assets/{rest...}", list)
 m.HandleFunc(os.Args[1], list)
 m.HandleFunc("/nil", nil)
}
func unused() { web.HandleFunc("GET /unreachable", list) }
func main() {
 first := web.NewServeMux(); second := web.NewServeMux()
 h := &Handler{}
 configure(first, h)
 second.HandleFunc("GET /items/{id}", h.Get)
 web.HandleFunc("GET /default", list)
 web.Handle("/default-handler", h)
}
`})
	a := New(dir)
	graph, err := a.Analyze(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Registrations) != 9 {
		t.Fatalf("expected nine root-reachable registrations, got %+v", graph.Registrations)
	}
	byPattern := make(map[string]RouteRegistration)
	for _, route := range graph.Registrations {
		byPattern[route.DeclaredPattern] = route
	}
	get := byPattern["GET example.com/items/{id}"]
	if get.Method != "GET" || get.Host != "example.com" || get.Pattern != "example.com/items/{id}" || !get.PatternKnown || get.Handler == nil || get.Handler.Name != "Handler.Get" || get.Site == "" {
		t.Fatalf("lost typed method/pattern/handler/source evidence: %+v", get)
	}
	if post := byPattern["POST /items"]; post.Handler == nil || post.Handler.Name != "Handler.ServeHTTP" {
		t.Fatalf("concrete Handler was not resolved: %+v", post)
	}
	if adapted := byPattern["/adapted/{$}"]; adapted.Method != "*" || adapted.Handler == nil || adapted.Handler.Name != "list" {
		t.Fatalf("HandlerFunc adapter or methodless pattern lost: %+v", adapted)
	}
	if get.Mux == byPattern["GET /items/{id}"].Mux || get.Mux == byPattern["GET /default"].Mux {
		t.Fatal("distinct mux instances were mixed")
	}
	if byPattern["GET /default"].Mux != "net/http.DefaultServeMux" || byPattern["/default-handler"].Mux != "net/http.DefaultServeMux" {
		t.Fatal("package-level registration lost default mux identity")
	}
	if nilRoute := byPattern["/nil"]; nilRoute.Handler != nil || !strings.Contains(nilRoute.Issue, "nil handler") {
		t.Fatalf("nil handler invented a destination: %+v", nilRoute)
	}
	if unknown := byPattern[""]; unknown.PatternKnown || unknown.MethodKnown || unknown.Handler == nil || unknown.Issue == "" {
		t.Fatalf("dynamic pattern must remain unknown without losing a known target: %+v", unknown)
	}
	for _, call := range graph.Calls {
		if call.Callee.Name == "Handler.Get" || call.Callee.Name == "Handler.ServeHTTP" || call.Callee.Name == "list" {
			t.Fatalf("registration invented an invocation: %+v", call)
		}
	}
	first, _ := json.Marshal(graph)
	second, err := a.Analyze(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	next, _ := json.Marshal(second)
	if string(first) != string(next) {
		t.Fatal("registration analysis is not deterministic")
	}
}

// Local replacement uses Chi's real package/type/API identities without a
// network dependency. Separate integration checks exercise the actual library.
func chiRouteFixture(t *testing.T, source string) *Analysis {
	t.Helper()
	dir := fixture(t, map[string]string{
		"main.go":    source,
		"chi/go.mod": "module github.com/go-chi/chi/v5\n\ngo 1.24\n",
		"chi/chi.go": `package chi
import "net/http"
type Router interface {
 Get(string, http.HandlerFunc); Post(string, http.HandlerFunc)
 MethodFunc(string,string,http.HandlerFunc)
 Route(string,func(Router)) Router; Group(func(Router)) Router
}
type Mux struct{}
func NewRouter() *Mux { return &Mux{} }
func (*Mux) Get(string,http.HandlerFunc) {}
func (*Mux) Post(string,http.HandlerFunc) {}
func (*Mux) MethodFunc(string,string,http.HandlerFunc) {}
func (m *Mux) Route(p string, f func(Router)) Router { f(m); return m }
func (m *Mux) Group(f func(Router)) Router { f(m); return m }
func (*Mux) Mount(string,http.Handler) {}
func (*Mux) ServeHTTP(http.ResponseWriter,*http.Request) {}
`,
	})
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/project\n\ngo 1.24\n\nrequire github.com/go-chi/chi/v5 v5.0.0\nreplace github.com/go-chi/chi/v5 => ./chi\n"), 0644); err != nil {
		t.Fatal(err)
	}
	a, err := New(dir).Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestChiRegistrationPrefixesStayWithTheirActualCallbacks(t *testing.T) {
	a := chiRouteFixture(t, `package main
import ("net/http"; "github.com/go-chi/chi/v5")
type Handler struct{}
func (*Handler) List(http.ResponseWriter,*http.Request) {}
func (*Handler) Create(http.ResponseWriter,*http.Request) {}
func Items(r chi.Router, h *Handler) { r.Get("/", h.List); r.Post("/", h.Create) }
func Inventory(r chi.Router) { r.Get("/",nil) }
func configure(h *Handler) {
 r := chi.NewRouter()
 r.Route("/items", func(child chi.Router) { child.Group(func(group chi.Router) { Items(group,h) }) })
 r.Route("/other", func(child chi.Router) { Items(child,h) })
 r.Route("/inventory", Inventory)
 r.MethodFunc("HEAD", "/health/{$}", h.List)
}
func main() { configure(&Handler{}) }
`)
	graph, err := a.Analyze(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Registrations) != 6 {
		t.Fatalf("expected six precise registrations: %+v", graph.Registrations)
	}
	seen := make(map[string]bool)
	for _, route := range graph.Registrations {
		if !route.PatternKnown || !route.MethodKnown || route.Site == "" {
			t.Fatalf("lost exact route source/prefix: %+v", route)
		}
		seen[route.Method+" "+route.Pattern] = true
		if route.Pattern == "/inventory/" {
			if route.Handler != nil || route.Issue != "nil handler" {
				t.Fatalf("inventory registration borrowed an item handler: %+v", route)
			}
		} else if route.Handler == nil {
			t.Fatalf("missing registered method: %+v", route)
		}
	}
	for _, pattern := range []string{"GET /items/", "POST /items/", "GET /other/", "POST /other/", "GET /inventory/", "HEAD /health/{$}"} {
		if !seen[pattern] {
			t.Errorf("missing %s", pattern)
		}
	}
	for _, call := range graph.Calls {
		if strings.HasPrefix(call.Callee.Name, "Handler.") {
			t.Fatal("route configuration must not invoke handlers")
		}
	}
}

func TestRegistrationUncertaintyAndUnrelatedAPIs(t *testing.T) {
	dir := fixture(t, map[string]string{"main.go": `package main
import ("net/http"; "os")
type Other struct{}
func (Other) HandleFunc(string,http.HandlerFunc) {}
func one(http.ResponseWriter,*http.Request) {}
func two(http.ResponseWriter,*http.Request) {}
func main() {
 h := one
 if len(os.Args)>1 { h = one } else { h = two }
 read:=func()http.HandlerFunc{return h}; _=read()
 mux := http.NewServeMux()
 mux.HandleFunc("GET /ambiguous",h)
 var maybe http.HandlerFunc
 if len(os.Args)>1 { maybe=one }
 readMaybe:=func()http.HandlerFunc{return maybe}
 mux.HandleFunc("GET /maybe",readMaybe())
 Other{}.HandleFunc("GET /not-http",one)
}
`})
	graph, err := New(dir).Analyze(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Registrations) != 2 || graph.Registrations[0].Handler != nil || graph.Registrations[0].Issue != "handler unresolved" || graph.Registrations[1].Handler != nil || graph.Registrations[1].Issue != "handler unresolved" {
		t.Fatalf("ambiguous handlers must not become guessed links: %+v", graph.Registrations)
	}
}

func TestServeMuxInitializationAliasesAndValueReceivers(t *testing.T) {
	dir := fixture(t, map[string]string{"main.go": `package main
import "net/http"
var mux = http.NewServeMux()
var zero http.ServeMux
type Handler struct{}
func (Handler) ServeHTTP(http.ResponseWriter,*http.Request) {}
func list(http.ResponseWriter,*http.Request) {}
func init() { http.HandleFunc("GET /initialized",list) }
func configure(m *http.ServeMux) *http.ServeMux { alias:=m; alias.Handle("POST /global",Handler{}); return alias }
func main() { configure(mux).HandleFunc("/alias/{$}",list); zero.HandleFunc("/zero",list) }
`})
	graph, err := New(dir).Analyze(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Registrations) != 4 {
		t.Fatalf("missing initializer/global/zero/alias registrations: %+v", graph.Registrations)
	}
	var instance string
	for _, route := range graph.Registrations {
		if route.Handler == nil || route.Issue != "" || !route.PatternKnown {
			t.Fatalf("lost known handler/router evidence: %+v", route)
		}
		switch route.Pattern {
		case "/initialized":
			if route.Registrar.Name != "init" || route.Mux != "net/http.DefaultServeMux" {
				t.Fatalf("initialization must have its own real source owner: %+v", route)
			}
		case "/global":
			instance = route.Mux
			if route.Handler.Name != "Handler.ServeHTTP" {
				t.Fatal("value receiver handler was not resolved")
			}
		case "/alias/{$}":
			if instance != "" && route.Mux != instance {
				t.Fatal("helper return/alias lost mux identity")
			}
		case "/zero":
			if route.Mux == instance {
				t.Fatal("zero-value mux mixed with global constructed mux")
			}
		}
	}
	for _, call := range graph.Calls {
		if call.Callee.Name == "init" || call.Callee.Name == "list" || call.Callee.Name == "Handler.ServeHTTP" {
			t.Fatal("initialization registration must not invent invocation edges")
		}
	}
}

func TestInlineServeMuxHandlersHaveDistinctRegistrationIdentities(t *testing.T) {
	dir := fixture(t, map[string]string{"main.go": `package main
import "net/http"
func main() {
 mux:=http.NewServeMux()
 h:=func(http.ResponseWriter,*http.Request) {}
 mux.HandleFunc("GET /first",h)
 mux.HandleFunc("POST /second",func(http.ResponseWriter,*http.Request){})
}
`})
	graph, err := New(dir).Analyze(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Registrations) != 2 {
		t.Fatal("inline handler registrations were lost")
	}
	first, second := graph.Registrations[0].Handler, graph.Registrations[1].Handler
	if first == nil || second == nil || first.Identity() == second.Identity() || !strings.HasPrefix(first.Name, "inline handler") || first.Flow == nil || first.File == "" {
		t.Fatalf("literal handler identity/source/body lost: %+v", graph.Registrations)
	}
	if len(graph.Calls) != 0 {
		t.Fatal("registering an inline body does not execute it")
	}
}

func TestBoundChiConfigurationRecursionAndUnknownPrefixes(t *testing.T) {
	a := chiRouteFixture(t, `package main
import ("net/http"; "github.com/go-chi/chi/v5")
type Handler struct{}
func (*Handler) List(http.ResponseWriter,*http.Request) {}
func (h *Handler) Configure(r chi.Router) { r.Get("/",h.List); r.Route("/nested",h.Configure) }
func main() { h:=&Handler{}; r:=chi.NewRouter(); r.Route("/items",h.Configure) }
`)
	graph, err := a.Analyze(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Registrations) != 1 || graph.Registrations[0].Pattern != "/items/" || graph.Registrations[0].Handler == nil || len(graph.RegistrationWarnings) == 0 {
		t.Fatalf("bound callback/recursive configuration must preserve known facts and disclose incomplete expansion: %+v", graph)
	}
	local, err := a.Analyze(context.Background(), "Handler.Configure")
	if err != nil {
		t.Fatal(err)
	}
	if len(local.Registrations) != 1 || local.Registrations[0].PatternKnown || local.Registrations[0].DeclaredPattern != "/" || local.Registrations[0].Issue == "" {
		t.Fatal("a library function root must not invent its mount prefix")
	}
	root, err := a.resolveRoot("main")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := a.inspectRegistrations(ctx, root, make(map[*ssa.Function]analyzedFunction), &Graph{}); err != context.Canceled {
		t.Fatalf("registration scan ignored cancellation: %v", err)
	}
	scanner := &routeScanner{a: a, ctx: context.Background(), frames: maxRouteFrames}
	scanner.visit(root, nil, nil, root)
	if len(scanner.warnings) != 1 {
		t.Fatal("bounded analysis must disclose incomplete findings")
	}
}

func TestServeMuxTypeAliasesAndResolvedInterfaceMethods(t *testing.T) {
	dir := fixture(t, map[string]string{"main.go": `package main
import "net/http"
type Mux = http.ServeMux
type registrar interface { HandleFunc(string,func(http.ResponseWriter,*http.Request)) }
func list(http.ResponseWriter,*http.Request) {}
func configure(r registrar) { r.HandleFunc("GET /typed/{id}",list) }
func main() { var mux Mux; configure(&mux) }
`})
	graph, err := New(dir).Analyze(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Registrations) != 1 || graph.Registrations[0].Issue != "" || graph.Registrations[0].Handler == nil || graph.Registrations[0].Pattern != "/typed/{id}" {
		t.Fatalf("known concrete API identity must survive aliases and user-defined interfaces: %+v", graph.Registrations)
	}
}

func TestRegistrationDeduplicationUsesFullSourceIdentity(t *testing.T) {
	dir := fixture(t, map[string]string{
		"main.go": `package main
import "net/http"
func list(http.ResponseWriter,*http.Request) {}
func main() { mux:=http.NewServeMux(); alpha(mux); beta(mux); alpha(mux) }
`,
		"alpha.go": "package main\nimport \"net/http\"\nfunc alpha(m *http.ServeMux) { m.HandleFunc(\"GET /same\",list) }\n",
		"beta.go":  "package main\nimport \"net/http\"\nfunc beta(m *http.ServeMux) { m.HandleFunc(\"GET /same\",list) }\n",
	})
	graph, err := New(dir).Analyze(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Registrations) != 2 || graph.Registrations[0].Registrar.Identity() == graph.Registrations[1].Registrar.Identity() {
		t.Fatal("identical evidence should deduplicate, but same-line registrations in different declarations must remain distinct")
	}
}

func TestChiUnsupportedMountAndRepeatedPrefixProvenance(t *testing.T) {
	a := chiRouteFixture(t, `package main
import ("net/http";"github.com/go-chi/chi/v5")
func list(http.ResponseWriter,*http.Request) {}
func routes(r chi.Router) { r.Get("/",list) }
func main() {
 r:=chi.NewRouter()
 r.Route("/same",routes)
 r.Route("/same",routes)
 child:=chi.NewRouter()
 child.Get("/before",list)
 r.Mount("/v1",child)
 child.Get("/after",list)
}
`)
	graph, err := a.Analyze(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Registrations) != 3 || len(graph.RegistrationWarnings) == 0 {
		t.Fatalf("lost repeated/mounted facts or their uncertainty: %+v", graph)
	}
	for _, route := range graph.Registrations {
		if route.Pattern == "/same/" {
			if !route.PatternKnown || len(route.Provenance) != 2 {
				t.Fatal("deduplicated facts must preserve both prefix call-site origins")
			}
		} else if route.PatternKnown || route.Issue == "" || len(route.Provenance) == 0 {
			t.Fatal("unsupported Mount must not present child-local paths as known application paths")
		}
	}
}
