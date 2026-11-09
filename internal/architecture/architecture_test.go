// Package architecture makes the implementation's dependency rules executable.
package architecture

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

const module = "github.com/PranavJoshi2893/codebound/"

func TestPackageBoundaries(t *testing.T) {
	// These are direct-import rules. In particular, diagram/model cannot depend
	// on the analyzer, and application workflows cannot depend on UI adapters.
	allowed := map[string][]string{
		"internal/app":                   {"internal/calltree", "internal/calltree/diagram", "internal/project"},
		"internal/calltree":              {"internal/calltree/model"},
		"internal/calltree/model":        {},
		"internal/calltree/diagram":      {"internal/calltree/model"},
		"internal/calltree/viewer":       {"internal/calltree/model", "internal/calltree/diagram"},
		"internal/project":               {"internal/scaffold/create"},
		"internal/scaffold/create":       {"internal/scaffold/templatefile", "internal/scaffold/toolchain"},
		"internal/scaffold/module":       {"internal/scaffold/templatefile"},
		"internal/scaffold/templatefile": {},
		"internal/scaffold/toolchain":    {},
		"internal/tui":                   {"internal/app", "internal/project"},
	}
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate repository")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../.."))
	err := filepath.WalkDir(filepath.Join(root, "internal"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		directory, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}
		pkg := filepath.ToSlash(directory)
		permitted, known := allowed[pkg]
		if !known {
			t.Errorf("declare dependency rules for new package %s", pkg)
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, item := range file.Imports {
			importPath, err := strconv.Unquote(item.Path.Value)
			if err != nil {
				return err
			}
			if !strings.HasPrefix(importPath, module) {
				continue
			}
			dependency := strings.TrimPrefix(importPath, module)
			accepted := false
			for _, name := range permitted {
				if name == dependency {
					accepted = true
					break
				}
			}
			if !accepted {
				t.Errorf("%s must not import %s (%s)", pkg, dependency, filepath.Base(path))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
