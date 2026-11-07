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
		{"/calltree.svg", "GET", "image/svg+xml", "<svg>graph</svg>", 200},
		{"/missing", "GET", "", "", 404},
		{"/calltree.svg", "POST", "", "", 405},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(test.method, test.path, nil))
		if response.Code != test.status ||
			!strings.Contains(response.Header().Get("Content-Type"), test.contentType) ||
			!strings.Contains(response.Body.String(), test.body) {
			t.Errorf("%s %s: unexpected response %d %s", test.method, test.path, response.Code, response.Body.String())
		}
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
