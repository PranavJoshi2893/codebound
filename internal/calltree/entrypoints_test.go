package calltree

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestAutoRootForNestedMain(t *testing.T) {
	dir := fixture(t, map[string]string{
		"cmd/api/main.go": "package main\nfunc main() {}\n",
	})
	analyzer := New(dir)
	analyzer.Pattern = "./..."
	graph, err := analyzer.Analyze(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if graph.Root.Package != "example.com/project/cmd/api" {
		t.Fatalf("wrong automatic root: %+v", graph.Root)
	}
}

func TestEntryPointsSortedAndReusable(t *testing.T) {
	dir := fixture(t, map[string]string{
		"app.go":             "package app\nfunc main() {}\n",
		"cmd/api/main.go":    "package main\nfunc main() {}\n",
		"cmd/worker/main.go": "package main\nfunc main() {}\n",
	})
	analyzer := New(dir)
	analyzer.Pattern = "./..."
	analysis, err := analyzer.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"example.com/project/cmd/api.main", "example.com/project/cmd/worker.main"}
	if roots := analysis.EntryPoints(); !reflect.DeepEqual(roots, want) {
		t.Fatalf("got %v, want %v", roots, want)
	}
	_, err = analysis.Analyze(context.Background(), "")
	var choices *EntryPointError
	if !errors.As(err, &choices) || !reflect.DeepEqual(choices.Candidates, want) {
		t.Fatalf("missing root choices: %v", err)
	}
	for _, root := range want {
		if _, err := analysis.Analyze(context.Background(), root); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLibraryRequiresExplicitRoot(t *testing.T) {
	dir := fixture(t, map[string]string{"app.go": "package app\nfunc Run() {}\n"})
	analyzer := New(dir)
	_, err := analyzer.Analyze(context.Background(), "")
	var choices *EntryPointError
	if !errors.As(err, &choices) || len(choices.Candidates) != 0 {
		t.Fatalf("expected no-main diagnostic: %v", err)
	}
	if _, err := analyzer.Analyze(context.Background(), "Run"); err != nil {
		t.Fatal(err)
	}
}
