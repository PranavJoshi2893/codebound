// Package cmd adapts command-line arguments and streams to application use cases.
package cmd

import (
	"context"
	"errors"

	"github.com/PranavJoshi2893/codebound/internal/app"
	"github.com/PranavJoshi2893/codebound/internal/calltree"
	"github.com/PranavJoshi2893/codebound/internal/calltree/viewer"
	"github.com/PranavJoshi2893/codebound/internal/project"
	"github.com/PranavJoshi2893/codebound/internal/scaffold/module"
	"github.com/PranavJoshi2893/codebound/internal/tui"
	"github.com/spf13/cobra"
)

// NewCommand builds an independent command tree. No flags, streams, or execution
// state are shared between invocations.
func NewCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "codebound",
		Short: "Define and enforce code boundaries",
		Long: `CodeBound is an opinionated tool for creating and maintaining
consistent project structures and architectural boundaries.

Run without arguments in a terminal to create a project or view a call tree,
or use create <project-name> for non-interactive creation.`,
		Version: "0.1.4-nightly.7",
		Args:    cobra.NoArgs,
		RunE:    interactiveOrHelp,
	}
	command.AddCommand(newCreateCommand(), newModuleCommand(), newCalltreeCommand())
	return command
}

// Composition belongs here, not in the use cases or presentation adapters.
func application(command *cobra.Command) *app.Application {
	dependencies := app.Dependencies{
		Output:  command.OutOrStdout(),
		Browser: viewer.Browser{},
		Load: func(ctx context.Context, path, pattern string) (app.Analysis, error) {
			analyzer := calltree.New(path)
			analyzer.Pattern = pattern
			return analyzer.Load(ctx)
		},
		CreateProject: project.CreateContext,
		CreateModule:  module.CreateAPIModuleContext,
	}
	if canPrompt(command) {
		dependencies.Prompter = tui.Terminal{Input: command.InOrStdin(), Output: command.OutOrStdout()}
	}
	return app.New(dependencies)
}

func commandError(err error) error {
	if errors.Is(err, app.ErrCancelled) {
		return nil
	}
	return err
}
