package calltree

import (
	"go/types"
	"strings"

	"github.com/PranavJoshi2893/codebound/internal/calltree/model"
	"golang.org/x/tools/go/ssa"
)

func isServeMuxType(typ types.Type) bool {
	if typ == nil {
		return false
	}
	typ = types.Unalias(typ)
	if pointer, ok := typ.(*types.Pointer); ok {
		typ = types.Unalias(pointer.Elem())
	}
	named, ok := typ.(*types.Named)
	return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == "net/http" && named.Obj().Name() == "ServeMux"
}

func (f *routeFrame) serveMuxCall(site ssa.CallInstruction, call routeInvocation) ([]routeValue, bool) {
	if call.pkg != "net/http" {
		return nil, false
	}
	if call.name == "NewServeMux" {
		return []routeValue{f.newRouter(site)}, true
	}
	if call.name != "Handle" && call.name != "HandleFunc" {
		return nil, false
	}
	receiver := call.receiverName
	if receiver != "" && receiver != "ServeMux" {
		return nil, false
	}
	if len(call.args) != 2 {
		return nil, true
	}
	mux := call.receiver.router
	if receiver == "" {
		if pkg := f.scanner.a.program.ImportedPackage("net/http"); pkg != nil {
			if global, ok := pkg.Members["DefaultServeMux"].(*ssa.Global); ok {
				if cell := f.value(global).cell; cell != nil && cell.set && !cell.uncertain {
					mux = cell.value.router
				}
			}
		}
	}
	route := model.RouteRegistration{Framework: "net/http", Mux: "unknown ServeMux", DeclaredPattern: call.args[0].text, PatternKnown: call.args[0].textKnown, MethodKnown: call.args[0].textKnown, Method: "*"}
	if mux != nil {
		route.Mux = mux.id
	} else {
		route.Issue = "router instance unresolved"
	}
	if route.PatternKnown {
		pattern := route.DeclaredPattern
		if split := strings.IndexAny(pattern, " \t"); split >= 0 {
			route.Method, pattern = pattern[:split], strings.TrimLeft(pattern[split+1:], " \t")
		}
		route.Pattern = pattern
		if slash := strings.IndexByte(pattern, '/'); slash > 0 {
			route.Host = pattern[:slash]
		}
	} else {
		route.Issue = appendRouteIssue(route.Issue, "pattern unresolved")
	}
	f.register(site, route, call.args[1])
	return nil, true
}
