package cmd

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRootWithoutTerminalShowsHelp(t *testing.T) {
	var output bytes.Buffer
	rootCmd := NewCommand()
	rootCmd.SetIn(strings.NewReader(""))
	rootCmd.SetOut(&output)
	rootCmd.SetErr(&output)
	rootCmd.SetArgs([]string{})
	if err := rootCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Available Commands:") {
		t.Fatalf("expected help without a terminal, got %q", output.String())
	}
	if rootCmd.Flags().Lookup("toggle") != nil || strings.Contains(output.String(), "--toggle") {
		t.Fatal("root help must not advertise the unused Cobra starter flag")
	}
}

func TestCommandTreesDoNotShareState(t *testing.T) {
	first, second := NewCommand(), NewCommand()
	firstCalltree, _, err := first.Find([]string{"calltree"})
	if err != nil {
		t.Fatal(err)
	}
	secondCalltree, _, err := second.Find([]string{"calltree"})
	if err != nil {
		t.Fatal(err)
	}
	if err := firstCalltree.Flags().Set("format", "json"); err != nil {
		t.Fatal(err)
	}
	format, err := secondCalltree.Flags().GetString("format")
	if err != nil || format != "web" {
		t.Fatalf("shared flags: %q, %v", format, err)
	}
	first.SilenceUsage = true
	if second.SilenceUsage {
		t.Fatal("shared execution state")
	}
}

func TestRootCommandsHonorCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, args := range [][]string{{"create", "example"}, {"module", "users"}, {"calltree", "--format", "json"}} {
		command := NewCommand()
		command.SetArgs(args)
		command.SetOut(&bytes.Buffer{})
		command.SetErr(&bytes.Buffer{})
		if err := command.ExecuteContext(ctx); !errors.Is(err, context.Canceled) {
			t.Fatalf("%v: %v", args, err)
		}
	}
}
