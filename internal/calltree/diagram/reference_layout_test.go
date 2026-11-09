package diagram

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/PranavJoshi2893/codebound/internal/calltree"
)

func TestRebalancerLayoutGeometry(t *testing.T) {
	dir := os.Getenv("CODEBOUND_REBALANCER_DIR")
	if dir == "" {
		t.Skip("set CODEBOUND_REBALANCER_DIR for the read-only reference layout check")
	}
	a := calltree.New(dir)
	a.Pattern = "./..."
	graph, err := a.Analyze(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	index := IndexGraph(graph)
	for _, contexts := range []bool{false, true} {
		for focus := -1; focus < len(index.Functions); focus++ {
			t.Run(fmt.Sprintf("contexts=%v/focus=%d", contexts, focus), func(t *testing.T) {
				var dot bytes.Buffer
				if err := index.WriteBrowserDOT(&dot, BrowserOptions{Focus: focus, Contexts: contexts}); err != nil {
					t.Fatal(err)
				}
				layout := renderDOTLayout(t, dot.String())
				checkLayoutTextSeparate(t, layout)
				checkLayoutRoutesAvoidText(t, layout)
				if focus == -1 {
					width, height := layoutExtent(t, layout)
					t.Logf("rebalancer layout: %.0f × %.0f points; contexts=%v", width, height, contexts)
				}
			})
		}
	}
}
