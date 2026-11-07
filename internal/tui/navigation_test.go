package tui

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	for _, path := range []string{"", ".", "~", "~/", "  "} {
		got, err := projectDirectory(path)
		if err != nil || got != dir {
			t.Errorf("path %q: got %q, %v", path, got, err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "file"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"missing", "file"} {
		if _, err := projectDirectory(path); err == nil {
			t.Errorf("accepted non-directory %q", path)
		}
	}
}

func TestRootChoiceSkipsUnnecessaryPrompt(t *testing.T) {
	got, err := ChooseRoot(context.Background(), strings.NewReader(""), io.Discard, []string{"example.com/app.main"})
	if err != nil || got != "example.com/app.main" {
		t.Fatalf("unexpected root: %q, %v", got, err)
	}
	if _, err := ChooseRoot(context.Background(), strings.NewReader(""), io.Discard, nil); err == nil {
		t.Fatal("empty roots accepted")
	}
}

func TestPromptsHonorCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ChooseAction(ctx, strings.NewReader(""), io.Discard); !errors.Is(err, context.Canceled) {
		t.Fatalf("action prompt ignored context: %v", err)
	}
	if _, err := CalltreePath(ctx, strings.NewReader(""), io.Discard); !errors.Is(err, context.Canceled) {
		t.Fatalf("path prompt ignored context: %v", err)
	}
	if _, err := ChooseRoot(ctx, strings.NewReader(""), io.Discard, []string{"first.main", "second.main"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("root prompt ignored context: %v", err)
	}
}
