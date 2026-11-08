package calltree

import (
	"go/token"
	"sort"
	"strings"

	"golang.org/x/tools/go/ssa"
)

// Record captures and direct invocation modes before inspecting any closure.
func (i *contextInspector) collectBindings(function *ssa.Function) {
	for _, block := range function.Blocks {
		for _, instruction := range block.Instrs {
			if closure, ok := instruction.(*ssa.MakeClosure); ok {
				for index, variable := range closure.Fn.(*ssa.Function).FreeVars {
					i.bindings[variable] = closure.Bindings[index]
				}
			}
			if site, ok := instruction.(ssa.CallInstruction); ok {
				if closure := site.Common().StaticCallee(); closure != nil && closure.Parent() != nil {
					mode := contextExecution(site)
					if mode == "" {
						mode = "direct"
					}
					i.closureExecution[closure] = append(i.closureExecution[closure], mode)
				}
			}
		}
	}
	for _, closure := range function.AnonFuncs {
		i.collectBindings(closure)
	}
}

func (i *contextInspector) collectStores(function *ssa.Function) {
	for _, block := range function.Blocks {
		for _, instruction := range block.Instrs {
			if store, ok := instruction.(*ssa.Store); ok {
				address := i.captureAddress(store.Addr)
				if _, local := address.(*ssa.Alloc); local {
					i.stores[address] = append(i.stores[address], store.Val)
				}
			}
		}
	}
	for _, closure := range function.AnonFuncs {
		i.collectStores(closure)
	}
}

func (i *contextInspector) captureAddress(value ssa.Value) ssa.Value {
	for {
		variable, ok := value.(*ssa.FreeVar)
		if !ok || i.bindings[variable] == nil {
			return value
		}
		value = i.bindings[variable]
	}
}

// A helper receiving &ctx can change ctx outside our local inspection. Keep
// known local stores, but add an unresolved possibility instead of guessing.
func (i *contextInspector) collectEscapes() {
	for address := range i.stores {
		i.escaped[address] = hasUnknownPointerUse(address)
	}
	for variable := range i.bindings {
		address := i.captureAddress(variable)
		if _, local := address.(*ssa.Alloc); local && hasUnknownPointerUse(variable) {
			i.escaped[address] = true
		}
	}
}

func hasUnknownPointerUse(value ssa.Value) bool {
	references := value.Referrers()
	if references == nil {
		return true
	}
	for _, reference := range *references {
		switch reference := reference.(type) {
		case *ssa.Store:
			if reference.Addr != value {
				return true
			}
		case *ssa.UnOp:
			if reference.Op != token.MUL {
				return true
			}
		case *ssa.MakeClosure:
			if reference.Fn.(*ssa.Function).Parent() == nil {
				return true
			}
		case *ssa.DebugRef:
		default:
			return true
		}
	}
	return false
}

// Follow local assignments, conversions, branch merges, and captured variables.
// This is deliberately not a second whole-program analysis: fields, globals,
// and results from custom wrappers remain unresolved. All possible local stores
// are considered; their order and branch conditions are not interpreted.
func (i *contextInspector) valueSources(value ssa.Value) []ssa.Value {
	var sources []ssa.Value
	pending := []ssa.Value{value}
	seen := make(map[ssa.Value]bool)
	for len(pending) > 0 {
		value = pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if value == nil || seen[value] {
			continue
		}
		seen[value] = true
		switch value := value.(type) {
		case *ssa.Phi:
			pending = append(pending, value.Edges...)
		case *ssa.ChangeInterface:
			pending = append(pending, value.X)
		case *ssa.MakeInterface:
			pending = append(pending, value.X)
		case *ssa.ChangeType:
			pending = append(pending, value.X)
		case *ssa.Convert:
			pending = append(pending, value.X)
		case *ssa.FreeVar:
			if binding := i.bindings[value]; binding != nil {
				pending = append(pending, binding)
			} else {
				sources = append(sources, value)
			}
		case *ssa.UnOp:
			address := i.captureAddress(value.X)
			stores := i.stores[address]
			if value.Op == token.MUL && len(stores) > 0 {
				pending = append(pending, stores...)
				if i.escaped[address] {
					sources = append(sources, value)
				}
			} else {
				sources = append(sources, value)
			}
		case *ssa.Alloc:
			if stores := i.stores[value]; len(stores) > 0 {
				pending = append(pending, stores...)
				if i.escaped[value] {
					sources = append(sources, value)
				}
			} else {
				sources = append(sources, value)
			}
		default:
			sources = append(sources, value)
		}
	}
	return sources
}

func (i *contextInspector) contextOrigin(value ssa.Value) string {
	var origins []string
	for _, source := range i.valueSources(value) {
		switch source := source.(type) {
		case *ssa.Parameter:
			origin := "parameter " + source.Name()
			if source.Parent() != i.function {
				origin += " (closure at " + i.location(source.Parent().Pos()) + ")"
			}
			origins = append(origins, origin)
		case *ssa.Call:
			if _, kind := contextConstructor(source.Common()); kind != "" {
				origins = append(origins, i.createdContext(source.Common(), source.Pos()))
			} else {
				origins = append(origins, "unresolved return from "+callTarget(source.Common()))
			}
		case *ssa.Extract:
			call, ok := source.Tuple.(*ssa.Call)
			if ok && source.Index == 0 {
				if _, kind := contextConstructor(call.Common()); kind != "" {
					origins = append(origins, i.createdContext(call.Common(), call.Pos()))
					continue
				}
			}
			origins = append(origins, "unresolved")
		case *ssa.Const:
			if source.IsNil() {
				origins = append(origins, "nil")
			} else {
				origins = append(origins, "unresolved")
			}
		default:
			origins = append(origins, "unresolved")
		}
	}
	return joinContextOrigins(origins)
}

func (i *contextInspector) cancelledContext(value ssa.Value) (string, bool) {
	known := isContextNamedType(value.Type(), "CancelFunc") || isContextNamedType(value.Type(), "CancelCauseFunc")
	var origins []string
	for _, source := range i.valueSources(value) {
		if extract, ok := source.(*ssa.Extract); ok && extract.Index == 1 {
			if call, ok := extract.Tuple.(*ssa.Call); ok {
				if _, kind := contextConstructor(call.Common()); kind == "derive" {
					origins = append(origins, i.createdContext(call.Common(), call.Pos()))
					known = true
					continue
				}
			}
		}
		origins = append(origins, "unresolved")
	}
	return joinContextOrigins(origins), known
}

func (i *contextInspector) createdContext(call *ssa.CallCommon, position token.Pos) string {
	target, _ := contextConstructor(call)
	return target + " at " + i.location(position)
}

func joinContextOrigins(origins []string) string {
	unique := make(map[string]bool)
	for _, origin := range origins {
		unique[origin] = true
	}
	if len(unique) == 0 {
		return "unresolved"
	}
	ordered := make([]string, 0, len(unique))
	for origin := range unique {
		ordered = append(ordered, origin)
	}
	sort.Strings(ordered)
	return strings.Join(ordered, " or ")
}
