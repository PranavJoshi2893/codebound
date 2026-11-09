// Package create generates the default project from an embedded blueprint.
package create

import (
	"context"
	"embed"
	"fmt"
	"os"

	"github.com/PranavJoshi2893/codebound/internal/scaffold/templatefile"
	"github.com/PranavJoshi2893/codebound/internal/scaffold/toolchain"
)

//go:embed templates/*
var templateFiles embed.FS

type TemplateData struct{ ModPath string }

func projectPlan(mod string) (*templatefile.Plan, error) {
	return (templatefile.Blueprint{
		Directories: []string{"cmd/api", "internal/config", "internal/database", "internal/server", "internal/router", "internal/response", "migrations"},
		Templates: []templatefile.Template{
			{Source: "templates/makefile.tmpl", Destination: "Makefile", Mode: 0644},
			{Source: "templates/.env.tmpl", Destination: ".env", Mode: 0600},
			{Source: "templates/.gitignore.tmpl", Destination: ".gitignore", Mode: 0644},
			{Source: "templates/main.go.tmpl", Destination: "cmd/api/main.go", Mode: 0644},
			{Source: "templates/config.go.tmpl", Destination: "internal/config/config.go", Mode: 0644},
			{Source: "templates/database.go.tmpl", Destination: "internal/database/database.go", Mode: 0644},
			{Source: "templates/server.go.tmpl", Destination: "internal/server/server.go", Mode: 0644},
			{Source: "templates/router.go.tmpl", Destination: "internal/router/router.go", Mode: 0644},
			{Source: "templates/response.go.tmpl", Destination: "internal/response/response.go", Mode: 0644},
		},
	}).Compile(templateFiles, TemplateData{ModPath: mod})
}

type Generator struct{ Tools toolchain.Runner }

func InitializeProjectContext(ctx context.Context, rootDir, modPath string) error {
	return (Generator{Tools: toolchain.System{}}).Create(ctx, rootDir, modPath)
}

func (g Generator) Create(ctx context.Context, rootDir, mod string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if mod == "" {
		mod = rootDir
	}
	plan, err := projectPlan(mod)
	if err != nil {
		return err
	}
	if err := os.Mkdir(rootDir, 0755); err != nil {
		return fmt.Errorf("failed to create root directory: %w", err)
	}
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := g.Tools.Run(ctx, rootDir, "go", "mod", "init", "--", mod); err != nil {
		return err
	}
	if err := plan.Apply(ctx, root); err != nil {
		return err
	}
	if err := g.Tools.Run(ctx, rootDir, "go", "mod", "tidy"); err != nil {
		return err
	}
	return g.Tools.Run(ctx, rootDir, "git", "init")
}
