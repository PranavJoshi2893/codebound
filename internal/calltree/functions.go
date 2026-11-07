package calltree

import (
	"fmt"
	"go/ast"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

func functionName(function *ast.FuncDecl) string {
	if function.Recv == nil {
		return function.Name.Name
	}
	var receiver ast.Expr = function.Recv.List[0].Type
	for {
		switch value := receiver.(type) {
		case *ast.StarExpr:
			receiver = value.X
		case *ast.IndexExpr:
			receiver = value.X
		case *ast.IndexListExpr:
			receiver = value.X
		case *ast.Ident:
			return value.Name + "." + function.Name.Name
		default:
			return function.Name.Name
		}
	}
}

func projectPackages(loaded []*packages.Package) map[string]bool {
	modules := make(map[string]bool)
	for _, pkg := range loaded {
		if pkg.Module != nil {
			modules[pkg.Module.Path] = true
		}
	}
	project := make(map[string]bool)
	packages.Visit(loaded, func(pkg *packages.Package) bool {
		if pkg.Module != nil && modules[pkg.Module.Path] {
			project[pkg.PkgPath] = true
		}
		return true
	}, nil)
	return project
}

func projectDeclaration(function *ssa.Function, project map[string]bool) (*ast.FuncDecl, bool) {
	if function == nil || function.Pkg == nil || !project[function.Pkg.Pkg.Path()] {
		return nil, false
	}
	declaration, ok := function.Syntax().(*ast.FuncDecl)
	return declaration, ok
}

func findFunction(functions map[*ssa.Function]bool, project map[string]bool, name string) (*ssa.Function, error) {
	var match *ssa.Function
	for function := range functions {
		declaration, ok := projectDeclaration(function, project)
		if !ok {
			continue
		}
		shortName := functionName(declaration)
		if name != shortName && name != function.Pkg.Pkg.Path()+"."+shortName {
			continue
		}
		if match != nil {
			return nil, fmt.Errorf("function %q is ambiguous; use a package-qualified name", name)
		}
		match = function
	}
	if match == nil {
		return nil, fmt.Errorf("function %q not found", name)
	}
	return match, nil
}
