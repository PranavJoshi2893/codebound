package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestModuleCommandPropagatesGenerationFailure(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/app\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "internal", "users"), 0755); err != nil {
		t.Fatal(err)
	}
	moduleCmd := newModuleCommand()
	moduleCmd.SetArgs([]string{"users"})
	moduleCmd.SilenceErrors = true
	if err := moduleCmd.Execute(); err == nil {
		t.Fatal("module command swallowed the generation error")
	}
}
