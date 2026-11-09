package viewer

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PranavJoshi2893/codebound/internal/calltree"
	"github.com/PranavJoshi2893/codebound/internal/calltree/diagram"
)

// Optional read-only integration check; ordinary regression tests use small,
// self-contained framework fixtures and do not depend on a sibling checkout.
func TestRebalancerHTTPProjection(t *testing.T) {
	dir := os.Getenv("CODEBOUND_REBALANCER_DIR")
	if dir == "" {
		t.Skip("set CODEBOUND_REBALANCER_DIR for the read-only reference-project check")
	}
	a := calltree.New(dir)
	a.Pattern = "./..."
	graph, err := a.Analyze(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"Handler.Create": "Service.CreateItem", "Handler.GetByID": "Service.GetItemByID", "Handler.List": "Service.ListItems"}
	found, dispatched := make(map[string]bool), make(map[string]bool)
	for _, call := range graph.Calls {
		if target, ok := want[call.Caller.Name]; ok && call.Callee.Name == target {
			if call.Path != nil || call.Site == "" {
				t.Fatalf("service invocation lost its actual call site: %+v", call)
			}
			found[call.Caller.Name] = true
		}
		if _, ok := want[call.Callee.Name]; ok {
			if call.Path == nil || !call.Path.HTTPDispatch {
				t.Fatalf("handler without real HTTP dispatch: %+v", call)
			}
			dispatched[call.Callee.Name] = true
		}
		if call.Caller.Name == "NewHandler" {
			t.Fatal("constructor treated as request handler")
		}
	}
	if len(found) != 3 || len(dispatched) != 3 {
		t.Fatalf("missing request paths: services=%v dispatch=%v", found, dispatched)
	}
	index := diagram.IndexGraph(graph)
	for _, contexts := range []bool{false, true} {
		image, err := renderViewSVG(context.Background(), index, diagram.BrowserOptions{Focus: -1, Contexts: contexts})
		if err != nil {
			t.Fatal(err)
		}
		for handler, service := range want {
			if !strings.Contains(string(image), handler) || !strings.Contains(string(image), service) {
				t.Fatalf("missing rendered path %s → %s", handler, service)
			}
		}
		if !strings.Contains(string(image), "possible HTTP dispatch") {
			t.Fatal("rendering discarded dispatch semantics")
		}
		if artifacts := os.Getenv("CODEBOUND_LAYOUT_ARTIFACTS"); artifacts != "" {
			name := "calls"
			if contexts {
				name = "contexts"
			}
			var dot bytes.Buffer
			if err := index.WriteBrowserDOT(&dot, diagram.BrowserOptions{Focus: -1, Contexts: contexts}); err != nil {
				t.Fatal(err)
			}
			for extension, data := range map[string][]byte{"svg": image, "dot": dot.Bytes()} {
				if err := os.WriteFile(filepath.Join(artifacts, "rebalancer-layout-"+name+"."+extension), data, 0644); err != nil {
					t.Fatal(err)
				}
			}
		}
		t.Logf("rebalancer: %d visible functions, %d calls; contexts=%v rendered successfully", len(index.Functions), len(graph.Calls), contexts)
	}
}
