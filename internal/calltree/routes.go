package calltree

import (
	"context"
	"fmt"
	"go/ast"
	"go/types"
	"path/filepath"

	"github.com/PranavJoshi2893/codebound/internal/calltree/model"
	"golang.org/x/tools/go/ssa"
)

const maxRouteFrames = 4096
const maxRouteInstructions = 200000

// Registration analysis follows actual configuration arguments, not the merged
// VTA callback graph. In particular, two Route callbacks must not exchange their
// prefixes. This deliberately bounded abstract interpretation never runs code.
type routeScanner struct {
	a                    *Analysis
	ctx                  context.Context
	descriptions         map[*ssa.Function]analyzedFunction
	registrations        []model.RouteRegistration
	warnings             []string
	seen                 map[routeRegistrationID]int
	active               map[*ssa.Function]bool
	globals              map[*ssa.Global]*routeCell
	mutableGlobals       map[*ssa.Global]bool
	initialized          map[*ssa.Function]bool
	unresolvedMounts     map[string]model.RouteOrigin
	frames, instructions int
}

type routeRegistrationID struct {
	registrar, handler                                     model.FunctionID
	framework, mux, site, method, pattern, declared, issue string
	methodKnown, patternKnown                              bool
}

type routeRouter struct {
	id, prefix string
	known      bool
	origins    []model.RouteOrigin
}

type routeValue struct {
	typ                 types.Type
	router              *routeRouter
	function            *ssa.Function
	bindings            []routeValue
	text                string
	textKnown, nilValue bool
	cell                *routeCell
}

// Local address cells handle compiler-created captures and straightforward
// aliases. Different stores join to unknown rather than choosing the last CFG
// branch. Opaque heap fields and wrappers intentionally remain unsupported.
type routeCell struct {
	value     routeValue
	set       bool
	uncertain bool
}

type routeFrame struct {
	scanner         *routeScanner
	function, owner *ssa.Function
	values          map[ssa.Value]routeValue
	results         map[ssa.Value][]routeValue
	evaluating      map[ssa.Value]bool
	id              int
}

type routeInvocation struct {
	pkg, name    string
	receiverName string
	receiver     routeValue
	args         []routeValue
}

func (a *Analysis) inspectRegistrations(ctx context.Context, root *ssa.Function, descriptions map[*ssa.Function]analyzedFunction, graph *Graph) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	scanner := &routeScanner{
		a: a, ctx: ctx, descriptions: descriptions,
		seen: make(map[routeRegistrationID]int), active: make(map[*ssa.Function]bool),
		globals: make(map[*ssa.Global]*routeCell), initialized: make(map[*ssa.Function]bool),
		mutableGlobals:   make(map[*ssa.Global]bool),
		unresolvedMounts: make(map[string]model.RouteOrigin),
	}
	found := false
	for fn := range a.functions {
		if err := ctx.Err(); err != nil {
			return err
		}
		if scanner.projectFunction(fn) {
			found = found || hasRouteInstructions(fn)
			// Do not choose whichever global assignment appears first in SSA
			// block order. Non-initializer writes require a heap/CFG analysis;
			// expose them as unknown instead of inventing an exact binding.
			if !routeInitializer(fn) {
				for _, block := range fn.Blocks {
					for _, instruction := range block.Instrs {
						if store, ok := instruction.(*ssa.Store); ok {
							if global, ok := store.Addr.(*ssa.Global); ok {
								scanner.mutableGlobals[global] = true
							}
						}
					}
				}
			}
		}
	}
	if !found {
		return ctx.Err()
	}
	// Executable initialization precedes main, but is not a made-up main -> init
	// call. Explicit library/function roots keep their existing root-local scope.
	if root.Name() == "main" && root.Pkg.Pkg.Name() == "main" {
		scanner.visit(root.Pkg.Func("init"), nil, nil, root)
	}
	scanner.visit(root, nil, nil, root)
	for i := range scanner.registrations {
		route := &scanner.registrations[i]
		if origin, mounted := scanner.unresolvedMounts[route.Mux]; mounted {
			route.PatternKnown = false
			route.Issue = appendRouteIssue(route.Issue, "mounted router prefix unresolved (Mount is not expanded)")
			route.Provenance = append(append([]model.RouteOrigin(nil), route.Provenance...), origin)
		}
	}
	graph.Registrations, graph.RegistrationWarnings = scanner.registrations, scanner.warnings
	return ctx.Err()
}

func (s *routeScanner) warn(message string) {
	for _, previous := range s.warnings {
		if previous == message {
			return
		}
	}
	s.warnings = append(s.warnings, message)
}

func (s *routeScanner) projectFunction(fn *ssa.Function) bool {
	return fn != nil && fn.Pkg != nil && s.a.projectPackages[fn.Pkg.Pkg.Path()]
}

func (s *routeScanner) description(fn *ssa.Function) analyzedFunction {
	if info, ok := s.descriptions[fn]; ok {
		return info
	}
	_, inline := fn.Syntax().(*ast.FuncLit)
	if routeInitializer(fn) || inline {
		description := model.Function{Name: "package initialization", Package: fn.Pkg.Pkg.Path()}
		if inline {
			description.Name = "inline handler (" + s.a.sourceLocation(fn.Pos()) + ")"
		}
		if inline && fn.Pos().IsValid() {
			position := s.a.program.Fset.Position(fn.Pos())
			description.File, _ = filepath.Rel(s.a.dir, position.Filename)
			description.Line = position.Line
		}
		for _, block := range fn.Blocks {
			for _, instruction := range block.Instrs {
				if instruction.Pos().IsValid() && description.File == "" {
					position := s.a.program.Fset.Position(instruction.Pos())
					description.File, _ = filepath.Rel(s.a.dir, position.Filename)
					description.Line = position.Line
				}
			}
		}
		flow, sites := s.a.inspectExecution(fn)
		description.Flow = flow
		info := analyzedFunction{description: description, sites: sites}
		s.descriptions[fn] = info
		return info
	}
	info := s.a.analyzeFunction(fn)
	s.descriptions[fn] = info
	return info
}

func (s *routeScanner) visit(fn *ssa.Function, args, bindings []routeValue, owner *ssa.Function) []routeValue {
	// Bound configuration methods are transparent compiler adapters, not opaque
	// middleware. Bind the actual receiver before interpreting their declaration.
	if fn != nil && !s.projectFunction(fn) && fn.Synthetic != "" {
		if target := s.handler(routeValue{function: fn}); target != nil {
			if len(args) != len(target.Params) {
				args = append(append([]routeValue(nil), bindings...), args...)
			}
			if len(args) == len(target.Params) {
				fn, bindings = target, nil
			}
		}
	}
	if !s.projectFunction(fn) || s.ctx.Err() != nil {
		return nil
	}
	if routeInitializer(fn) {
		if s.initialized[fn] {
			return nil
		}
		s.initialized[fn] = true
		owner = fn
	}
	if s.active[fn] {
		relevant := hasRouteInstructions(fn)
		for _, arg := range args {
			relevant = relevant || arg.router != nil
		}
		if relevant {
			s.warn("Route registration analysis is partial: recursive router configuration was not expanded.")
		}
		return nil
	}
	if s.frames >= maxRouteFrames || s.instructions >= maxRouteInstructions {
		s.warn("Route registration analysis is partial: recursion or the configuration analysis limit was reached.")
		return nil
	}
	if _, ok := s.a.projectDeclaration(fn); ok {
		owner = fn
	}
	if _, ok := s.a.projectDeclaration(owner); !ok && !routeInitializer(owner) {
		return nil
	}
	s.frames++
	s.active[fn] = true
	defer delete(s.active, fn)
	f := &routeFrame{scanner: s, function: fn, owner: owner, values: make(map[ssa.Value]routeValue), results: make(map[ssa.Value][]routeValue), evaluating: make(map[ssa.Value]bool), id: s.frames}
	for i, parameter := range fn.Params {
		value := routeValue{typ: parameter.Type()}
		if i < len(args) {
			value = args[i]
		}
		f.values[parameter] = value
	}
	for i, free := range fn.FreeVars {
		if i < len(bindings) {
			f.values[free] = bindings[i]
		}
	}
	var returned []routeValue
	for _, block := range fn.Blocks {
		for _, instruction := range block.Instrs {
			s.instructions++
			if s.ctx.Err() != nil {
				return nil
			}
			if s.instructions > maxRouteInstructions {
				s.warn("Route registration analysis is partial: the configuration instruction limit was reached.")
				return returned
			}
			switch instruction := instruction.(type) {
			case *ssa.Store:
				address := f.value(instruction.Addr)
				if address.cell != nil {
					value := f.value(instruction.Val)
					if address.cell.set {
						value = joinRouteValues(address.cell.value, value)
					}
					address.cell.value, address.cell.set = value, true
				}
			case ssa.CallInstruction:
				results := f.call(instruction)
				if value := instruction.Value(); value != nil {
					f.results[value] = results
					if len(results) == 1 {
						f.values[value] = results[0]
					}
				}
			case *ssa.Return:
				for i, result := range instruction.Results {
					value := f.value(result)
					if i >= len(returned) {
						returned = append(returned, value)
					} else {
						returned[i] = joinRouteValues(returned[i], value)
					}
				}
			}
		}
	}
	return returned
}

func (f *routeFrame) call(site ssa.CallInstruction) []routeValue {
	common := site.Common()
	var args []routeValue
	for _, argument := range common.Args {
		args = append(args, f.value(argument))
	}
	callee := common.StaticCallee()
	function := f.value(common.Value)
	if callee == nil {
		callee = function.function
	}
	invocation := routeInvocation{args: args}
	var object *types.Func
	if common.IsInvoke() {
		object = common.Method
		invocation.receiver = f.value(common.Value)
		if invocation.receiver.typ != nil && !isRouteInterface(invocation.receiver.typ) {
			callee = f.scanner.lookupMethod(invocation.receiver.typ, object.Pkg(), object.Name())
			args = append([]routeValue{invocation.receiver}, args...)
			if callee != nil {
				object, _ = callee.Object().(*types.Func)
			}
		}
	} else if callee != nil {
		object, _ = callee.Object().(*types.Func)
		if callee.Signature.Recv() != nil && len(args) > 0 {
			invocation.receiver, invocation.args = args[0], args[1:]
		}
	}
	if object != nil && object.Pkg() != nil {
		invocation.pkg, invocation.name = object.Pkg().Path(), object.Name()
	}
	invocation.receiverName = routeReceiverName(common, callee)
	if results, recognized := f.chiCall(site, invocation); recognized {
		return results
	}
	if results, recognized := f.serveMuxCall(site, invocation); recognized {
		return results
	}
	if f.scanner.projectFunction(callee) {
		return f.scanner.visit(callee, args, function.bindings, f.owner)
	}
	if callee != nil && callee.Synthetic != "" {
		if f.scanner.handler(routeValue{function: callee}) != nil {
			return f.scanner.visit(callee, args, function.bindings, f.owner)
		}
		if hasRouteInstructions(callee) {
			f.scanner.warn("Registration through a synthetic API adapter was not resolved; some registrations may be missing.")
		}
	}
	if common.IsInvoke() && object != nil && object.Pkg() != nil && f.scanner.a.projectPackages[object.Pkg().Path()] {
		f.scanner.warn("Route registration analysis is partial: an interface configuration call could not be resolved.")
	}
	var results []routeValue
	for i := 0; i < common.Signature().Results().Len(); i++ {
		results = append(results, routeValue{typ: common.Signature().Results().At(i).Type()})
	}
	return results
}

func isRouteInterface(typ types.Type) bool {
	_, ok := typ.Underlying().(*types.Interface)
	return ok
}

func (f *routeFrame) newRouter(site ssa.CallInstruction) routeValue {
	return routeValue{typ: site.Common().Signature().Results().At(0).Type(), router: &routeRouter{id: fmt.Sprintf("%s@%d", f.scanner.a.sourceLocation(site.Pos()), f.id), known: true}}
}

// Synthetic bound-method wrappers have one project declaration target. Opaque
// adapters with multiple possible targets are not flattened into a guessed one.
func (s *routeScanner) handler(value routeValue) *ssa.Function {
	fn := value.function
	if fn == nil && value.typ != nil && !value.nilValue && !isRouteInterface(value.typ) {
		fn = s.lookupMethod(value.typ, nil, "ServeHTTP")
	}
	for depth := 0; fn != nil && depth < 8; depth++ {
		if _, ok := s.a.projectDeclaration(fn); ok {
			return fn
		}
		if _, inline := fn.Syntax().(*ast.FuncLit); inline && s.projectFunction(fn) {
			return fn
		}
		if fn.Synthetic == "" {
			return nil
		}
		var next *ssa.Function
		for _, block := range fn.Blocks {
			for _, instruction := range block.Instrs {
				if call, ok := instruction.(ssa.CallInstruction); ok {
					candidate := call.Common().StaticCallee()
					if candidate == nil || next != nil && next != candidate {
						return nil
					}
					next = candidate
				}
			}
		}
		fn = next
	}
	return nil
}

func (f *routeFrame) register(site ssa.CallInstruction, route model.RouteRegistration, handler routeValue) {
	s := f.scanner
	info := s.description(f.owner)
	route.Registrar, route.Site, route.Location = info.description, info.sites[site], s.a.sourceLocation(site.Pos())
	if target := s.handler(handler); target != nil {
		description := s.description(target).description
		route.Handler = &description
	} else if handler.nilValue {
		route.Issue = appendRouteIssue(route.Issue, "nil handler")
	} else {
		route.Issue = appendRouteIssue(route.Issue, "handler unresolved")
	}
	var target model.FunctionID
	if route.Handler != nil {
		target = route.Handler.Identity()
	}
	key := routeRegistrationID{registrar: route.Registrar.Identity(), handler: target, framework: route.Framework, mux: route.Mux, site: route.Site, method: route.Method, pattern: route.Pattern, declared: route.DeclaredPattern, issue: route.Issue, methodKnown: route.MethodKnown, patternKnown: route.PatternKnown}
	if id, duplicate := s.seen[key]; duplicate {
		previous := &s.registrations[id]
		for _, origin := range route.Provenance {
			found := false
			for _, known := range previous.Provenance {
				found = found || known == origin
			}
			if !found {
				previous.Provenance = append(previous.Provenance, origin)
			}
		}
		return
	}
	s.seen[key] = len(s.registrations)
	route.Provenance = append([]model.RouteOrigin(nil), route.Provenance...)
	s.registrations = append(s.registrations, route)
}

func appendRouteIssue(previous, issue string) string {
	if previous == "" {
		return issue
	}
	return previous + "; " + issue
}

func (s *routeScanner) lookupMethod(typ types.Type, pkg *types.Package, name string) *ssa.Function {
	object, _, _ := types.LookupFieldOrMethod(typ, true, pkg, name)
	if _, ok := object.(*types.Func); !ok {
		return nil
	}
	return s.a.program.LookupMethod(typ, pkg, name)
}

func routeReceiverName(common *ssa.CallCommon, callee *ssa.Function) string {
	var typ types.Type
	if callee != nil && callee.Signature.Recv() != nil {
		typ = callee.Signature.Recv().Type()
	} else if common.IsInvoke() {
		typ = common.Value.Type()
	}
	if typ == nil {
		return ""
	}
	typ = types.Unalias(typ)
	if pointer, ok := typ.(*types.Pointer); ok {
		typ = types.Unalias(pointer.Elem())
	}
	if named, ok := typ.(*types.Named); ok {
		return named.Obj().Name()
	}
	return ""
}

func routeInitializer(fn *ssa.Function) bool {
	return fn != nil && fn.Name() == "init" && fn.Synthetic == "package initializer"
}

func hasRouteInstructions(fn *ssa.Function) bool {
	for _, block := range fn.Blocks {
		for _, instruction := range block.Instrs {
			call, ok := instruction.(ssa.CallInstruction)
			if !ok {
				continue
			}
			var object *types.Func
			if call.Common().IsInvoke() {
				object = call.Common().Method
			} else if target := call.Common().StaticCallee(); target != nil {
				object, _ = target.Object().(*types.Func)
			}
			if object == nil || object.Pkg() == nil {
				continue
			}
			// This is only a cheap pass-enablement check. A user-defined
			// interface can hide the API declaration; actual recognition still
			// requires resolving its concrete package/type/method identity.
			if call.Common().IsInvoke() && (object.Name() == "Handle" || object.Name() == "HandleFunc") {
				return true
			}
			switch object.Pkg().Path() {
			case "net/http":
				if object.Name() == "Handle" || object.Name() == "HandleFunc" {
					return true
				}
			case chiPackage:
				return true
			}
		}
	}
	return false
}
