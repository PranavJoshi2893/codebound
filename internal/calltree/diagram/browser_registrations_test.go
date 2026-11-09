package diagram

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/PranavJoshi2893/codebound/internal/calltree/model"
)

func registrationFixture() *model.Graph {
	root := model.Function{Name: "main", Package: "example/app"}
	configure := model.Function{Name: "ItemRoutes", Package: "example/router", Flow: &model.FunctionFlow{Scopes: []model.FlowScope{{ID: "body", Entry: 0, Blocks: []model.FlowBlock{{ID: 0, Exit: "return", Steps: []model.FlowStep{
		{ID: "s1", Reference: "B0.1", Kind: "call", Label: "r.Get(…)"},
		{ID: "s2", Reference: "B0.2", Kind: "call", Label: "r.Post(…)"},
		{ID: "s3", Reference: "B0.3", Kind: "call", Label: "r.Get(…)"},
	}}}}}}}
	server := model.Function{Name: "Server.Run", Package: "example/server"}
	list := model.Function{Name: "Handler.List", Package: "example/item"}
	create := model.Function{Name: "Handler.Create", Package: "example/item"}
	return &model.Graph{Root: root, Calls: []model.Call{{Caller: root, Callee: configure}, {Caller: root, Callee: server}, {Caller: server, Callee: list, Path: &model.CallPath{HTTPDispatch: true}}}, Registrations: []model.RouteRegistration{
		{Registrar: configure, Handler: &list, Framework: "chi", Mux: "router", Site: "s1", Location: "router.go:10", Method: "GET", MethodKnown: true, DeclaredPattern: "/", Pattern: "/items/", PatternKnown: true},
		{Registrar: configure, Handler: &create, Framework: "chi", Mux: "router", Site: "s2", Location: "router.go:11", Method: "POST", MethodKnown: true, DeclaredPattern: "/", Pattern: "/items/", PatternKnown: true},
		{Registrar: configure, Framework: "chi", Mux: "router", Site: "s3", Location: "router.go:12", Method: "GET", MethodKnown: true, DeclaredPattern: "/", Pattern: "/inventory/", PatternKnown: true, Issue: "nil handler"},
	}}
}

func TestBrowserRegistrationLayerAndFocus(t *testing.T) {
	graph := registrationFixture()
	before, _ := json.Marshal(graph)
	index := IndexGraph(graph)
	if len(index.Functions) != 5 || len(index.Calls) != 3 || len(index.Registrations) != 3 {
		t.Fatal("registrations must not become invocation edges or fake functions")
	}
	if index.RegistrationOnly(3) || !index.RegistrationOnly(4) || index.phase(3) != phaseHTTP || index.phase(4) != 0 {
		t.Fatal("registration-only handlers must not acquire invocation/lifecycle reachability")
	}
	for _, contexts := range []bool{false, true} {
		for _, focus := range []int{-1, 1, 3, 4} {
			var output bytes.Buffer
			if err := index.WriteBrowserDOT(&output, BrowserOptions{Focus: focus, Contexts: contexts}); err != nil {
				t.Fatal(err)
			}
			dot := output.String()
			for _, want := range []string{"registers", "arrowhead=odiamond", `tailport="s`, `URL="registration:`} {
				if !strings.Contains(dot, want) {
					t.Errorf("missing distinct registration presentation %s", want)
				}
			}
			if focus == -1 || focus == 1 {
				if !strings.Contains(dot, "registers GET /items/") || !strings.Contains(dot, "registers POST /items/") || !strings.Contains(dot, "nil handler") || strings.Count(dot, "arrowhead=odiamond") != 2 {
					t.Fatal("missing route wiring or invented a nil-handler destination")
				}
			}
			layout := renderDOTLayout(t, dot)
			checkLayoutTextSeparate(t, layout)
			checkLayoutRoutesAvoidText(t, layout)
		}
	}
	var hidden bytes.Buffer
	if err := index.WriteBrowserDOT(&hidden, BrowserOptions{Focus: -1, HideRegistrations: true}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(hidden.String(), "registers") || strings.Contains(hidden.String(), "Handler.Create") || !strings.Contains(hidden.String(), "possible HTTP dispatch") {
		t.Fatal("hiding wiring must retain dispatch calls and remove registration-only nodes")
	}
	var focused bytes.Buffer
	if err := index.WriteBrowserDOT(&focused, BrowserOptions{Focus: 1, HideRegistrations: true}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(focused.String(), "Handler.") {
		t.Fatal("disabled registrations must not leak their neighbors into call-only focus")
	}
	after, _ := json.Marshal(graph)
	if string(before) != string(after) {
		t.Fatal("browser registration rendering mutated source evidence")
	}
}

func TestRegistrationCyclesRespectInvocationBackbone(t *testing.T) {
	graph := registrationFixture()
	from, target := graph.Calls[0].Callee, graph.Root
	graph.Registrations = []model.RouteRegistration{
		{Registrar: from, Handler: &target, Framework: "net/http", Site: "s1", Method: "GET", MethodKnown: true, Pattern: "/feedback", PatternKnown: true},
		{Registrar: from, Handler: &from, Framework: "net/http", Site: "s2", Method: "POST", MethodKnown: true, Pattern: "/self", PatternKnown: true},
	}
	index := IndexGraph(graph)
	edges := index.browserEdges(nil, false)
	before := append([]browserEdge(nil), edges...)
	feedback := index.registrationFeedback(edges, []int{0, 1})
	if !feedback[0] || !feedback[1] {
		t.Fatal("registration feedback/self loops must not introduce cyclic ranks")
	}
	if !reflect.DeepEqual(before, edges) {
		t.Fatal("registration layout changed invocation backbone decisions")
	}
	var dot bytes.Buffer
	if err := index.WriteBrowserDOT(&dot, BrowserOptions{Focus: -1}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dot.String(), `tailport="s1_w:w"`) || !strings.Contains(dot.String(), "registers POST /self → this function") || strings.Contains(dot.String(), "n1 -> n1") {
		t.Fatal("feedback needs exterior ports; self-registration stays explicitly at its row without a crossing loop")
	}
	layout := renderDOTLayout(t, dot.String())
	checkLayoutTextSeparate(t, layout)
	checkLayoutRoutesAvoidText(t, layout)
}
