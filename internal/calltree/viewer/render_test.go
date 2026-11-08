package viewer

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/PranavJoshi2893/codebound/internal/calltree"
)

func TestRenderSVG(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test stubs use Unix shell scripts")
	}
	tools := t.TempDir()
	t.Setenv("PATH", tools)
	path := filepath.Join(tools, "dot")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf '<svg>graph</svg>'\n"), 0755); err != nil {
		t.Fatal(err)
	}
	svg, err := RenderSVG(context.Background(), &calltree.Graph{})
	if err != nil || string(svg) != "<svg>graph</svg>" {
		t.Fatalf("unexpected render: %s, %v", svg, err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf 'invalid graph' >&2\nexit 1\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := RenderSVG(context.Background(), &calltree.Graph{}); err == nil || !strings.Contains(err.Error(), "invalid graph") {
		t.Fatalf("missing Graphviz diagnostic: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := RenderSVG(ctx, &calltree.Graph{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}

func TestLimitedBuffer(t *testing.T) {
	buffer := limitedBuffer{limit: 4}
	if n, err := buffer.Write([]byte("abcdef")); n != 4 || !errors.Is(err, errOutputLimit) || buffer.String() != "abcd" {
		t.Fatalf("output limit not enforced: n=%d err=%v content=%q", n, err, buffer.String())
	}
	if n, err := buffer.Write([]byte("more")); n != 0 || !errors.Is(err, errOutputLimit) {
		t.Fatalf("full buffer accepted more data: n=%d err=%v", n, err)
	}
}

func TestOutputLimitAppliesToStreamCopies(t *testing.T) {
	buffer := limitedBuffer{limit: 4}
	if _, err := io.Copy(&buffer, strings.NewReader("abcdef")); !errors.Is(err, errOutputLimit) {
		t.Fatalf("stream copy bypassed output limit: %v", err)
	}
	if buffer.String() != "abcd" {
		t.Fatalf("unexpected bounded output: %q", buffer.String())
	}
}

func TestRenderDeadline(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test stubs use Unix shell scripts")
	}
	tools := t.TempDir()
	t.Setenv("PATH", tools)
	if err := os.WriteFile(filepath.Join(tools, "dot"), []byte("#!/bin/sh\nwhile :; do :; done\n"), 0755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := RenderSVG(ctx, &calltree.Graph{}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("renderer did not honor deadline: %v", err)
	}
}

func TestMissingGraphviz(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := RenderSVG(context.Background(), &calltree.Graph{}); err == nil || !strings.Contains(err.Error(), "Graphviz") {
		t.Fatalf("expected actionable missing-Graphviz error, got %v", err)
	}
}

func TestNodeMapMetadata(t *testing.T) {
	output := []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg><map><area shape="rect" href="function:3" title="Run &amp; &lt;source&gt;" coords="5,10,100,60"/><area shape="poly" href="ignored" coords="1,2,3,4"/></map>`)
	svg, err := embedNodeMap(output)
	if err != nil {
		t.Fatal(err)
	}
	nodes := decodeNodeMetadata(t, svg)
	if len(nodes) != 1 || nodes[0].ID != 3 || nodes[0].Label != "Run & <source>" || nodes[0].Bounds != [4]int{5, 10, 100, 60} {
		t.Fatalf("incorrect node map: %+v", nodes)
	}
	for _, bad := range []string{
		`<svg>`,
		`<svg></svg><map><area shape="rect" href="function:-1" coords="1,2,3,4"/></map>`,
		`<svg></svg><map><area shape="rect" href="function:0" coords="1,2,3"/></map>`,
		`<svg></svg><map><area shape="rect" href="function:0" coords="1,no,3,4"/></map>`,
		`<svg></svg><map><area shape="rect" href="function:0" coords="3,2,1,4"/></map>`,
	} {
		if _, err := embedNodeMap([]byte(bad)); err == nil {
			t.Errorf("accepted malformed renderer output: %s", bad)
		}
	}
}

func decodeNodeMetadata(t *testing.T, svg []byte) []graphNode {
	t.Helper()
	var document struct {
		Metadata string `xml:"metadata"`
	}
	if err := xml.Unmarshal(svg, &document); err != nil {
		t.Fatal(err)
	}
	var nodes []graphNode
	if err := json.Unmarshal([]byte(document.Metadata), &nodes); err != nil {
		t.Fatal(err)
	}
	return nodes
}

func TestGraphvizSelectableNodes(t *testing.T) {
	if _, err := exec.LookPath("dot"); err != nil {
		t.Skip("Graphviz is not installed")
	}
	root := calltree.Function{Name: "main", Package: "example/app"}
	leaf := calltree.Function{Name: "<source>", Package: "example/worker"}
	graph := &calltree.Graph{Root: root, Calls: []calltree.Call{{Caller: root, Callee: leaf}}}
	svg, err := RenderSVG(context.Background(), graph)
	if err != nil {
		t.Fatal(err)
	}
	nodes := decodeNodeMetadata(t, svg)
	if len(nodes) != 2 || nodes[0].ID != 0 || nodes[1].ID != 1 || !strings.Contains(nodes[1].Label, "<source>") {
		t.Fatalf("Graphviz lost selectable function identities: %+v", nodes)
	}
}
