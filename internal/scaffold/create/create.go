package create

import (
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/PranavJoshi2893/codebound/internal/scaffold/templatefile"
)

func InitializeProject(rootDir string, modPath string) error {
	mod := modPath
	if mod == "" {
		mod = rootDir
	}

	if err := os.Mkdir(rootDir, 0755); err != nil {
		return fmt.Errorf("failed to create root directory: %w", err)
	}

	cmd := exec.Command("go", "mod", "init", "--", mod)
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
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		return err
	}
	defer root.Close()
	folders := []string{
		filepath.Join("cmd", "api"),
		filepath.Join("internal", "config"),
		filepath.Join("internal", "database"),
		filepath.Join("internal", "server"),
		filepath.Join("internal", "router"),
		filepath.Join("internal", "response"),
		"migrations",
	}

	for _, folder := range folders {
		err := root.MkdirAll(folder, 0755)
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
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		return err
	}
	defer root.Close()
	files := map[string]string{
		"templates/makefile.tmpl":    "Makefile",
		"templates/.env.tmpl":        ".env",
		"templates/.gitignore.tmpl":  ".gitignore",
		"templates/main.go.tmpl":     "cmd/api/main.go",
		"templates/config.go.tmpl":   "internal/config/config.go",
		"templates/database.go.tmpl": "internal/database/database.go",
		"templates/server.go.tmpl":   "internal/server/server.go",
		"templates/router.go.tmpl":   "internal/router/router.go",
		"templates/response.go.tmpl": "internal/response/response.go",
	}

	data := TemplateData{
		ModPath: modPath,
	}

	for src, dest := range files {
		content, err := templateFiles.ReadFile(src)
		if err != nil {
			return fmt.Errorf("failed to read embedded template %s: %w", src, err)
		}

		perm := os.FileMode(0644)
		if dest == ".env" {
			perm = 0600
		}

		if err := templatefile.Write(root, dest, string(content), data, perm); err != nil {
			return err
		}
	}

	return nil
}
