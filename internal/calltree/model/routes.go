package model

// RouteRegistration describes configuration, not invocation or request flow.
// Pattern is the composed path (including a ServeMux host qualifier); the exact
// registration argument is retained separately. Unknown facts are never guessed.
type RouteRegistration struct {
	Registrar       Function      `json:"registrar"`
	Handler         *Function     `json:"handler,omitempty"`
	Framework       string        `json:"framework"`
	Mux             string        `json:"mux"`
	Site            string        `json:"site,omitempty"`
	Location        string        `json:"location"`
	Method          string        `json:"method,omitempty"` // "*" means all methods.
	MethodKnown     bool          `json:"method_known"`
	DeclaredPattern string        `json:"declared_pattern,omitempty"`
	Pattern         string        `json:"pattern,omitempty"`
	PatternKnown    bool          `json:"pattern_known"`
	Host            string        `json:"host,omitempty"`
	Issue           string        `json:"issue,omitempty"`
	Provenance      []RouteOrigin `json:"provenance,omitempty"`
}

type RouteOrigin struct {
	Location string `json:"location"`
	Pattern  string `json:"pattern,omitempty"`
}

// Label is shared by presentation adapters; it is not executable markup.
func (route RouteRegistration) Label() string {
	method := route.Method
	if !route.MethodKnown {
		method = "unknown method"
	} else if method == "*" {
		method = "ALL"
	}
	pattern := route.Pattern
	if !route.PatternKnown {
		pattern = "unknown pattern"
		if route.DeclaredPattern != "" {
			pattern = route.DeclaredPattern + " (prefix unknown)"
		}
	}
	return method + " " + pattern
}
