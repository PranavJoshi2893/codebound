package calltree

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyzeCallsAndBoundaries(t *testing.T) {
	dir := fixture(t, map[string]string{
		"main.go": `package app
import "example.com/project/handler"
type runner interface { Run() }
type worker struct{}
func (worker) Run() {}
func invoke(r runner) { r.Run() }
func execute(callback func()) { callback() }
func task() {}
func cycle() { cycle() }
func main() { invoke(worker{}); execute(task); handler.Handle(); cycle() }
`,
		"handler/handler.go": `package handler
import (
 "example.com/project/repository"
 "example.com/project/service"
)
func Handle() { service.Do(); repository.Get() }
`,
		"service/service.go": `package service
import "example.com/project/repository"
func Do() { repository.Get() }
`,
		"repository/repository.go": "package repository\nfunc Get() {}\n",
	})
	analyzer := New(dir)
	analyzer.Pattern = "./..."
	graph, err := analyzer.Analyze(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{graph.Root.Name: true}
	var violation, interfaceCall, recursion bool
	for _, call := range graph.Calls {
		names[call.Callee.Name] = true
		violation = violation || call.Violation == "handler must not call repository"
		interfaceCall = interfaceCall || (call.Callee.Name == "worker.Run" && call.ThroughInterface)
		recursion = recursion || (call.Caller.Name == "cycle" && call.Callee.Name == "cycle")
		if filepath.IsAbs(call.Callee.File) {
			t.Errorf("expected relative source path, got %q", call.Callee.File)
		}
	}
	for _, name := range []string{"main", "invoke", "worker.Run", "execute", "task", "Handle", "Do", "Get", "cycle"} {
		if !names[name] {
			t.Errorf("missing function %q", name)
		}
	}
	if !violation || !interfaceCall || !recursion {
		t.Fatalf("missing graph behavior: violation=%t interface=%t recursion=%t", violation, interfaceCall, recursion)
	}
	second, err := analyzer.Analyze(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, _ := json.Marshal(graph)
	secondJSON, _ := json.Marshal(second)
	if !bytes.Equal(firstJSON, secondJSON) {
		t.Fatal("analysis output is not deterministic")
	}
}

func TestRootResolution(t *testing.T) {
	dir := fixture(t, map[string]string{
		"main.go":        "package app\nimport \"example.com/project/other\"\nfunc Run() { other.Run() }\n",
		"other/other.go": "package other\nfunc Run() {}\ntype Worker struct{}\nfunc (*Worker) Start() {}\n",
	})
	analyzer := New(dir)
	analyzer.Pattern = "./..."
	for _, name := range []string{"example.com/project.Run", "Worker.Start", "example.com/project/other.Worker.Start"} {
		if _, err := analyzer.Analyze(context.Background(), name); err != nil {
			t.Errorf("root %q: %v", name, err)
		}
	}
	for name, expected := range map[string]string{"Run": "ambiguous", "Missing": "not found"} {
		if _, err := analyzer.Analyze(context.Background(), name); err == nil || !strings.Contains(err.Error(), expected) {
			t.Errorf("root %q: expected %s, got %v", name, expected, err)
		}
	}
}

func TestAnalyzeErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := New(".").Analyze(ctx, "main"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	dir := fixture(t, map[string]string{"main.go": "package app\nfunc Broken( {}"})
	if _, err := New(dir).Analyze(context.Background(), "Broken"); err == nil {
		t.Fatal("invalid source was accepted")
	}
}

func fixture(t *testing.T, files map[string]string) string {
	t.Helper()
	t.Setenv("GOWORK", "off")
	dir := t.TempDir()
	files["go.mod"] = "module example.com/project\n\ngo 1.24\n"
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}
