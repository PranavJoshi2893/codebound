package calltree

import "strings"

// Layer detection is a naming heuristic, not a configurable architecture policy.
func inferLayer(pkgPath, file string) string {
	parts := strings.FieldsFunc(strings.ToLower(pkgPath+"/"+file), func(r rune) bool {
		return r == '/' || r == '\\' || r == '_' || r == '-' || r == '.'
	})
	for _, part := range parts {
		switch part {
		case "handler", "handlers", "controller", "controllers":
			return "handler"
		case "service", "services", "usecase", "usecases":
			return "service"
		case "repository", "repositories", "repo", "repos", "store", "storage":
			return "repository"
		}
	}
	return ""
}

func boundaryViolation(caller, callee Function, throughInterface bool) string {
	if caller.Layer == "service" && callee.Layer == "service" && caller.Package != callee.Package {
		if throughInterface {
			return ""
		}
		return "cross-service call must use an interface"
	}
	if caller.Layer == "" || callee.Layer == "" || caller.Layer == callee.Layer {
		return ""
	}
	if (caller.Layer == "handler" && callee.Layer == "service") ||
		(caller.Layer == "service" && callee.Layer == "repository") {
		return ""
	}
	return caller.Layer + " must not call " + callee.Layer
}
