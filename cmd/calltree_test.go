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

func TestCalltreeRejectsInvalidFlags(t *testing.T) {
	for _, args := range [][]string{
		{"--format", "invalid"}, {"--port", "-1"}, {"--port", "65536"},
		{"--root", ""}, {"--pattern", ""}, {"--path", ""}, {"unexpected-argument"},
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
