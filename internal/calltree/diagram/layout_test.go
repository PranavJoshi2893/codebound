package diagram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/PranavJoshi2893/codebound/internal/calltree/model"
)

// Check actual Graphviz text geometry, not just the exported DOT attributes.
// The dense fixture reproduces overlapping labels on parallel context arrows.
func TestContextLayoutKeepsTextSeparate(t *testing.T) {
	layout := renderContextLayout(t, contextLayoutFixture())
	checkLayoutTextSeparate(t, layout)
}

func TestBrowserLayoutKeepsTextSeparate(t *testing.T) {
	graph := contextLayoutFixture()
	for _, contexts := range []bool{false, true} {
		for _, focus := range []int{-1, 1, 2} {
			var dot bytes.Buffer
			if err := IndexGraph(graph).WriteBrowserDOT(&dot, BrowserOptions{Contexts: contexts, Focus: focus}); err != nil {
				t.Fatal(err)
			}
			checkLayoutTextSeparate(t, renderDOTLayout(t, dot.String()))
		}
	}
}

func TestBrowserContextRoutesBetweenPackages(t *testing.T) {
	root := Function{Name: "main", Package: "example/cmd/api", Context: &ContextInfo{Operations: []ContextOperation{
		{Kind: "create", Target: "context.Background", IDs: []string{"C1"}},
	}}}
	left := Function{Name: "Start", Package: "example/server"}
	right := Function{Name: "Serve", Package: left.Package, Context: &ContextInfo{Operations: []ContextOperation{
		{Kind: "derive", Target: "context.WithTimeout", IDs: []string{"C2"}, ParentIDs: []string{"C1"}},
	}}}
	read := Function{Name: "Read", Package: "example/repo"}
	write := Function{Name: "Write", Package: read.Package}
	sink := Function{Name: "Store", Package: "example/database"}
	call := func(from, to Function, id string) Call {
		return Call{Caller: from, Callee: to, Contexts: []ContextBinding{{Parameter: "ctx", IDs: []string{id}}}}
	}
	graph := &Graph{Root: root, Calls: []Call{
		call(root, left, "C1"), call(root, right, "C1"),
		call(left, write, "C1"), call(right, read, "C2"),
		call(read, sink, "C2"), call(write, sink, "C1"),
		call(left, write, "C1"), // Same relationship, not another route.
	}}
	var dot bytes.Buffer
	if err := IndexGraph(graph).WriteBrowserDOT(&dot, BrowserOptions{Contexts: true, Focus: -1}); err != nil {
		t.Fatal(err)
	}
	layout := renderDOTLayout(t, dot.String())
	checkLayoutTextSeparate(t, layout)
	if len(layout.Edges) != 6 {
		t.Fatalf("expected six function-to-function routes and no context branches, got %d", len(layout.Edges))
	}
	for i, first := range layout.Edges {
		for _, second := range layout.Edges[i+1:] {
			if first.Tail == second.Tail || first.Tail == second.Head || first.Head == second.Tail || first.Head == second.Head {
				continue // Shared endpoints are junctions, not crossings.
			}
			if layoutPathsCross(first, second) {
				t.Errorf("separable package flow has crossing routes: %d→%d and %d→%d", first.Tail, first.Head, second.Tail, second.Head)
			}
		}
	}
}

func TestBrowserDenseRoutesStayOutsideExecutionText(t *testing.T) {
	graph := browserRoutingFixture()
	var dot bytes.Buffer
	if err := IndexGraph(graph).WriteBrowserDOT(&dot, BrowserOptions{Contexts: true, Focus: -1}); err != nil {
		t.Fatal(err)
	}
	layout := renderDOTLayout(t, dot.String())
	checkLayoutTextSeparate(t, layout)
	local := 0
	for _, edge := range IndexGraph(graph).browserEdges(nil, false) {
		if edge.localRecursion() {
			local++
		}
	}
	if len(layout.Edges)+local != len(graph.Calls) || !strings.Contains(dot.String(), "↻ recursive call") {
		t.Fatal("routing must not hide cyclic, recursive, or cross-package relationships")
	}
	checkLayoutRoutesAvoidText(t, layout)
}

func checkLayoutRoutesAvoidText(t *testing.T, layout contextLayout) {
	t.Helper()
	drawings := append(layout.Objects, layout.Edges...)
	for _, edge := range layout.Edges {
		for _, node := range drawings {
			for _, text := range node.textBounds() {
				for _, segment := range layoutLineSegments(edge) {
					if layoutLineHitsText(segment, text) {
						t.Errorf("route %d→%d cuts through graph text %q (drawing %d→%d)", edge.Tail, edge.Head, text.text, node.Tail, node.Head)
					}
				}
			}
		}
	}
}

// Tall cards with fan-out, shared targets, feedback, and recursion reproduce
// the routing failure that a small three-node call tree does not expose.
func browserRoutingFixture() *Graph {
	var functions []Function
	for i, name := range []string{"main", "Server.Start", "Server.Run", "Repository.Get", "Repository.Put", "Store"} {
		pkg := []string{"example/cmd/api", "example/server", "example/server", "example/repository", "example/repository", "example/database"}[i]
		fn := Function{Name: name, Package: pkg, Flow: &model.FunctionFlow{Scopes: []model.FlowScope{{ID: "body", Entry: 0, Blocks: []model.FlowBlock{{ID: 0, Label: "entry", Exit: "return"}}}}}}
		for n := 1; n <= 4; n++ {
			fn.Flow.Scopes[0].Blocks[0].Steps = append(fn.Flow.Scopes[0].Blocks[0].Steps, model.FlowStep{ID: fmt.Sprintf("q0b0s%d", n), Reference: fmt.Sprintf("B0.%d", n), Kind: "call", Label: fmt.Sprintf("operation%d(…)", n)})
		}
		functions = append(functions, fn)
	}
	graph := &Graph{Root: functions[0]}
	for _, edge := range [][3]int{{0, 1, 1}, {0, 2, 3}, {0, 3, 2}, {1, 2, 1}, {1, 3, 2}, {1, 4, 4}, {2, 1, 3}, {2, 3, 1}, {2, 4, 2}, {3, 4, 2}, {4, 3, 1}, {3, 5, 4}, {4, 5, 3}, {1, 1, 3}} {
		from, to, site := edge[0], edge[1], edge[2]
		functions[from].Flow.Scopes[0].Blocks[0].Steps[site-1].Label = functions[to].Name + "(…)"
		graph.Calls = append(graph.Calls, Call{Caller: functions[from], Callee: functions[to], Site: fmt.Sprintf("q0b0s%d", site), Contexts: []ContextBinding{{Parameter: "ctx", IDs: []string{"C1"}}}})
	}
	return graph
}

func checkLayoutTextSeparate(t *testing.T, layout contextLayout) {
	t.Helper()
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
	Bounds  string          `json:"bb"`
	Objects []layoutDrawing `json:"objects"`
	Edges   []layoutDrawing `json:"edges"`
}

func renderContextLayout(t *testing.T, graph *Graph) contextLayout {
	t.Helper()
	return renderDOTLayout(t, contextDOTOutput(t, graph))
}

func renderDOTLayout(t *testing.T, source string) contextLayout {
	t.Helper()
	dot, err := exec.LookPath("dot")
	if err != nil {
		t.Skip("Graphviz is required to check rendered label placement")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, dot, "-Tjson")
	command.Stdin = strings.NewReader(source)
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
	Name   string `json:"name"`
	Width  string `json:"width"`
	Height string `json:"height"`
	Tail   int    `json:"tail"`
	Head   int    `json:"head"`
	Path   []struct {
		Op     string       `json:"op"`
		Points [][2]float64 `json:"points"`
	} `json:"_draw_"`
	Label         []layoutTextOperation `json:"_ldraw_"`
	ExternalLabel []layoutTextOperation `json:"_xldraw_"`
}

type layoutTextOperation struct {
	Op    string    `json:"op"`
	Point []float64 `json:"pt"`
	Align string    `json:"align"`
	Width float64   `json:"width"`
	Size  float64   `json:"size"`
	Text  string    `json:"text"`
}

// Sample cubic paths to check the rendered routes rather than just DOT options.
// This is a regression fixture, not a claim that arbitrary graphs are planar.
func layoutPathsCross(first, second layoutDrawing) bool {
	cross := func(a, b, c [2]float64) float64 {
		return (b[0]-a[0])*(c[1]-a[1]) - (b[1]-a[1])*(c[0]-a[0])
	}
	firstSegments, secondSegments := layoutLineSegments(first), layoutLineSegments(second)
	for _, a := range firstSegments {
		for _, b := range secondSegments {
			if cross(a[0], a[1], b[0])*cross(a[0], a[1], b[1]) < -1e-6 &&
				cross(b[0], b[1], a[0])*cross(b[0], b[1], a[1]) < -1e-6 {
				return true
			}
		}
	}
	return false
}

func layoutLineSegments(drawing layoutDrawing) [][2][2]float64 {
	var lines [][2][2]float64
	for _, path := range drawing.Path {
		if path.Op == "L" {
			for i := 1; i < len(path.Points); i++ {
				lines = append(lines, [2][2]float64{path.Points[i-1], path.Points[i]})
			}
		}
		if path.Op != "b" {
			continue
		}
		for i := 0; i+3 < len(path.Points); i += 3 {
			p := path.Points[i : i+4]
			previous := p[0]
			for step := 1; step <= 24; step++ {
				t := float64(step) / 24
				u := 1 - t
				var point [2]float64
				for axis := range point {
					point[axis] = u*u*u*p[0][axis] + 3*u*u*t*p[1][axis] + 3*u*t*t*p[2][axis] + t*t*t*p[3][axis]
				}
				lines = append(lines, [2][2]float64{previous, point})
				previous = point
			}
		}
	}
	return lines
}

func layoutLineHitsText(line [2][2]float64, text layoutTextBounds) bool {
	low, high := 0.0, 1.0
	minimum := [2]float64{text.left + 0.5, text.bottom + 0.5}
	maximum := [2]float64{text.right - 0.5, text.top - 0.5}
	for axis := range minimum {
		delta := line[1][axis] - line[0][axis]
		if math.Abs(delta) < 1e-9 {
			if line[0][axis] <= minimum[axis] || line[0][axis] >= maximum[axis] {
				return false
			}
			continue
		}
		first := (minimum[axis] - line[0][axis]) / delta
		last := (maximum[axis] - line[0][axis]) / delta
		low = math.Max(low, math.Min(first, last))
		high = math.Min(high, math.Max(first, last))
		if low >= high {
			return false
		}
	}
	return low < high
}

type layoutTextBounds struct {
	text                     string
	left, right, bottom, top float64
}

func (drawing layoutDrawing) textBounds() []layoutTextBounds {
	var labels []layoutTextBounds
	fontSize := 14.0
	for _, operation := range append(drawing.Label, drawing.ExternalLabel...) {
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
