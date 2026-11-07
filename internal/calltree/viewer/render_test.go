package viewer

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

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

func TestLimitedBuffer(t *testing.T) {
	buffer := limitedBuffer{limit: 4}
	if n, err := buffer.Write([]byte("abcdef")); n != 4 || !errors.Is(err, errOutputLimit) || buffer.String() != "abcd" {
		t.Fatalf("output limit not enforced: n=%d err=%v content=%q", n, err, buffer.String())
	}
	if n, err := buffer.Write([]byte("more")); n != 0 || !errors.Is(err, errOutputLimit) {
		t.Fatalf("full buffer accepted more data: n=%d err=%v", n, err)
	}
}

func TestOutputLimitAppliesToStreamCopies(t *testing.T) {
	buffer := limitedBuffer{limit: 4}
	if _, err := io.Copy(&buffer, strings.NewReader("abcdef")); !errors.Is(err, errOutputLimit) {
		t.Fatalf("stream copy bypassed output limit: %v", err)
	}
	if buffer.String() != "abcd" {
		t.Fatalf("unexpected bounded output: %q", buffer.String())
	}
}

func TestRenderDeadline(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test stubs use Unix shell scripts")
	}
	tools := t.TempDir()
	t.Setenv("PATH", tools)
	if err := os.WriteFile(filepath.Join(tools, "dot"), []byte("#!/bin/sh\nwhile :; do :; done\n"), 0755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := RenderSVG(ctx, &calltree.Graph{}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("renderer did not honor deadline: %v", err)
	}
}

func TestMissingGraphviz(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := RenderSVG(context.Background(), &calltree.Graph{}); err == nil || !strings.Contains(err.Error(), "Graphviz") {
		t.Fatalf("expected actionable missing-Graphviz error, got %v", err)
	}
}
