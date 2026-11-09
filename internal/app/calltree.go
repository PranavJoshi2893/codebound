package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/PranavJoshi2893/codebound/internal/calltree"
	"github.com/PranavJoshi2893/codebound/internal/calltree/diagram"
)

type Analysis interface {
	EntryPoints() []string
	Analyze(context.Context, string) (*calltree.Graph, error)
}

type Loader func(context.Context, string, string) (Analysis, error)

type Browser interface {
	Check() error
	Show(context.Context, io.Writer, *calltree.Graph, int) error
}

type CalltreeOptions struct {
	Path, Pattern, Root, Format string
	Port                        int
}

func DefaultCalltreeOptions() CalltreeOptions {
	return CalltreeOptions{Path: ".", Pattern: "./...", Format: "web"}
}

func (o CalltreeOptions) Validate() error {
	if strings.TrimSpace(o.Path) == "" || strings.TrimSpace(o.Pattern) == "" {
		return fmt.Errorf("path and pattern must not be empty")
	}
	switch o.Format {
	case "web", "dot", "json":
	default:
		return fmt.Errorf("unsupported format %q; choose web, dot, or json", o.Format)
	}
	if o.Port < 0 || o.Port > 65535 {
		return fmt.Errorf("port must be between 0 and 65535")
	}
	return nil
}

func (a *Application) Calltree(ctx context.Context, options CalltreeOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := options.Validate(); err != nil {
		return err
	}
	if options.Format == "web" {
		if err := a.dependencies.Browser.Check(); err != nil {
			return err
		}
	}
	analysis, err := a.dependencies.Load(ctx, options.Path, options.Pattern)
	if err != nil {
		return err
	}
	root, err := a.chooseRoot(ctx, analysis, options)
	if err != nil {
		return err
	}
	graph, err := analysis.Analyze(ctx, root)
	if err != nil {
		return err
	}
	switch options.Format {
	case "dot":
		return diagram.WriteDOT(a.dependencies.Output, graph)
	case "json":
		encoder := json.NewEncoder(a.dependencies.Output)
		encoder.SetIndent("", "  ")
		return encoder.Encode(graph)
	default:
		return a.dependencies.Browser.Show(ctx, a.dependencies.Output, graph, options.Port)
	}
}

func (a *Application) chooseRoot(ctx context.Context, analysis Analysis, options CalltreeOptions) (string, error) {
	if options.Root != "" {
		return options.Root, nil
	}
	roots := analysis.EntryPoints()
	if len(roots) == 1 {
		return roots[0], nil
	}
	if len(roots) > 1 && options.Format == "web" && a.dependencies.Prompter != nil {
		return a.dependencies.Prompter.ChooseRoot(ctx, roots)
	}
	return "", &calltree.EntryPointError{Candidates: roots}
}
