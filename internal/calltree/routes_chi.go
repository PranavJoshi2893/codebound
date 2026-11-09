package calltree

import (
	"github.com/PranavJoshi2893/codebound/internal/calltree/model"
	"golang.org/x/tools/go/ssa"
	"strings"
)

const chiPackage = "github.com/go-chi/chi/v5"

func composeChiPattern(prefix, pattern string) string {
	if prefix == "" {
		return pattern
	}
	return strings.TrimSuffix(prefix, "/") + pattern
}

func (f *routeFrame) chiCall(site ssa.CallInstruction, call routeInvocation) ([]routeValue, bool) {
	if call.pkg != chiPackage {
		return nil, false
	}
	if call.name == "NewRouter" || call.name == "NewMux" {
		return []routeValue{f.newRouter(site)}, true
	}
	receiver := call.receiverName
	if receiver != "Router" && receiver != "Mux" {
		return nil, false
	}
	router := call.receiver.router
	if router == nil {
		router = &routeRouter{id: "unknown Chi router"}
	}
	switch call.name {
	case "Route", "Group":
		child := *router
		callback := 0
		if call.name == "Route" {
			if len(call.args) < 2 {
				return nil, true
			}
			pattern := call.args[0]
			child.prefix = composeChiPattern(router.prefix, pattern.text)
			child.known = router.known && pattern.textKnown
			child.origins = append(append([]model.RouteOrigin(nil), router.origins...), model.RouteOrigin{Location: f.scanner.a.sourceLocation(site.Pos()), Pattern: pattern.text})
			callback = 1
		}
		if len(call.args) > callback {
			fn := call.args[callback]
			if fn.function != nil {
				f.scanner.visit(fn.function, []routeValue{{router: &child}}, fn.bindings, f.owner)
			} else {
				f.scanner.warn("Chi configuration callback unresolved; some route registrations may be missing.")
			}
		}
		return []routeValue{{router: &child}}, true
	case "With":
		return []routeValue{call.receiver}, true
	case "Use":
		return nil, true
	case "Mount":
		f.scanner.warn("Chi Mount is not expanded; registrations inside opaque mounted handlers may be missing.")
		if len(call.args) > 1 && call.args[1].router != nil {
			f.scanner.unresolvedMounts[call.args[1].router.id] = model.RouteOrigin{Location: f.scanner.a.sourceLocation(site.Pos()), Pattern: call.args[0].text}
		}
		return nil, true
	}
	method, patternIndex, handlerIndex := "", 0, 1
	methodKnown := true
	switch call.name {
	case "Get":
		method = "GET"
	case "Post":
		method = "POST"
	case "Put":
		method = "PUT"
	case "Patch":
		method = "PATCH"
	case "Delete":
		method = "DELETE"
	case "Head":
		method = "HEAD"
	case "Options":
		method = "OPTIONS"
	case "Connect":
		method = "CONNECT"
	case "Trace":
		method = "TRACE"
	case "Handle", "HandleFunc":
		method = "*"
	case "Method", "MethodFunc":
		if len(call.args) < 3 {
			return nil, true
		}
		method, methodKnown, patternIndex, handlerIndex = call.args[0].text, call.args[0].textKnown, 1, 2
	default:
		return nil, false
	}
	if len(call.args) <= handlerIndex {
		return nil, true
	}
	pattern := call.args[patternIndex]
	route := model.RouteRegistration{Framework: "chi", Mux: router.id, Method: method, MethodKnown: methodKnown, DeclaredPattern: pattern.text, Pattern: composeChiPattern(router.prefix, pattern.text), PatternKnown: router.known && pattern.textKnown, Provenance: router.origins}
	if !route.PatternKnown {
		route.Issue = "pattern or router prefix unresolved"
	}
	if !methodKnown {
		route.Issue = appendRouteIssue(route.Issue, "method unresolved")
	}
	f.register(site, route, call.args[handlerIndex])
	return nil, true
}
