package viewer

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHandler(t *testing.T) {
	handler := Handler([]byte("<svg>graph</svg>"))
	for _, test := range []struct {
		path, method, contentType, body string
		status                          int
	}{
		{"/", "GET", "text/html", "calltree.svg", 200},
		{"/viewer.js", "GET", "text/javascript", "function zoomAt(", 200},
		{"/calltree.svg", "GET", "image/svg+xml", "<svg>graph</svg>", 200},
		{"/missing", "GET", "", "", 404},
		{"/calltree.svg", "POST", "", "", 405},
		{"/viewer.js", "POST", "", "", 405},
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
		for _, path := range []string{"/", "/viewer.js", "/calltree.svg"} {
			request := httptest.NewRequest("GET", path, nil)
			request.Host = test.host
			request.Header.Set("Origin", test.origin)
			request.Header.Set("Sec-Fetch-Site", test.fetchSite)
			response := httptest.NewRecorder()
			Handler([]byte("<svg/>")).ServeHTTP(response, request)
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
	Handler([]byte("<svg/>")).ServeHTTP(response, request)
	var scriptPolicy string
	for _, directive := range strings.Split(response.Header().Get("Content-Security-Policy"), ";") {
		if strings.HasPrefix(strings.TrimSpace(directive), "script-src ") {
			scriptPolicy = strings.TrimSpace(directive)
		}
	}
	if scriptPolicy != "script-src 'self'" {
		t.Fatalf("scripts must be same-origin only, without inline or eval permissions: %s", scriptPolicy)
	}
	for _, expected := range []string{`<script src="viewer.js" defer></script>`, `<img id="graph"`, `id="zoom-in"`, `id="zoom-out"`, `id="fit"`, `tabindex="0"`} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Errorf("missing viewer markup %q", expected)
		}
	}
	if strings.Contains(response.Body.String(), "<object") || strings.Contains(response.Body.String(), "<iframe") || strings.Contains(response.Body.String(), "<svg") {
		t.Fatal("graph must remain an isolated image, not an active SVG document")
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
	go func() { result <- Serve(ctx, listener, []byte("<svg/>")) }()
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
