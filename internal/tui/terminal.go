package tui

import (
	"context"
	"io"

	"github.com/PranavJoshi2893/codebound/internal/app"
	"github.com/PranavJoshi2893/codebound/internal/project"
)

// Terminal adapts the existing forms to the application's prompt boundary.
type Terminal struct {
	Input  io.Reader
	Output io.Writer
}

func (t Terminal) ChooseAction(ctx context.Context) (app.Action, error) {
	return ChooseAction(ctx, t.Input, t.Output)
}

func (t Terminal) Project(ctx context.Context) (project.Options, error) {
	return Run(ctx, t.Input, t.Output)
}

func (t Terminal) CalltreePath(ctx context.Context) (string, error) {
	return CalltreePath(ctx, t.Input, t.Output)
}

func (t Terminal) ChooseRoot(ctx context.Context, roots []string) (string, error) {
	return ChooseRoot(ctx, t.Input, t.Output, roots)
}
