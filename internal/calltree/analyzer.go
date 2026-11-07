package calltree

import (
	"context"
	"fmt"
	"path/filepath"

	"golang.org/x/tools/go/callgraph/cha"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

type Analyzer struct {
	Dir     string
	Pattern string
}

func New(dir string) *Analyzer {
	return &Analyzer{Dir: dir, Pattern: "."}
}

// Analyze builds a reachable, module-local graph using class hierarchy analysis.
// CHA is conservative: interface and callback edges are possible calls, not
// evidence that those calls occur at runtime.
func (a *Analyzer) Analyze(ctx context.Context, rootName string) (*Graph, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	dir, err := filepath.Abs(a.Dir)
	if err != nil {
		return nil, fmt.Errorf("resolve project directory: %w", err)
	}
	loaded, err := loadPackages(ctx, dir, a.Pattern)
	if err != nil {
		return nil, err
	}
	program, _ := ssautil.AllPackages(loaded, ssa.InstantiateGenerics)
	program.Build()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	project := projectPackages(loaded)
	root, err := findFunction(ssautil.AllFunctions(program), project, rootName)
	if err != nil {
		return nil, err
	}
	graph := cha.CallGraph(program)
	return collectCalls(ctx, program, graph, project, root, dir)
}

func loadPackages(ctx context.Context, dir, pattern string) ([]*packages.Package, error) {
	loaded, err := packages.Load(&packages.Config{
		Context: ctx,
		Dir:     dir,
		Mode:    packages.LoadAllSyntax | packages.NeedModule,
	}, pattern)
	if err != nil {
		return nil, fmt.Errorf("load packages: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(loaded) == 0 {
		return nil, fmt.Errorf("pattern %q matched no packages", pattern)
	}
	var loadErr error
	packages.Visit(loaded, func(pkg *packages.Package) bool {
		if loadErr == nil && len(pkg.Errors) > 0 {
			loadErr = fmt.Errorf("loading %s: %s", pkg.PkgPath, pkg.Errors[0])
		}
		return true
	}, nil)
	if loadErr != nil {
		return nil, loadErr
	}
	return loaded, nil
}
