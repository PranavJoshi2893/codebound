// Package app owns CodeBound's use cases. It has no terminal, command framework,
// HTTP server, or process-signal dependency; those are supplied by adapters.
package app

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/PranavJoshi2893/codebound/internal/project"
)

var ErrCancelled = errors.New("cancelled")

type Action string

const (
	CreateProject Action = "create"
	ViewCalltree  Action = "calltree"
)

type Prompter interface {
	ChooseAction(context.Context) (Action, error)
	Project(context.Context) (project.Options, error)
	CalltreePath(context.Context) (string, error)
	ChooseRoot(context.Context, []string) (string, error)
}

// Dependencies are assembled at the executable boundary. Tests and other
// frontends can run the same workflows without a terminal or external tools.
type Dependencies struct {
	Prompter      Prompter
	Output        io.Writer
	Load          Loader
	Browser       Browser
	CreateProject func(context.Context, project.Options) error
	CreateModule  func(context.Context, string) error
}

type Application struct{ dependencies Dependencies }

func New(dependencies Dependencies) *Application {
	return &Application{dependencies: dependencies}
}

func (a *Application) Create(ctx context.Context, options project.Options) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := options.Validate(); err != nil {
		return err
	}
	if err := a.dependencies.CreateProject(ctx, options); err != nil {
		return err
	}
	_, err := fmt.Fprintf(a.dependencies.Output, "Project %s created.\n\n  cd %s\n  go run ./cmd/api\n", options.RootDir, options.RootDir)
	return err
}

func (a *Application) Module(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return a.dependencies.CreateModule(ctx, name)
}

func (a *Application) Interactive(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	action, err := a.dependencies.Prompter.ChooseAction(ctx)
	if err != nil {
		return err
	}
	switch action {
	case CreateProject:
		options, err := a.dependencies.Prompter.Project(ctx)
		if err != nil {
			return err
		}
		return a.Create(ctx, options)
	case ViewCalltree:
		path, err := a.dependencies.Prompter.CalltreePath(ctx)
		if err != nil {
			return err
		}
		options := DefaultCalltreeOptions()
		options.Path = path
		return a.Calltree(ctx, options)
	default:
		return fmt.Errorf("unknown action %q", action)
	}
}
