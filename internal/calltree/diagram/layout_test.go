package diagram

import (
	"context"
	"encoding/json"
	"math"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Check actual Graphviz text geometry, not just the exported DOT attributes.
// The dense fixture reproduces overlapping labels on parallel context arrows.
func TestContextLayoutKeepsTextSeparate(t *testing.T) {
	layout := renderContextLayout(t, contextLayoutFixture())
	var labels []layoutTextBounds
	for _, drawing := range append(layout.Objects, layout.Edges...) {
		labels = append(labels, drawing.textBounds()...)
	}
	if len(labels) == 0 {
		t.Fatal("Graphviz did not return text geometry")
	}
	for index, first := range labels {
		for _, second := range labels[index+1:] {
			xOverlap := math.Min(first.right, second.right) - math.Max(first.left, second.left)
			yOverlap := math.Min(first.top, second.top) - math.Max(first.bottom, second.bottom)
			if xOverlap > 0.5 && yOverlap > 0.5 {
				t.Errorf("rendered text overlaps: %q and %q", first.text, second.text)
			}
		}
	}
}

func TestContextLayoutUsesPolygonalConnectors(t *testing.T) {
	layout := renderContextLayout(t, contextLayoutFixture())
	segments := 0
	for _, edge := range layout.Edges {
		for _, path := range edge.Path {
			if path.Op == "L" {
				segments += len(path.Points) - 1
				continue
			}
			if path.Op != "b" {
				continue
			}
			// Graphviz also encodes straight segments as cubic Beziers. Their
			// control points must lie on the segment, apart from rounding.
			for index := 0; index+3 < len(path.Points); index += 3 {
				start, end := path.Points[index], path.Points[index+3]
				dx, dy := end[0]-start[0], end[1]-start[1]
				length := math.Hypot(dx, dy)
				if length == 0 {
					continue
				}
				segments++
				for _, control := range path.Points[index+1 : index+3] {
					deviation := math.Abs((control[0]-start[0])*dy-(control[1]-start[1])*dx) / length
					if deviation > 0.1 {
						t.Errorf("connector contains a curved segment: control point is %.2f points off the line", deviation)
					}
				}
			}
		}
	}
	if segments == 0 {
		t.Fatal("Graphviz did not return connector geometry")
	}
}

type contextLayout struct {
	Objects []layoutDrawing `json:"objects"`
	Edges   []layoutDrawing `json:"edges"`
}

func renderContextLayout(t *testing.T, graph *Graph) contextLayout {
	t.Helper()
	dot, err := exec.LookPath("dot")
	if err != nil {
		t.Skip("Graphviz is required to check rendered label placement")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, dot, "-Tjson")
	command.Stdin = strings.NewReader(contextDOTOutput(t, graph))
	encoded, err := command.Output()
	if err != nil {
		if failure, ok := err.(*exec.ExitError); ok {
			if strings.Contains(string(failure.Stderr), `Format: "json" not recognized`) {
				t.Skip("Graphviz JSON output is required to check rendered text geometry")
			}
			t.Fatalf("render context layout: %v\n%s", err, failure.Stderr)
		}
		t.Fatalf("render context layout: %v", err)
	}
	var layout contextLayout
	if err := json.Unmarshal(encoded, &layout); err != nil {
		t.Fatal(err)
	}
	return layout
}

type layoutDrawing struct {
	Path []struct {
		Op     string       `json:"op"`
		Points [][2]float64 `json:"points"`
	} `json:"_draw_"`
	Label []struct {
		Op    string    `json:"op"`
		Point []float64 `json:"pt"`
		Align string    `json:"align"`
		Width float64   `json:"width"`
		Size  float64   `json:"size"`
		Text  string    `json:"text"`
	} `json:"_ldraw_"`
}

type layoutTextBounds struct {
	text                     string
	left, right, bottom, top float64
}

func (drawing layoutDrawing) textBounds() []layoutTextBounds {
	var labels []layoutTextBounds
	fontSize := 14.0
	for _, operation := range drawing.Label {
		if operation.Op == "F" {
			fontSize = operation.Size
		}
		if operation.Op != "T" || len(operation.Point) != 2 {
			continue
		}
		left := operation.Point[0]
		switch operation.Align {
		case "c":
			left -= operation.Width / 2
		case "r":
			left -= operation.Width
		}
		// Text positions are baselines. Use the font's em height to include
		// ascenders and descenders, with a half-point comparison tolerance.
		labels = append(labels, layoutTextBounds{
			text: operation.Text, left: left, right: left + operation.Width,
			bottom: operation.Point[1] - fontSize*0.2, top: operation.Point[1] + fontSize*0.8,
		})
	}
	return labels
}

func contextLayoutFixture() *Graph {
	main := Function{Name: "main", Package: "example.com/layout", File: "main.go", Line: 1, Context: &ContextInfo{
		Operations: []ContextOperation{
			{Kind: "create", Context: "background", Target: "context.Background", Location: "main.go:2:1"},
			{Kind: "derive", Context: "signal", Parent: "background", Target: "signal.NotifyContext", Location: "main.go:3:1"},
			{Kind: "cancel", Context: "signal", Target: "cancel", Execution: "deferred", Location: "main.go:4:1"},
			{Kind: "forward", Context: "signal", Target: "example.com/layout.run", Location: "main.go:5:1"},
		},
	}}
	run := Function{Name: "run", Package: "example.com/layout", File: "main.go", Line: 10, Context: &ContextInfo{
		Parameters: []string{"ctx"},
		Operations: []ContextOperation{
			{Kind: "derive", Context: "child", Parent: "parameter ctx", Target: "context.WithTimeout", Detail: "timeout: 5s", Location: "main.go:11:1"},
			{Kind: "cancel", Context: "child", Target: "cancel", Execution: "deferred", Location: "main.go:12:1"},
			{Kind: "forward", Context: "child", Target: "example.com/layout.consume", Execution: "goroutine", Location: "main.go:13:1"},
			{Kind: "read", Context: "parameter ctx", Target: "context.Context.Done", Location: "main.go:14:1"},
			{Kind: "observe", Context: "parameter ctx", Target: "context.Context.Done", Location: "main.go:14:1"},
			{Kind: "read", Context: "parameter ctx", Target: "context.Context.Err", Location: "main.go:15:1"},
			{Kind: "read", Context: "parameter ctx", Target: "context.Context.Deadline", Location: "main.go:16:1"},
			{Kind: "create", Context: "shutdown background", Target: "context.Background", Location: "main.go:17:1"},
			{Kind: "derive", Context: "shutdown", Parent: "shutdown background", Target: "context.WithTimeout", Detail: "timeout: 1s", Location: "main.go:18:1"},
			{Kind: "cancel", Context: "shutdown", Target: "cancel", Execution: "deferred", Location: "main.go:19:1"},
			{Kind: "forward", Context: "shutdown", Target: "example.com/layout.consume", Location: "main.go:20:1"},
		},
	}}
	consume := Function{Name: "consume", Package: "example.com/layout", File: "main.go", Line: 25, Context: &ContextInfo{
		Parameters: []string{"ctx"},
		Operations: []ContextOperation{{Kind: "read", Context: "parameter ctx", Target: "context.Context.Err", Location: "main.go:26:1"}},
	}}
	ids := map[string][]string{
		"background": {"C1"}, "signal": {"C2"}, "parameter ctx": {"C2"},
		"child": {"C3"}, "shutdown background": {"C4"}, "shutdown": {"C5"},
	}
	for _, function := range []Function{main, run, consume} {
		for index := range function.Context.Operations {
			operation := &function.Context.Operations[index]
			operation.IDs = ids[operation.Context]
			operation.ParentIDs = ids[operation.Parent]
		}
	}
	run.Context.Bindings = []ContextBinding{{Parameter: "ctx", IDs: []string{"C2"}}}
	consume.Context.Bindings = []ContextBinding{{Parameter: "ctx", IDs: []string{"C3", "C5"}}}
	consume.Context.Operations[0].IDs = []string{"C3", "C5"}
	return &Graph{Root: main, Calls: []Call{
		{Caller: main, Callee: run, Contexts: []ContextBinding{{Parameter: "ctx", IDs: []string{"C2"}}}},
		{Caller: run, Callee: consume, Contexts: []ContextBinding{{Parameter: "ctx", IDs: []string{"C3"}}}, Execution: "goroutine"},
		{Caller: run, Callee: consume, Contexts: []ContextBinding{{Parameter: "ctx", IDs: []string{"C5"}}}},
	}}
}
