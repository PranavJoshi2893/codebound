package viewer

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/PranavJoshi2893/codebound/internal/calltree"
	"github.com/PranavJoshi2893/codebound/internal/calltree/model"
)

//go:embed testdata/viewer_test.js
var browserTests []byte

// Run explicitly with CODEBOUND_BROWSER_TESTS=1. Firefox is only a test tool;
// the viewer itself has no browser automation or JavaScript package dependency.
func TestBrowserGraphNavigation(t *testing.T) {
	if os.Getenv("CODEBOUND_BROWSER_TESTS") != "1" {
		t.Skip("set CODEBOUND_BROWSER_TESTS=1 to run headless Firefox interaction tests")
	}
	firefox, err := exec.LookPath("firefox")
	if err != nil {
		t.Fatal("Firefox is required for browser interaction tests")
	}
	type browserReport struct {
		Passed []string `json:"passed"`
		Error  string   `json:"error"`
	}
	reports := make(chan browserReport, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		page := strings.Replace(string(indexHTML), "</head>", `<script src="viewer_test.js" defer></script></head>`, 1)
		_, _ = writer.Write([]byte(page))
	})
	mux.HandleFunc("GET /viewer_test.js", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		_, _ = writer.Write(browserTests)
	})
	mux.HandleFunc("GET /test-report", func(writer http.ResponseWriter, request *http.Request) {
		var report browserReport
		if err := json.Unmarshal([]byte(request.URL.Query().Get("result")), &report); err != nil {
			report.Error = "invalid browser test report: " + err.Error()
		}
		select {
		case reports <- report:
		default:
		}
		writer.WriteHeader(http.StatusNoContent)
	})
	root := calltree.Function{Name: "main", Package: "example", File: "main.go", Line: 1}
	root.Flow = &calltree.FunctionFlow{Scopes: []calltree.FlowScope{{ID: "body", Entry: 0, Blocks: []calltree.FlowBlock{{
		ID: 0, Label: "entry", Exit: "return", Steps: []calltree.FlowStep{
			{ID: "q0b0s1", Reference: "B0.1", Kind: "call", Label: "context.Background()"},
			{ID: "q0b0s2", Reference: "B0.2", Kind: "go", Label: "Worker.Run(…)"},
		},
	}}}}}
	root.Context = &calltree.ContextInfo{Operations: []calltree.ContextOperation{{Site: "q0b0s1", Kind: "create", Target: "context.Background", IDs: []string{"C1"}}}}
	worker := calltree.Function{Name: "Worker.Run", Package: root.Package, File: "worker.go", Line: 5,
		Context: &calltree.ContextInfo{
			Parameters: []string{"ctx"},
			Bindings:   []calltree.ContextBinding{{Parameter: "ctx", IDs: []string{"C1", "I1"}, Unresolved: true}},
			Operations: []calltree.ContextOperation{
				{Kind: "observe", Target: "context.Context.Done", Context: "ctx", IDs: []string{"C1", "I1"}, Unresolved: true, Location: "worker.go:6"},
				{Kind: "detach", Target: "context.WithoutCancel", Context: "detached", IDs: []string{"C2"}, ParentIDs: []string{"C1", "I1"}, ParentUnresolved: true, Location: "worker.go:7"},
			},
		}}
	leaf := calltree.Function{Name: "<script>not markup</script>", Package: "example/repo", File: "repo.go", Line: 10}
	for index := range 60 {
		worker.Context.Operations = append(worker.Context.Operations, calltree.ContextOperation{
			Kind: "read", Target: "context.Context.Err", Context: "ctx", IDs: []string{"C1"},
			Location: fmt.Sprintf("worker.go:%d", index+10),
		})
	}
	graph := &calltree.Graph{Root: root, Calls: []calltree.Call{
		{Caller: root, Callee: worker, Site: "q0b0s2", ThroughInterface: true, Execution: "goroutine", Contexts: []calltree.ContextBinding{{Parameter: "ctx", IDs: []string{"C1"}}}},
		{Caller: worker, Callee: leaf, Path: &calltree.CallPath{Entry: 0, Exit: 1, HTTPDispatch: true}, Violation: "example architecture warning"},
		{Caller: worker, Callee: worker, Execution: "deferred"},
	}}
	graph.Evidence = &model.CallEvidence{
		Nodes: []model.EvidenceFunction{{ID: "example.Worker.Run", Visible: true}, {ID: "framework.dispatch"}, {ID: "example/repo.leaf", Visible: true}},
		Edges: []model.EvidenceCall{
			{From: 0, To: 1, Location: "worker.go:8", Instruction: "b0i1", HTTPDispatch: true},
			{From: 1, To: 2, Location: "framework.go:12", Instruction: "b0i2"},
		},
	}
	graph.Calls = append(graph.Calls, graph.Calls[0]) // Repeated evidence is shown once in the UI.
	svg, err := RenderSVG(context.Background(), graph)
	if err != nil {
		t.Fatal(err)
	}
	mux.Handle("/", Handler(svg, graph))
	// Exercise the real page, script, and image under the production CSP.
	server := httptest.NewServer(protectLocalViewer(mux))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, firefox, "--headless", "--no-remote", "--profile", t.TempDir(), server.URL)
	command.WaitDelay = 5 * time.Second
	var logs bytes.Buffer
	command.Stdout = &logs
	command.Stderr = &logs
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	var browserErr error
	go func() {
		browserErr = command.Wait()
		close(done)
	}()
	defer func() {
		cancel()
		<-done
	}()

	select {
	case report := <-reports:
		if report.Error != "" {
			t.Fatalf("browser tests failed after %v: %s", report.Passed, report.Error)
		}
		if len(report.Passed) == 0 {
			t.Fatal("browser did not execute any interaction tests")
		}
		t.Logf("browser checks passed: %s", strings.Join(report.Passed, ", "))
	case <-done:
		t.Fatalf("browser exited before reporting: %v\n%s", browserErr, logs.String())
	case <-ctx.Done():
		cancel()
		<-done
		t.Fatalf("browser tests timed out: %s", logs.String())
	}
}
