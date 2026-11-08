package calltree

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// Load type-checks source and builds SSA once, before any root is selected.
func (a *Analyzer) Load(ctx context.Context) (*Analysis, error) {
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

	analysis := &Analysis{
		dir:             dir,
		program:         program,
		projectPackages: findProjectPackages(loaded),
		functions:       ssautil.AllFunctions(program),
	}
	// Root discovery does not need a call graph. Build it on the first query,
	// then share the same graph across later queries, including concurrent ones.
	analysis.callGraph = sync.OnceValue(analysis.buildCallGraph)
	return analysis, nil
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

func findProjectPackages(loaded []*packages.Package) map[string]bool {
	modules := make(map[string]bool)
	for _, pkg := range loaded {
		if pkg.Module != nil {
			modules[pkg.Module.Path] = true
		}
	}

	// Display only packages from the selected modules, not their dependencies.
	project := make(map[string]bool)
	packages.Visit(loaded, func(pkg *packages.Package) bool {
		if pkg.Module != nil && modules[pkg.Module.Path] {
			project[pkg.PkgPath] = true
		}
		return true
	}, nil)
	return project
}
