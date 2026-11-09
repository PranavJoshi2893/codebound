package viewer

import (
	"github.com/PranavJoshi2893/codebound/internal/calltree/diagram"
	"github.com/PranavJoshi2893/codebound/internal/calltree/model"
)

// Send each function's details once, even when it has many callers.
type graphData struct {
	Functions            []model.Function    `json:"functions"`
	Calls                []callData          `json:"calls"`
	Evidence             *model.CallEvidence `json:"evidence,omitempty"`
	Registrations        []registrationData  `json:"registrations,omitempty"`
	RegistrationWarnings []string            `json:"registration_warnings,omitempty"`
	RegisteredOnly       []int               `json:"registered_only,omitempty"`
}

type registrationData struct {
	From            int                 `json:"from"`
	To              int                 `json:"to"`
	Label           string              `json:"label"`
	Framework       string              `json:"framework"`
	Mux             string              `json:"mux"`
	Site            string              `json:"site,omitempty"`
	Location        string              `json:"location"`
	Method          string              `json:"method,omitempty"`
	MethodKnown     bool                `json:"method_known"`
	DeclaredPattern string              `json:"declared_pattern,omitempty"`
	Pattern         string              `json:"pattern,omitempty"`
	PatternKnown    bool                `json:"pattern_known"`
	Host            string              `json:"host,omitempty"`
	Issue           string              `json:"issue,omitempty"`
	Provenance      []model.RouteOrigin `json:"provenance,omitempty"`
}

type callData struct {
	From             int                    `json:"from"`
	To               int                    `json:"to"`
	ThroughInterface bool                   `json:"through_interface,omitempty"`
	Dynamic          bool                   `json:"dynamic,omitempty"`
	Violation        string                 `json:"violation,omitempty"`
	Contexts         []model.ContextBinding `json:"contexts,omitempty"`
	Execution        string                 `json:"execution,omitempty"`
	Site             string                 `json:"site,omitempty"`
	Path             *model.CallPath        `json:"path,omitempty"`
}

func describeGraph(index *diagram.IndexedGraph) graphData {
	data := graphData{Functions: []model.Function{}, Calls: []callData{}}
	if len(index.Functions) == 0 {
		return data
	}
	data.Functions = index.Functions
	data.Evidence = index.Evidence()
	data.RegistrationWarnings = index.RegistrationWarnings()
	for id := range index.Functions {
		if index.RegistrationOnly(id) {
			data.RegisteredOnly = append(data.RegisteredOnly, id)
		}
	}
	for _, indexed := range index.Registrations {
		route := indexed.Registration
		data.Registrations = append(data.Registrations, registrationData{From: indexed.From, To: indexed.To, Label: route.Label(), Framework: route.Framework, Mux: route.Mux, Site: route.Site, Location: route.Location, Method: route.Method, MethodKnown: route.MethodKnown, DeclaredPattern: route.DeclaredPattern, Pattern: route.Pattern, PatternKnown: route.PatternKnown, Host: route.Host, Issue: route.Issue, Provenance: route.Provenance})
	}
	data.Calls = make([]callData, len(index.Calls))
	for i, indexed := range index.Calls {
		call := indexed.Call
		data.Calls[i] = callData{
			From: indexed.From, To: indexed.To,
			ThroughInterface: call.ThroughInterface, Violation: call.Violation,
			Contexts: call.Contexts, Execution: call.Execution, Site: call.Site,
			Path:    call.Path,
			Dynamic: call.Dynamic,
		}
	}
	return data
}
