package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"charm.land/huh/v2"
)

type Action string

const (
	CreateProject Action = "create"
	ViewCalltree  Action = "calltree"
)

func ChooseAction(ctx context.Context, input io.Reader, output io.Writer) (Action, error) {
	action := CreateProject
	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[Action]().Title("What would you like to do?").
			Options(huh.NewOption("Create a project", CreateProject), huh.NewOption("View a call tree", ViewCalltree)).
			Value(&action),
	).Title("Codebound"))
	err := runPrompt(ctx, form, input, output)
	return action, err
}

func CalltreePath(ctx context.Context, input io.Reader, output io.Writer) (string, error) {
	path := "."
	form := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Project directory?").
			Description("Press Enter to use the current directory.").Value(&path).
			Validate(func(value string) error {
				_, err := projectDirectory(value)
				return err
			}),
	).Title("View a call tree"))
	if err := runPrompt(ctx, form, input, output); err != nil {
		return "", err
	}
	return projectDirectory(path)
}

func ChooseRoot(ctx context.Context, input io.Reader, output io.Writer, roots []string) (string, error) {
	if len(roots) == 0 {
		return "", fmt.Errorf("no entry points available")
	}
	if len(roots) == 1 {
		return roots[0], nil
	}
	root := roots[0]
	options := make([]huh.Option[string], 0, len(roots))
	for _, name := range roots {
		options = append(options, huh.NewOption(name, name))
	}
	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title("Which entry point?").
			Description("This project has multiple applications.").
			Options(options...).Value(&root),
	).Title("View a call tree"))
	err := runPrompt(ctx, form, input, output)
	return root, err
}

func runPrompt(ctx context.Context, form *huh.Form, input io.Reader, output io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	err := form.WithTheme(huh.ThemeFunc(formTheme)).WithInput(input).WithOutput(output).RunWithContext(ctx)
	if errors.Is(err, huh.ErrUserAborted) {
		return ErrCancelled
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

func projectDirectory(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		path = "."
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~"))
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("open project directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("project path must be a directory")
	}
	return path, nil
}
