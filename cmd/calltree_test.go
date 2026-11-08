package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PranavJoshi2893/codebound/internal/calltree"
)

func TestCalltreeExports(t *testing.T) {
	t.Setenv("GOWORK", "off")
	dir := t.TempDir()
	for name, content := range map[string]string{
		"go.mod":  "module example.com/app\n\ngo 1.24\n",
		"main.go": "package main\nfunc main() { greet() }\nfunc greet() {}\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, format := range []string{"dot", "json"} {
		var output bytes.Buffer
		command := newCalltreeCommand()
		command.SetOut(&output)
		command.SetErr(&output)
		command.SetArgs([]string{"--path", dir, "--format", format})
		if err := command.Execute(); err != nil {
			t.Fatal(err)
		}
		if format == "dot" {
			if !strings.HasPrefix(output.String(), "digraph CallTree") {
				t.Fatalf("invalid DOT output: %s", output.String())
			}
		} else {
			var graph calltree.Graph
			if err := json.Unmarshal(output.Bytes(), &graph); err != nil {
				t.Fatal(err)
			}
			if graph.Root.Name != "main" || len(graph.Calls) != 1 || graph.Calls[0].Callee.Name != "greet" {
				t.Fatalf("unexpected graph: %+v", graph)
			}
		}
	}
}

func TestCalltreeContextExports(t *testing.T) {
	dir := calltreeProject(t, map[string]string{"main.go": `package main
import "context"
func main() {
 ctx, cancel := context.WithCancel(context.Background())
 defer cancel()
 consume(ctx)
}
func consume(ctx context.Context) { _ = ctx.Err() }
`})
	for _, format := range []string{"dot", "json"} {
		t.Run(format, func(t *testing.T) {
			var output bytes.Buffer
			command := newCalltreeCommand()
			command.SetOut(&output)
			command.SetErr(&output)
			command.SetArgs([]string{dir, "--format", format})
			if err := command.Execute(); err != nil {
				t.Fatal(err)
			}
			if format == "dot" {
				for _, expected := range []string{`starts C2: WithCancel`, `cancel/stop C2 [deferred]`, `ctx: C2`, `reads Err: C2`, `label="ctx=C2"`} {
					if !strings.Contains(output.String(), expected) {
						t.Errorf("missing %q in context DOT export", expected)
					}
				}
				return
			}
			var graph calltree.Graph
			if err := json.Unmarshal(output.Bytes(), &graph); err != nil {
				t.Fatal(err)
			}
			if graph.Root.Context == nil || len(graph.Root.Context.Operations) == 0 {
				t.Fatal("missing root context metadata in JSON export")
			}
			if len(graph.Calls) != 1 || graph.Calls[0].Callee.Context == nil || len(graph.Calls[0].Callee.Context.Parameters) != 1 {
				t.Fatalf("missing forwarded context parameter: %+v", graph.Calls)
			}
		})
	}
}

func TestCalltreeRejectsInvalidFlags(t *testing.T) {
	for _, args := range [][]string{
		{"--format", "invalid"}, {"--port", "-1"}, {"--port", "65536"},
		{"--root", ""}, {"--pattern", ""}, {"--path", ""}, {"first", "second"},
		{"project", "--path", "other"},
	} {
		command := newCalltreeCommand()
		command.SilenceErrors = true
		command.SilenceUsage = true
		command.SetArgs(args)
		if err := command.Execute(); err == nil {
			t.Errorf("accepted invalid flags: %v", args)
		}
	}
}

func TestCalltreePositionalPathDiscoversNestedMain(t *testing.T) {
	dir := calltreeProject(t, map[string]string{
		"cmd/api/main.go": "package main\nfunc main() {}\n",
	})
	command := newCalltreeCommand()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetArgs([]string{dir, "--format", "json"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	var graph calltree.Graph
	if err := json.Unmarshal(output.Bytes(), &graph); err != nil {
		t.Fatal(err)
	}
	if graph.Root.Package != "example.com/app/cmd/api" {
		t.Fatalf("did not discover nested executable: %+v", graph.Root)
	}
}

func TestCalltreeDefaultsToCurrentDirectory(t *testing.T) {
	dir := calltreeProject(t, map[string]string{"cmd/api/main.go": "package main\nfunc main() {}\n"})
	t.Chdir(dir)
	command := newCalltreeCommand()
	command.SetOut(&bytes.Buffer{})
	command.SetArgs([]string{"--format", "dot"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestMultipleRootsWithoutTerminalReturnChoices(t *testing.T) {
	dir := calltreeProject(t, map[string]string{
		"cmd/api/main.go":    "package main\nfunc main() {}\n",
		"cmd/worker/main.go": "package main\nfunc main() {}\n",
	})
	command := newCalltreeCommand()
	command.SetIn(strings.NewReader(""))
	command.SetOut(&bytes.Buffer{})
	command.SetArgs([]string{dir, "--format", "json"})
	command.SilenceErrors = true
	err := command.Execute()
	var choices *calltree.EntryPointError
	if !errors.As(err, &choices) || len(choices.Candidates) != 2 {
		t.Fatalf("expected root choices instead of a prompt: %v", err)
	}
	if !command.SilenceUsage {
		t.Fatal("runtime error would dump command usage")
	}
	command = newCalltreeCommand()
	command.SetOut(&bytes.Buffer{})
	command.SetArgs([]string{dir, "--root", "example.com/app/cmd/worker.main", "--format", "json"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestMissingGraphvizReturnsShortDiagnostic(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	command := newCalltreeCommand()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs([]string{})
	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), "install Graphviz") {
		t.Fatalf("unexpected diagnostic: %v", err)
	}
	if strings.Contains(output.String(), "Usage:") {
		t.Fatalf("runtime failure dumped command usage: %s", output.String())
	}
}

func calltreeProject(t *testing.T, files map[string]string) string {
	t.Helper()
	t.Setenv("GOWORK", "off")
	dir := t.TempDir()
	files["go.mod"] = "module example.com/app\n\ngo 1.24\n"
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

func TestCalltreeHonorsCommandContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	command := newCalltreeCommand()
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs([]string{"--format", "json"})
	if err := command.ExecuteContext(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}
