package templatefile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteRejectsTraversalAndSymlinks(t *testing.T) {
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := Write(root, "../escape", "content", nil, 0644); err == nil {
		t.Fatal("parent traversal was accepted")
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := Write(root, "link/file.go", "content", nil, 0644); err == nil {
		t.Fatal("escaping symlink was accepted")
	}
	if entries, err := os.ReadDir(outside); err != nil || len(entries) != 0 {
		t.Fatalf("wrote outside root: %v, %v", entries, err)
	}
}

func TestRenderFailureDoesNotCreateFile(t *testing.T) {
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := Write(root, "file.go", "{{.Missing}}", struct{}{}, 0644); err == nil {
		t.Fatal("invalid template was accepted")
	}
	if _, err := root.Stat("file.go"); !os.IsNotExist(err) {
		t.Fatalf("render failure created a destination: %v", err)
	}
}
