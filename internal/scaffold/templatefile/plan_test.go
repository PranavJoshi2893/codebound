package templatefile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestPlanRendersBeforeFilesystemChanges(t *testing.T) {
	sources := fstest.MapFS{"first": {Data: []byte("valid")}, "second": {Data: []byte("{{.Missing}}")}}
	blueprint := Blueprint{Templates: []Template{
		{Source: "first", Destination: "first.go", Mode: 0644},
		{Source: "second", Destination: "second.go", Mode: 0644},
	}}
	if _, err := blueprint.Compile(sources, struct{}{}); err == nil {
		t.Fatal("accepted failing template")
	}
}

func TestPlanRejectsUnsafeAndDuplicatePaths(t *testing.T) {
	sources := fstest.MapFS{"source": {Data: []byte("content")}}
	for _, name := range []string{"../outside", "/absolute", ".", "", "a/../b", "a\\b", "C:drive"} {
		if _, err := (Blueprint{Directories: []string{name}}).Compile(sources, nil); err == nil {
			t.Errorf("accepted directory %q", name)
		}
		if _, err := (Blueprint{Templates: []Template{{Source: "source", Destination: name, Mode: 0644}}}).Compile(sources, nil); err == nil {
			t.Errorf("accepted file %q", name)
		}
	}
	entry := Template{Source: "source", Destination: "file", Mode: 0644}
	if _, err := (Blueprint{Templates: []Template{entry, entry}}).Compile(sources, nil); err == nil {
		t.Fatal("accepted duplicate destinations")
	}
	for _, mode := range []os.FileMode{0, os.ModeSymlink | 0644} {
		entry.Mode = mode
		if _, err := (Blueprint{Templates: []Template{entry}}).Compile(sources, nil); err == nil {
			t.Fatal("accepted invalid permissions")
		}
	}
}

func TestPlanAppliesInDeclaredOrderAndPreservesFiles(t *testing.T) {
	directory := t.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := os.WriteFile(filepath.Join(directory, "existing"), []byte("user work"), 0644); err != nil {
		t.Fatal(err)
	}
	sources := fstest.MapFS{"source": {Data: []byte("hello {{.}}")}}
	plan, err := (Blueprint{Directories: []string{"empty"}, Templates: []Template{
		{Source: "source", Destination: "first", Mode: 0644},
		{Source: "source", Destination: "existing", Mode: 0644},
		{Source: "source", Destination: "last", Mode: 0644},
	}}).Compile(sources, "project")
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.Apply(context.Background(), root); !errors.Is(err, os.ErrExist) {
		t.Fatalf("expected exclusive creation failure, got %v", err)
	}
	for name, want := range map[string]string{"first": "hello project", "existing": "user work"} {
		content, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil || string(content) != want {
			t.Fatalf("%s: %q, %v", name, content, err)
		}
	}
	if _, err := os.Stat(filepath.Join(directory, "last")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("continued after failure")
	}
}

func TestPlanCancellationDoesNotCreateFiles(t *testing.T) {
	directory := t.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	plan, err := (Blueprint{Directories: []string{"empty"}}).Compile(fstest.MapFS{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := plan.Apply(ctx, root); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 0 {
		t.Fatalf("cancelled plan changed filesystem: %v, %v", entries, err)
	}
}
