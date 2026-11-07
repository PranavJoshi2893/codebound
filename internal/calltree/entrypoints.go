package calltree

import (
	"fmt"
	"sort"
	"strings"
)

// EntryPointError lets callers present ambiguous roots without parsing text.
type EntryPointError struct {
	Candidates []string
}

func (e *EntryPointError) Error() string {
	if len(e.Candidates) == 0 {
		return "no main entry point found; specify a function with --root Type.Method or --root package.Function"
	}
	return fmt.Sprintf("multiple main entry points found; choose one with --root:\n  %s", strings.Join(e.Candidates, "\n  "))
}

// EntryPoints returns stable, package-qualified names of executable mains.
func (a *Analysis) EntryPoints() []string {
	seen := make(map[string]bool)
	for function := range a.functions {
		declaration, ok := projectDeclaration(function, a.project)
		if !ok || declaration.Recv != nil || declaration.Name.Name != "main" || function.Pkg.Pkg.Name() != "main" {
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
