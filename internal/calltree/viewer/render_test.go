package viewer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/PranavJoshi2893/codebound/internal/calltree"
)

func TestRenderSVG(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test stubs use Unix shell scripts")
	}
	tools := t.TempDir()
	t.Setenv("PATH", tools)
	path := filepath.Join(tools, "dot")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf '<svg>graph</svg>'\n"), 0755); err != nil {
		t.Fatal(err)
	}
	svg, err := RenderSVG(context.Background(), &calltree.Graph{})
	if err != nil || string(svg) != "<svg>graph</svg>" {
		t.Fatalf("unexpected render: %s, %v", svg, err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf 'invalid graph' >&2\nexit 1\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := RenderSVG(context.Background(), &calltree.Graph{}); err == nil || !strings.Contains(err.Error(), "invalid graph") {
		t.Fatalf("missing Graphviz diagnostic: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := RenderSVG(ctx, &calltree.Graph{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}

func TestMissingGraphviz(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := RenderSVG(context.Background(), &calltree.Graph{}); err == nil || !strings.Contains(err.Error(), "Graphviz") {
		t.Fatalf("expected actionable missing-Graphviz error, got %v", err)
	}
}
