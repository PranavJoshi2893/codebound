package calltree

import (
	"fmt"
	"go/ast"
	"path/filepath"

	"golang.org/x/tools/go/ssa"
)

type analyzedFunction struct {
	description Function
	contexts    *contextInspector
}

func (a *Analysis) analyzeFunction(function *ssa.Function) analyzedFunction {
	description := a.describeFunction(function)
	contexts := a.inspectContexts(function)
	if contexts != nil {
		contexts.owner = description.Identity()
		description.Context = &contexts.info
	}
	return analyzedFunction{description: description, contexts: contexts}
}

func (a *Analysis) findFunction(name string) (*ssa.Function, error) {
	var match *ssa.Function
	for function := range a.functions {
		declaration, ok := a.projectDeclaration(function)
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

// Anonymous functions, synthetic wrappers, and dependency functions are not
// displayed. A false result means "not displayed", not an analysis failure.
func (a *Analysis) projectDeclaration(function *ssa.Function) (*ast.FuncDecl, bool) {
	if function == nil || function.Pkg == nil || !a.projectPackages[function.Pkg.Pkg.Path()] {
		return nil, false
	}
	declaration, ok := function.Syntax().(*ast.FuncDecl)
	return declaration, ok
}

// Only projectDeclaration-checked functions reach this conversion.
func (a *Analysis) describeFunction(function *ssa.Function) Function {
	declaration := function.Syntax().(*ast.FuncDecl)
	position := a.program.Fset.Position(declaration.Pos())
	file := position.Filename
	if relative, err := filepath.Rel(a.dir, file); err == nil {
		file = relative
	}
	return Function{
		Name:    functionName(declaration),
		Package: function.Pkg.Pkg.Path(),
		File:    file,
		Line:    position.Line,
		Layer:   inferLayer(function.Pkg.Pkg.Path(), file),
	}
}

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
