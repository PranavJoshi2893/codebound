package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootWithoutTerminalShowsHelp(t *testing.T) {
	var output bytes.Buffer
	rootCmd.SetIn(strings.NewReader(""))
	rootCmd.SetOut(&output)
	rootCmd.SetErr(&output)
	rootCmd.SetArgs([]string{})
	t.Cleanup(func() {
		rootCmd.SetIn(nil)
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Available Commands:") {
		t.Fatalf("expected help without a terminal, got %q", output.String())
	}
}
