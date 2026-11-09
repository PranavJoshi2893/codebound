package module

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func projectDirectory(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/app\n"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	return dir
}

func TestCancelledModuleDoesNotCreateDirectory(t *testing.T) {
	dir := projectDirectory(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := CreateAPIModuleContext(ctx, "users"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "internal")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("cancelled module changed filesystem")
	}
}

func TestModuleFindsProjectFromSubdirectory(t *testing.T) {
	dir := projectDirectory(t)
	nested := filepath.Join(dir, "internal", "nested")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(nested)
	if err := CreateAPIModule("users"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "internal", "users", "service.go")); err != nil {
		t.Fatal(err)
	}
}

func TestModuleRejectsDirectoryNamedGoMod(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.Mkdir(filepath.Join(dir, "go.mod"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := CreateAPIModule("users"); err == nil {
		t.Fatal("accepted directory as module manifest")
	}
}

func TestExistingModuleIsPreserved(t *testing.T) {
	root := projectDirectory(t)
	if err := CreateAPIModule("users"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "internal", "users", "handler.go")
	const existing = "user-written handler\n"
	if err := os.WriteFile(path, []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}
	if err := CreateAPIModule("users"); err == nil {
		t.Error("existing module was accepted")
	}
	content, err := os.ReadFile(path)
	if err != nil || string(content) != existing {
		t.Fatalf("existing file overwritten: %q, %v", content, err)
	}
}

func TestModuleCannotFollowEscapingSymlink(t *testing.T) {
	root := projectDirectory(t)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "internal")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := CreateAPIModule("users"); err == nil {
		t.Error("escaping symlink was accepted")
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("wrote outside project: %v, %v", entries, err)
	}
}

func TestModuleRejectsUnsafeNames(t *testing.T) {
	projectDirectory(t)
	for _, name := range []string{"../escape", "nested/name", "/absolute", "range", "users\npackage main", "tiny"} {
		if err := CreateAPIModule(name); err == nil {
			t.Errorf("accepted unsafe name %q", name)
		}
	}
}
