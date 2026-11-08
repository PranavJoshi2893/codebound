package viewer

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/PranavJoshi2893/codebound/internal/calltree"
)

func TestHandler(t *testing.T) {
	handler := Handler([]byte("<svg>graph</svg>"), nil)
	for _, test := range []struct {
		path, method, contentType, body string
		status                          int
	}{
		{"/", "GET", "text/html", "calltree.svg", 200},
		{"/assets/graph.js", "GET", "text/javascript", "function readNodes(", 200},
		{"/assets/camera.js", "GET", "text/javascript", "zoomAt(factor", 200},
		{"/assets/explorer.js", "GET", "text/javascript", "function selectFunction(", 200},
		{"/assets/viewer.css", "GET", "text/css", ".workspace", 200},
		{"/assets/explorer.js", "POST", "", "", 405},
		{"/assets/", "GET", "", "", 404},
		{"/calltree.svg", "GET", "image/svg+xml", "<svg>graph</svg>", 200},
		{"/calltree.svg?mode=calls", "GET", "image/svg+xml", "<svg>graph</svg>", 200},
		{"/calltree.svg?mode=other", "GET", "", "invalid graph mode", 400},
		{"/calltree.svg?focus=-1", "GET", "", "invalid graph focus", 400},
		{"/calltree.svg?focus=oops", "GET", "", "invalid graph focus", 400},
		{"/calltree.svg?focus=0", "GET", "", "invalid graph focus", 400},
		{"/calltree.svg?mode=contexts", "GET", "", "graph data unavailable", 503},
		{"/graph.json", "GET", "application/json", `"functions":[]`, 200},
		{"/graph.json", "POST", "", "", 405},
		{"/missing", "GET", "", "", 404},
		{"/calltree.svg", "POST", "", "", 405},
		{"/assets/graph.js", "POST", "", "", 405},
	} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(test.method, test.path, nil)
		request.Host = "127.0.0.1:3000"
		handler.ServeHTTP(response, request)
		if response.Code != test.status ||
			!strings.Contains(response.Header().Get("Content-Type"), test.contentType) ||
			!strings.Contains(response.Body.String(), test.body) {
			t.Errorf("%s %s: unexpected response %d %s", test.method, test.path, response.Code, response.Body.String())
		}
	}
}

func TestOnDemandGraphViews(t *testing.T) {
	root := calltree.Function{Name: "main", Package: "example/app"}
	leaf := calltree.Function{Name: "Leaf", Package: "example/worker"}
	graph := &calltree.Graph{Root: root, Calls: []calltree.Call{{Caller: root, Callee: leaf}}}
	handler := Handler([]byte("<svg>initial</svg>"), graph)
	for _, path := range []string{"/calltree.svg?mode=contexts&focus=1", "/calltree.svg?mode=contexts&focus=1", "/calltree.svg?focus=0"} {
		request := httptest.NewRequest("GET", path, nil)
		request.Host = "127.0.0.1:3000"
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		// Reuse the real Graphviz integration when available.
		if response.Code == 500 {
			if err := CheckGraphviz(); err != nil {
				t.Skip("Graphviz is not installed")
			}
			t.Fatalf("could not render %s: %s", path, response.Body.String())
		}
		if response.Code != 200 || len(decodeNodeMetadata(t, response.Body.Bytes())) != 2 {
			t.Fatalf("lost graph nodes for %s: %s", path, response.Body.String())
		}
	}
}

func TestViewerRejectsCrossSiteAccess(t *testing.T) {
	for _, test := range []struct {
		host, origin, fetchSite string
		status                  int
	}{
		{"evil.example:3000", "", "", 403},
		{"localhost.evil.example", "", "", 403},
		{"127.0.0.1:3000", "https://evil.example", "", 403},
		{"127.0.0.1:3000", "", "cross-site", 403},
		{"127.0.0.1:3000", "null", "", 403},
		{"127.0.0.1:3000", "http://127.0.0.1:9999", "", 403},
		{"127.0.0.1:3000", "http://127.0.0.1:3000", "same-origin", 200},
		{"localhost:3000", "", "", 200},
		{"[::1]:3000", "", "", 200},
	} {
		for _, path := range []string{"/", "/assets/graph.js", "/assets/camera.js", "/assets/explorer.js", "/assets/viewer.css", "/calltree.svg", "/graph.json"} {
			request := httptest.NewRequest("GET", path, nil)
			request.Host = test.host
			request.Header.Set("Origin", test.origin)
			request.Header.Set("Sec-Fetch-Site", test.fetchSite)
			response := httptest.NewRecorder()
			Handler([]byte("<svg/>"), nil).ServeHTTP(response, request)
			if response.Code != test.status {
				t.Errorf("%s %+v: got status %d", path, test, response.Code)
			}
			if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Content-Security-Policy") == "" {
				t.Fatal("missing viewer security headers")
			}
		}
	}
}

func TestViewerAllowsOnlySameOriginScripts(t *testing.T) {
	response := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/", nil)
	request.Host = "127.0.0.1:3000"
	Handler([]byte("<svg/>"), nil).ServeHTTP(response, request)
	var scriptPolicy string
	for _, directive := range strings.Split(response.Header().Get("Content-Security-Policy"), ";") {
		if strings.HasPrefix(strings.TrimSpace(directive), "script-src ") {
			scriptPolicy = strings.TrimSpace(directive)
		}
	}
	if scriptPolicy != "script-src 'self'" {
		t.Fatalf("scripts must be same-origin only, without inline or eval permissions: %s", scriptPolicy)
	}
	for _, expected := range []string{`<script src="assets/graph.js" defer></script>`, `<script src="assets/camera.js" defer></script>`, `<script src="assets/explorer.js" defer></script>`, `<link rel="stylesheet" href="assets/viewer.css">`, `<img id="graph"`, `id="zoom-in"`, `id="zoom-out"`, `id="fit"`, `tabindex="0"`, `id="reading-guide" open`, `id="function-search"`, `id="function-details"`} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Errorf("missing viewer markup %q", expected)
		}
	}
	if strings.Contains(response.Body.String(), "<object") || strings.Contains(response.Body.String(), "<iframe") || strings.Contains(response.Body.String(), "<svg") {
		t.Fatal("graph must remain an isolated image, not an active SVG document")
	}
}

func TestGraphDetails(t *testing.T) {
	root := calltree.Function{Name: "main", Package: "example", File: "main.go", Line: 1}
	worker := calltree.Function{Name: "Run<script>", Package: "example/worker", File: "worker.go", Line: 5,
		Context: &calltree.ContextInfo{Bindings: []calltree.ContextBinding{{Parameter: "ctx", IDs: []string{"C1"}, Unresolved: true}}}}
	graph := &calltree.Graph{Root: root, Calls: []calltree.Call{
		{Caller: root, Callee: worker, ThroughInterface: true, Execution: "goroutine", Violation: "layer warning",
			Contexts: []calltree.ContextBinding{{Parameter: "ctx", IDs: []string{"C1"}, Unresolved: true}}},
		{Caller: worker, Callee: worker},
		{Caller: root, Callee: worker},
	}}
	response := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/graph.json", nil)
	request.Host = "127.0.0.1:3000"
	Handler([]byte("<svg/>"), graph).ServeHTTP(response, request)
	var data graphData
	if err := json.Unmarshal(response.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Functions) != 2 || len(data.Calls) != 3 || data.Functions[0].Name != root.Name || data.Functions[1].Name != worker.Name {
		t.Fatalf("lost function identities or calls: %+v", data)
	}
	call := data.Calls[0]
	if call.From != 0 || call.To != 1 || !call.ThroughInterface || call.Execution != "goroutine" || call.Violation != "layer warning" || !call.Contexts[0].Unresolved {
		t.Fatalf("lost call evidence: %+v", call)
	}
	if data.Calls[1].From != 1 || data.Calls[1].To != 1 || data.Functions[1].Context == nil {
		t.Fatal("lost recursion or context details")
	}
}

func TestServeStopsOnCancellation(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- Serve(ctx, listener, []byte("<svg/>"), nil) }()
	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Get("http://" + listener.Addr().String() + "/")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	cancel()
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not stop on cancellation")
	}
}
