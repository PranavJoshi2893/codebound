package calltree

import (
	"fmt"
	"sort"
	"strings"

	"golang.org/x/tools/go/ssa"
)

// EntryPointError lets callers present ambiguous roots without parsing text.
type EntryPointError struct {
	Candidates []string
}

// EntryPoints returns stable, package-qualified names of executable mains.
func (a *Analysis) EntryPoints() []string {
	seen := make(map[string]bool)
	for function := range a.functions {
		declaration, ok := a.projectDeclaration(function)
		if !ok {
			continue
		}
		if declaration.Recv != nil || declaration.Name.Name != "main" || function.Pkg.Pkg.Name() != "main" {
			continue
		}
		seen[function.Pkg.Pkg.Path()+".main"] = true
	}
	roots := make([]string, 0, len(seen))
	for root := range seen {
		roots = append(roots, root)
	}
	sort.Strings(roots)
	return roots
}

func (a *Analysis) resolveRoot(name string) (*ssa.Function, error) {
	if name == "" {
		roots := a.EntryPoints()
		if len(roots) != 1 {
			return nil, &EntryPointError{Candidates: roots}
		}
		name = roots[0]
	}
	return a.findFunction(name)
}

func (e *EntryPointError) Error() string {
	if len(e.Candidates) == 0 {
		return "no main entry point found; specify a function with --root Type.Method or --root package.Function"
	}
	return fmt.Sprintf("multiple main entry points found; choose one with --root:\n  %s", strings.Join(e.Candidates, "\n  "))
}
