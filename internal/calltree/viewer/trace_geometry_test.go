package viewer

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"math"
	"os/exec"
	"strings"
	"testing"

	"github.com/PranavJoshi2893/codebound/internal/calltree/diagram"
	"github.com/PranavJoshi2893/codebound/internal/calltree/model"
)

func decodeTraces(t *testing.T, image []byte) graphTraces {
	t.Helper()
	var document struct {
		Metadata []struct {
			ID   string `xml:"id,attr"`
			Text string `xml:",chardata"`
		} `xml:"metadata"`
	}
	if err := xml.Unmarshal(image, &document); err != nil {
		t.Fatal(err)
	}
	for _, metadata := range document.Metadata {
		if metadata.ID == "codebound-traces" {
			var traces graphTraces
			if err := json.Unmarshal([]byte(metadata.Text), &traces); err != nil {
				t.Fatal(err)
			}
			return traces
		}
	}
	t.Fatal("missing trace metadata")
	return graphTraces{}
}

func TestGraphvizOriginsAttachToActualRows(t *testing.T) {
	if _, err := exec.LookPath("dot"); err != nil {
		t.Skip("Graphviz is not installed")
	}
	block := model.FlowBlock{ID: 0, Label: "entry", Exit: "return"}
	for i := 1; i <= 14; i++ {
		block.Steps = append(block.Steps, model.FlowStep{ID: fmt.Sprintf("s%d", i), Reference: fmt.Sprintf("B0.%d", i), Kind: "call", Label: fmt.Sprintf("work%d(<source>)", i)})
	}
	root := model.Function{Name: "main", Package: "example/app", Flow: &model.FunctionFlow{Scopes: []model.FlowScope{{ID: "body", Entry: 0, Blocks: []model.FlowBlock{block}}}}}
	leaf := model.Function{Name: "work", Package: "example/worker"}
	graph := &model.Graph{Root: root, Calls: []model.Call{
		{Caller: root, Callee: leaf, Site: "s2", Contexts: []model.ContextBinding{{Parameter: "ctx", IDs: []string{"C1"}}}},
		{Caller: root, Callee: leaf, Site: "s11", Execution: "goroutine", Contexts: []model.ContextBinding{{Parameter: "ctx", IDs: []string{"C2"}}}},
		{Caller: root, Callee: leaf, Site: "s2", Contexts: []model.ContextBinding{{Parameter: "ctx", IDs: []string{"C1"}}}},
		{Caller: root, Callee: leaf}, // Must never borrow either positioned origin.
	}}
	before, _ := json.Marshal(graph)
	index := diagram.IndexGraph(graph)
	for _, contexts := range []bool{false, true} {
		for _, focus := range []int{-1, 1} {
			image, err := renderViewSVG(context.Background(), index, diagram.BrowserOptions{Contexts: contexts, Focus: focus})
			if err != nil {
				t.Fatal(err)
			}
			traces := decodeTraces(t, image)
			if len(traces.Routes) != 3 || len(decodeNodeMetadata(t, image)) != 2 {
				t.Fatalf("distinct source rows plus one unknown route must preserve function identities: %+v", traces.Routes)
			}
			seen := make(map[string]bool)
			for _, route := range traces.Routes {
				if route.Path == "" || route.From != 0 || route.To != 1 || seen[route.Site] {
					t.Fatalf("incorrect/doubled route identity: %+v", route)
				}
				seen[route.Site] = true
				if route.Site == "" {
					continue
				}
				x := route.Origin[0]*traces.Transform[0] + traces.Transform[2]
				y := route.Origin[1]*traces.Transform[1] + traces.Transform[3]
				found := false
				for _, site := range traces.Sites {
					if site.From != route.From || site.Site != route.Site {
						continue
					}
					found = true
					if y < float64(site.Bounds[1])-1 || y > float64(site.Bounds[3])+1 || math.Abs(x-float64(site.Bounds[2])) > 16 {
						t.Fatalf("origin must visibly attach beside its actual row, not somewhere on the box: site=%+v origin=(%.1f,%.1f)", site, x, y)
					}
					if !strings.Contains(site.Label, "<source>") {
						t.Fatal("source names must survive as inert text")
					}
				}
				if !found {
					t.Fatalf("missing source row for %s", route.Site)
				}
			}
			if !seen["s2"] || !seen["s11"] || !seen[""] {
				t.Fatalf("missing positioned or unknown source: %v", seen)
			}
		}
	}
	after, _ := json.Marshal(graph)
	if string(before) != string(after) {
		t.Fatal("tracing must not change analysis or exported evidence")
	}
}

func TestTraceGeometryRejectsInvalidRendererData(t *testing.T) {
	valid := `<svg xmlns:xlink="http://www.w3.org/1999/xlink"><g class="graph" transform="scale(1 1) rotate(0) translate(4 100)"><g class="edge"><a xlink:href="call:0:1:s1"><path d="M0,0C1,2 3,4 5,6" stroke="#64748b"/><ellipse cx="0" cy="0" rx="2"/><polygon points="5,6 4,5 6,5"/></a></g></g></svg>`
	traces, err := readTraceGeometry([]byte(valid))
	if err != nil || len(traces.Routes) != 1 || traces.Routes[0].Site != "s1" || traces.Transform[0] != 4.0/3.0 {
		t.Fatalf("lost trace geometry: %+v %v", traces, err)
	}
	for _, invalid := range []string{
		strings.Replace(valid, "call:0:1:s1", "call:-1:1:s1", 1),
		strings.Replace(valid, "call:0:1:s1", "call:0", 1),
		strings.Replace(valid, "M0,0C1,2 3,4 5,6", "javascript:alert(1)", 1),
		strings.Replace(valid, "M0,0C1,2 3,4 5,6", "M0,0", 1),
		strings.Replace(valid, "call:0:1:s1", "registration:0:1:-1:s1", 1),
		strings.Replace(valid, "M0,0C1,2 3,4 5,6", "M0,0C1e309,2 3,4 5,6", 1),
		strings.Replace(valid, "#64748b", "url(https://example.com)", 1),
		strings.Replace(valid, `rx="2"`, `rx="NaN"`, 1),
		strings.Replace(valid, `rx="2"`, `rx="-1"`, 1),
		strings.Replace(valid, "5,6 4,5 6,5", "1,2,3", 1),
		strings.Replace(valid, "rotate(0)", "rotate(90)", 1),
		strings.Replace(valid, "translate(4 100)", "translate(NaN 100)", 1),
	} {
		if _, err := readTraceGeometry([]byte(invalid)); err == nil {
			t.Errorf("accepted invalid trace geometry: %s", invalid)
		}
	}
}
