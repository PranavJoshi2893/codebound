package toolchain

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestDiagnosticsAreBounded(t *testing.T) {
	buffer := diagnosticBuffer{remaining: 10}
	for _, text := range []string{"1234567", "8901234567", "discarded"} {
		if n, err := buffer.Write([]byte(text)); n != len(text) || err != nil {
			t.Fatalf("write: %d, %v", n, err)
		}
	}
	if buffer.String() != "1234567890" {
		t.Fatalf("unbounded diagnostics: %q", buffer.String())
	}
}

func TestRunnerPreservesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := (System{}).Run(ctx, t.TempDir(), "nonexistent"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestRunnerCancelsRunningProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix executable fixture")
	}
	dir := t.TempDir()
	tool := filepath.Join(dir, "wait")
	// A shell-builtin loop avoids orphaning a sleep subprocess on cancellation.
	if err := os.WriteFile(tool, []byte("#!/bin/sh\nwhile :; do :; done\n"), 0755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := (System{}).Run(ctx, dir, tool); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("lost cancellation: %v", err)
	}
}

func TestRunnerReportsWorkingDirectoryAndFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix executable fixture")
	}
	dir := t.TempDir()
	tool := filepath.Join(dir, "tool")
	if err := os.WriteFile(tool, []byte("#!/bin/sh\npwd\nprintf '%s' \"$1\" >&2\nexit 3\n"), 0755); err != nil {
		t.Fatal(err)
	}
	err := (System{}).Run(context.Background(), dir, tool, "argument")
	if err == nil || !strings.Contains(err.Error(), dir) || !strings.Contains(err.Error(), "argument") || !strings.Contains(err.Error(), "exit status 3") {
		t.Fatalf("missing tool diagnostics: %v", err)
	}
}
