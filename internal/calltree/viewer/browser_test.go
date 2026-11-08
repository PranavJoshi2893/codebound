package viewer

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

//go:embed viewer_test.js
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
	mux.Handle("/", Handler([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="1600" height="1200"><text x="50" y="50">Test graph</text></svg>`)))
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
