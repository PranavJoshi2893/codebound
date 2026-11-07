package module

import (
	"embed"
	"errors"
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"unicode"

	"github.com/PranavJoshi2893/codebound/internal/scaffold/templatefile"
)

func ValidateName(name string) error {
	if len(name) < 5 {
		return fmt.Errorf("api name must be at least 5 characters long")
	}
	for _, r := range name {
		if !unicode.IsLetter(r) {
			return fmt.Errorf("api name can only contain letters (invalid character: %c)", r)
		}
	}
	if !token.IsIdentifier(name) {
		return fmt.Errorf("api name must be a valid Go package identifier, not a keyword")
	}
	return nil
}

func CreateAPIModule(moduleName string) error {
	if err := ValidateName(moduleName); err != nil {
		return err
	}
	projectDir, err := findProjectRoot()
	if err != nil {
		return err
	}

	root, err := os.OpenRoot(projectDir)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := root.MkdirAll("internal", 0755); err != nil {
		return fmt.Errorf("create internal directory: %w", err)
	}
	path := filepath.Join("internal", moduleName)
	// Require a fresh directory: never reuse existing files or symlinks.
	if err := root.Mkdir(path, 0755); err != nil {
		return fmt.Errorf("create module directory: %w", err)
	}
	moduleRoot, err := root.OpenRoot(path)
	if err != nil {
		return err
	}
	defer moduleRoot.Close()
	return createModuleFiles(moduleRoot, moduleName)
}

func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("go.mod not found; not in a Go module")
		}
		dir = parent
	}
}

//go:embed templates/*
var templateFiles embed.FS

type TemplateData struct {
	Name string
}

func createModuleFiles(root *os.Root, moduleName string) error {
	files := map[string]string{
		"templates/handler.go.tmpl": "handler.go",
		"templates/service.go.tmpl": "service.go",
	}

	data := TemplateData{
		Name: moduleName,
	}

	for src, dest := range files {
		content, err := templateFiles.ReadFile(src)
		if err != nil {
			return fmt.Errorf("failed to read embedded template %s: %w", src, err)
		}

		if err := templatefile.Write(root, dest, string(content), data, 0644); err != nil {
			return err
		}
	}

	return nil
}
