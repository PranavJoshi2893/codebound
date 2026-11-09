package module

import (
	"context"
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
	return CreateAPIModuleContext(context.Background(), moduleName)
}

func CreateAPIModuleContext(ctx context.Context, moduleName string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := ValidateName(moduleName); err != nil {
		return err
	}
	plan, err := modulePlan(moduleName)
	if err != nil {
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
	return plan.Apply(ctx, moduleRoot)
}

func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		info, err := os.Stat(filepath.Join(dir, "go.mod"))
		if err == nil {
			if info.IsDir() {
				return "", fmt.Errorf("go.mod is a directory in %s", dir)
			}
			return dir, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("find go.mod: %w", err)
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

func modulePlan(name string) (*templatefile.Plan, error) {
	return (templatefile.Blueprint{Templates: []templatefile.Template{
		{Source: "templates/handler.go.tmpl", Destination: "handler.go", Mode: 0644},
		{Source: "templates/service.go.tmpl", Destination: "service.go", Mode: 0644},
	}}).Compile(templateFiles, TemplateData{Name: name})
}
