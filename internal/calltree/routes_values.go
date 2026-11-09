package calltree

import (
	"fmt"
	"go/constant"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ssa"
)

func joinRouteValues(a, b routeValue) routeValue {
	if sameRouteValue(a, b) {
		return a
	}
	return routeValue{typ: a.typ}
}

func sameRouteValue(a, b routeValue) bool {
	if a.typ != b.typ && (a.typ == nil || b.typ == nil || !types.Identical(a.typ, b.typ)) {
		return false
	}
	if a.function != b.function || a.cell != b.cell || a.text != b.text || a.textKnown != b.textKnown || a.nilValue != b.nilValue || len(a.bindings) != len(b.bindings) {
		return false
	}
	if (a.router == nil) != (b.router == nil) {
		return false
	}
	if a.router != nil && (a.router.id != b.router.id || a.router.prefix != b.router.prefix || a.router.known != b.router.known) {
		return false
	}
	for i := range a.bindings {
		if !sameRouteValue(a.bindings[i], b.bindings[i]) {
			return false
		}
	}
	return true
}

func (f *routeFrame) value(value ssa.Value) routeValue {
	if value == nil {
		return routeValue{}
	}
	if found, ok := f.values[value]; ok {
		return found
	}
	unknown := routeValue{typ: value.Type()}
	if f.evaluating[value] {
		return unknown
	}
	f.evaluating[value] = true
	defer delete(f.evaluating, value)
	result := unknown
	switch value := value.(type) {
	case *ssa.Const:
		result.nilValue = value.IsNil()
		if value.Value != nil && value.Value.Kind() == constant.String {
			result.text, result.textKnown = constant.StringVal(value.Value), true
		}
	case *ssa.Function:
		result.function = value
	case *ssa.MakeClosure:
		result.function, _ = value.Fn.(*ssa.Function)
		for _, binding := range value.Bindings {
			result.bindings = append(result.bindings, f.value(binding))
		}
	case *ssa.MakeInterface:
		result = f.value(value.X)
	case *ssa.ChangeInterface:
		result = f.value(value.X)
	case *ssa.ChangeType:
		result = f.value(value.X)
	case *ssa.Convert:
		result = f.value(value.X)
	case *ssa.Alloc:
		result.cell = &routeCell{uncertain: routeCellUncertain(value)}
		if isServeMuxType(value.Type()) {
			result.router = &routeRouter{id: fmt.Sprintf("%s@%d", f.scanner.a.sourceLocation(value.Pos()), f.id), known: true}
		}
	case *ssa.Global:
		cell := f.scanner.globals[value]
		if cell == nil {
			cell = &routeCell{uncertain: f.scanner.mutableGlobals[value]}
			if value.Pkg != nil && value.Pkg.Pkg.Path() == "net/http" && value.Name() == "DefaultServeMux" {
				cell.value = routeValue{router: &routeRouter{id: "net/http.DefaultServeMux", known: true}}
				cell.set = true
			}
			f.scanner.globals[value] = cell
		}
		result.cell = cell
		if isServeMuxType(value.Type()) {
			result.router = &routeRouter{id: value.String(), known: true}
		}
	case *ssa.UnOp:
		if value.Op == token.MUL {
			if cell := f.value(value.X).cell; cell != nil && cell.set && !cell.uncertain {
				result = cell.value
			}
		}
	case *ssa.Phi:
		if len(value.Edges) > 0 {
			result = f.value(value.Edges[0])
			for _, edge := range value.Edges[1:] {
				result = joinRouteValues(result, f.value(edge))
			}
		}
	case *ssa.BinOp:
		x, y := f.value(value.X), f.value(value.Y)
		if value.Op == token.ADD && x.textKnown && y.textKnown {
			result.text, result.textKnown = x.text+y.text, true
		}
	case *ssa.Extract:
		if values := f.results[value.Tuple]; value.Index < len(values) {
			result = values[value.Index]
		}
	}
	// Loads are not cached: a later call can refine a non-ambiguous capture cell.
	// Call results are filled by the instruction visitor, never recursively run.
	if _, load := value.(*ssa.UnOp); !load {
		f.values[value] = result
	}
	return result
}

// SSA block order is not execution order. A captured variable assigned on
// alternative paths must not appear to contain whichever handler was visited
// first. Only simple, single-initialization cells are treated as exact values.
func routeCellUncertain(address *ssa.Alloc) bool {
	references := address.Referrers()
	if references == nil {
		return true
	}
	stores := 0
	for _, reference := range *references {
		switch reference := reference.(type) {
		case *ssa.Store:
			if reference.Addr == address {
				stores++
				if stores > 1 || reference.Block() != address.Block() {
					return true
				}
			}
		case ssa.CallInstruction, *ssa.Phi, *ssa.Convert, *ssa.ChangeType:
			return true // The cell's address escapes to a possible writer/alias.
		case *ssa.MakeClosure:
			fn, ok := reference.Fn.(*ssa.Function)
			if !ok {
				return true
			}
			for i, binding := range reference.Bindings {
				if binding != address || i >= len(fn.FreeVars) {
					continue
				}
				if routeCaptureMayWrite(fn.FreeVars[i]) {
					return true
				}
			}
		}
	}
	return false
}

func routeCaptureMayWrite(value *ssa.FreeVar) bool {
	references := value.Referrers()
	if references == nil {
		return true
	}
	for _, reference := range *references {
		switch reference := reference.(type) {
		case *ssa.Store:
			if reference.Addr == value {
				return true
			}
		case ssa.CallInstruction, *ssa.MakeClosure, *ssa.Phi, *ssa.ChangeType, *ssa.Convert:
			return true
		}
	}
	return false
}
