package viewer

import (
	"github.com/PranavJoshi2893/codebound/internal/calltree/diagram"
	"github.com/PranavJoshi2893/codebound/internal/calltree/model"
)

// Send each function's details once, even when it has many callers.
type graphData struct {
	Functions []model.Function    `json:"functions"`
	Calls     []callData          `json:"calls"`
	Evidence  *model.CallEvidence `json:"evidence,omitempty"`
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
