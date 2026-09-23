package scaffold

import (
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

func InitializeProject(rootDir string, modPath string) error {
	mod := modPath
	if mod == "" {
		mod = rootDir
	}

	if err := os.Mkdir(rootDir, 0755); err != nil {
		return fmt.Errorf("failed to create root directory: %w", err)
	}

	cmd := exec.Command("go", "mod", "init", mod)
	cmd.Dir = rootDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run go mod init: %w (output: %s)", err, string(output))
	}

	if err := createFolders(rootDir); err != nil {
		return err
	}

	if err := createFiles(rootDir, mod); err != nil {
		return err
	}

	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = rootDir

	tidyOutput, err := tidyCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run go mod tidy: %w (output: %s)", err, string(tidyOutput))
	}

	gitInitCmd := exec.Command("git", "init")
	gitInitCmd.Dir = rootDir

	gitInitOutput, err := gitInitCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run git init: %w (output: %s)", err, string(gitInitOutput))
	}

	return nil
}

func createFolders(rootDir string) error {
	folders := []string{
		filepath.Join(rootDir, "cmd", "api"),
		filepath.Join(rootDir, "internal", "config"),
		filepath.Join(rootDir, "internal", "database"),
		filepath.Join(rootDir, "internal", "server"),
		filepath.Join(rootDir, "internal", "router"),
		filepath.Join(rootDir, "migrations"),
	}

	for _, folder := range folders {
		err := os.MkdirAll(folder, 0755)
		if err != nil {
			return fmt.Errorf("failed to create folder: %v", err)
		}
	}

	return nil
}

//go:embed templates/*
var templateFiles embed.FS

type TemplateData struct {
	ModPath string
}

func createFiles(rootDir string, modPath string) error {
	files := map[string]string{
		"templates/makefile.tmpl":    "Makefile",
		"templates/.env.tmpl":        ".env",
		"templates/.gitignore.tmpl":  ".gitignore",
		"templates/main.go.tmpl":     "cmd/api/main.go",
		"templates/config.go.tmpl":   "internal/config/config.go",
		"templates/database.go.tmpl": "internal/database/database.go",
		"templates/server.go.tmpl":   "internal/server/server.go",
		"templates/router.go.tmpl":   "internal/router/router.go",
	}

	data := TemplateData{
		ModPath: modPath,
	}

	for src, dest := range files {
		content, err := templateFiles.ReadFile(src)
		if err != nil {
			return fmt.Errorf("failed to read embedded template %s: %w", src, err)
		}

		destPath := filepath.Join(rootDir, dest)

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", dest, err)
		}

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
