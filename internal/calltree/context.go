package calltree

import (
	"go/constant"
	"go/token"
	"go/types"
	"time"

	"golang.org/x/tools/go/ssa"
)

// Each inspection owns its state. Loaded Analysis data stays read-only so
// separate root queries can safely run at the same time.
type contextInspector struct {
	analysis         *Analysis
	function         *ssa.Function
	owner            functionID
	info             ContextInfo
	bindings         map[*ssa.FreeVar]ssa.Value
	stores           map[ssa.Value][]ssa.Value
	escaped          map[ssa.Value]bool
	closureExecution map[*ssa.Function][]string
	origins          map[string][]ssa.Value
	sites            map[ssa.Instruction]string
}

func (a *Analysis) inspectContexts(function *ssa.Function, sites map[ssa.Instruction]string) *contextInspector {
	inspector := contextInspector{
		analysis:         a,
		function:         function,
		bindings:         make(map[*ssa.FreeVar]ssa.Value),
		stores:           make(map[ssa.Value][]ssa.Value),
		escaped:          make(map[ssa.Value]bool),
		closureExecution: make(map[*ssa.Function][]string),
		origins:          make(map[string][]ssa.Value),
		sites:            sites,
	}
	for _, parameter := range function.Params {
		if isContextType(parameter.Type()) {
			inspector.info.Parameters = append(inspector.info.Parameters, parameter.Name())
		}
	}
	inspector.collectBindings(function)
	inspector.collectStores(function)
	inspector.collectEscapes()
	inspector.inspectFunction(function, "")
	if len(inspector.info.Parameters) == 0 && len(inspector.info.Operations) == 0 {
		return nil
	}
	return &inspector
}

func (i *contextInspector) inspectFunction(function *ssa.Function, scope string) {
	for _, block := range function.Blocks {
		for _, instruction := range block.Instrs {
			switch instruction := instruction.(type) {
			case ssa.CallInstruction:
				i.inspectCall(instruction, scope)
			case *ssa.UnOp:
				if instruction.Op == token.ARROW {
					i.inspectReceive(instruction.X, instruction.Pos(), scope, "channel receive", i.sites[instruction])
				}
			case *ssa.Select:
				for _, state := range instruction.States {
					if state.Dir == types.RecvOnly {
						i.inspectReceive(state.Chan, state.Pos, scope, "select receive case", i.sites[instruction])
					}
				}
			}
		}
	}
	// Closure facts belong to the displayed enclosing function, but are marked
	// separately: declaring a closure does not prove that it will be invoked.
	for _, closure := range function.AnonFuncs {
		closureScope := "closure at " + i.location(closure.Pos())
		if modes := i.closureExecution[closure]; len(modes) > 0 {
			closureScope = joinContextOrigins(modes) + " " + closureScope
		}
		if scope != "" {
			closureScope = scope + " > " + closureScope
		}
		i.inspectFunction(closure, closureScope)
	}
}

func (i *contextInspector) inspectCall(site ssa.CallInstruction, scope string) {
	call := site.Common()
	if _, builtin := call.Value.(*ssa.Builtin); builtin {
		return
	}
	target, kind := contextConstructor(call)
	operation := ContextOperation{
		Target: callTarget(call), Location: i.location(site.Pos()), Scope: scope,
		Execution: contextExecution(site),
		Site:      i.sites[site],
	}

	if kind != "" {
		operation.Kind = kind
		operation.Context = i.createdContext(call, site.Pos())
		operation.Target = target
		if kind != "create" {
			operation.Parent = i.contextOrigin(call.Args[0])
		}
		operation.Detail = constructorDetail(target, call.Args)
		i.info.Operations = append(i.info.Operations, operation)
		return
	}

	if receiver, method := contextMethod(call); receiver != nil {
		operation.Kind = "read"
		operation.Context = i.contextOrigin(receiver)
		operation.Target = "context.Context." + method
		if method == "Done" {
			operation.Detail = "gets cancellation channel; receiving is recorded separately"
		}
		i.info.Operations = append(i.info.Operations, operation)
		return
	}

	if origin, ok := i.cancelledContext(call.Value); ok {
		operation.Kind = "cancel"
		operation.Context = origin
		operation.Target = "cancel"
		i.info.Operations = append(i.info.Operations, operation)
	}
	for _, argument := range call.Args {
		if !isContextType(argument.Type()) {
			continue
		}
		operation.Kind = "forward"
		if operation.Target == "context.Cause" {
			operation.Kind = "read"
		}
		operation.Context = i.contextOrigin(argument)
		i.info.Operations = append(i.info.Operations, operation)
	}
}

func (i *contextInspector) inspectReceive(channel ssa.Value, position token.Pos, scope, detail, site string) {
	var origins []string
	known := false
	for _, source := range i.valueSources(channel) {
		call, ok := source.(*ssa.Call)
		if ok {
			if receiver, method := contextMethod(call.Common()); receiver != nil && method == "Done" {
				origins = append(origins, i.contextOrigin(receiver))
				known = true
				continue
			}
		}
		origins = append(origins, "unresolved")
	}
	if known {
		origin := joinContextOrigins(origins)
		var values []ssa.Value
		for _, source := range origins {
			if sources, exists := i.origins[source]; exists {
				values = append(values, sources...)
			} else {
				values = append(values, nil)
			}
		}
		i.origins[origin] = values
		i.info.Operations = append(i.info.Operations, ContextOperation{
			Kind: "observe", Context: origin, Target: "context.Context.Done",
			Detail: detail, Location: i.location(position), Scope: scope, Site: site,
		})
	}
}

// Package identity, rather than spelling in the source, handles import aliases
// without confusing a user's similarly named functions with context APIs.
func contextConstructor(call *ssa.CallCommon) (target, kind string) {
	function := call.StaticCallee()
	if function == nil || function.Pkg == nil {
		return "", ""
	}
	pkg := function.Pkg.Pkg.Path()
	name := function.Name()
	if pkg == "context" {
		switch name {
		case "Background", "TODO":
			return "context." + name, "create"
		case "WithCancel", "WithCancelCause", "WithTimeout", "WithTimeoutCause", "WithDeadline", "WithDeadlineCause", "WithValue":
			return "context." + name, "derive"
		case "WithoutCancel":
			return "context." + name, "detach"
		}
	}
	if pkg == "os/signal" && name == "NotifyContext" {
		return "signal.NotifyContext", "derive"
	}
	return "", ""
}

func constructorDetail(target string, arguments []ssa.Value) string {
	switch target {
	case "context.Background", "context.TODO":
		return "independent; no cancellation or deadline"
	case "context.WithoutCancel":
		return "parent cancellation and deadline are not inherited"
	case "context.WithTimeout", "context.WithTimeoutCause":
		if duration, ok := arguments[1].(*ssa.Const); ok && duration.Value != nil {
			if nanoseconds, ok := constant.Int64Val(duration.Value); ok {
				return "timeout: " + time.Duration(nanoseconds).String()
			}
		}
		return "timeout: dynamic"
	case "context.WithDeadline", "context.WithDeadlineCause":
		return "deadline: dynamic"
	case "signal.NotifyContext":
		return "inherits parent cancellation and deadline; also cancelled by configured signals"
	default:
		return "inherits parent cancellation and deadline"
	}
}

func contextMethod(call *ssa.CallCommon) (ssa.Value, string) {
	var receiver ssa.Value
	var method string
	if call.IsInvoke() && isContextType(call.Value.Type()) {
		receiver, method = call.Value, call.Method.Name()
	} else if function := call.StaticCallee(); function != nil {
		// Method values (done := ctx.Done) and method expressions
		// (context.Context.Done(ctx)) use synthetic SSA wrappers.
		object, ok := function.Object().(*types.Func)
		if !ok {
			return nil, ""
		}
		signature := object.Type().(*types.Signature)
		if signature.Recv() == nil || !isContextType(signature.Recv().Type()) {
			return nil, ""
		}
		method = object.Name()
		if closure, ok := call.Value.(*ssa.MakeClosure); ok && len(closure.Bindings) > 0 {
			receiver = closure.Bindings[0]
		} else if len(call.Args) > 0 {
			receiver = call.Args[0]
		}
	}
	switch method {
	case "Done", "Err", "Deadline", "Value":
		return receiver, method
	}
	return nil, ""
}

func callTarget(call *ssa.CallCommon) string {
	if function := call.StaticCallee(); function != nil {
		if function.Pkg != nil && function.Signature.Recv() == nil {
			return function.Pkg.Pkg.Path() + "." + function.Name()
		}
		return function.String()
	}
	if call.IsInvoke() {
		return call.Method.FullName()
	}
	return "unresolved callback"
}

func contextExecution(site ssa.CallInstruction) string {
	switch site.(type) {
	case *ssa.Defer:
		return "deferred"
	case *ssa.Go:
		return "goroutine"
	default:
		return ""
	}
}

func isContextType(valueType types.Type) bool {
	return isContextNamedType(valueType, "Context")
}

func isContextNamedType(valueType types.Type, name string) bool {
	named, ok := types.Unalias(valueType).(*types.Named)
	return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == "context" && named.Obj().Name() == name
}

func (i *contextInspector) location(position token.Pos) string {
	return i.analysis.sourceLocation(position)
}
