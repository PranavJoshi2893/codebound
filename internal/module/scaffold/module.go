package scaffold

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

func CreateAPIModule(moduleName string) error {
	root, err := findProjectRoot()
	if err != nil {
		return err
	}

	folders := []string{
		filepath.Join(root, "internal", moduleName),
	}

	for _, folder := range folders {
		if err := os.MkdirAll(folder, 0755); err != nil {
			return fmt.Errorf("failed to create folder %s: %w", folder, err)
		}
	}

	if err := createModuleFiles(root, moduleName); err != nil {
		return err
	}

	return nil
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

type ModuleTemplateData struct {
	Name string
}

func createModuleFiles(rootDir string, moduleName string) error {
	files := map[string]string{
		"templates/handler.go.tmpl": filepath.Join("internal", moduleName, "handler.go"),
		"templates/service.go.tmpl": filepath.Join("internal", moduleName, "service.go"),
	}

	data := ModuleTemplateData{
		Name: moduleName,
	}

	for src, dest := range files {
		content, err := templateFiles.ReadFile(src)
		if err != nil {
			return fmt.Errorf("failed to read embedded template %s: %w", src, err)
		}

		destPath := filepath.Join(rootDir, dest)

		destFile, err := os.Create(destPath)
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", dest, err)
		}
		defer destFile.Close()

		tmpl, err := template.New(filepath.Base(src)).Parse(string(content))
		if err != nil {
			return fmt.Errorf("failed to parse template %s: %w", src, err)
		}

		if err := tmpl.Execute(destFile, data); err != nil {
			return fmt.Errorf("failed to execute template %s: %w", src, err)
		}
	}

	return nil
}
