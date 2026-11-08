package viewer

import "github.com/PranavJoshi2893/codebound/internal/calltree"

// Send each function's details once, even when it has many callers.
type graphData struct {
	Functions []calltree.Function `json:"functions"`
	Calls     []callData          `json:"calls"`
}

type callData struct {
	From             int                       `json:"from"`
	To               int                       `json:"to"`
	ThroughInterface bool                      `json:"through_interface,omitempty"`
	Violation        string                    `json:"violation,omitempty"`
	Contexts         []calltree.ContextBinding `json:"contexts,omitempty"`
	Execution        string                    `json:"execution,omitempty"`
}

func describeGraph(index *calltree.IndexedGraph) graphData {
	data := graphData{Functions: []calltree.Function{}, Calls: []callData{}}
	if len(index.Functions) == 0 {
		return data
	}
	data.Functions = index.Functions
	data.Calls = make([]callData, len(index.Calls))
	for i, indexed := range index.Calls {
		call := indexed.Call
		data.Calls[i] = callData{
			From: indexed.From, To: indexed.To,
			ThroughInterface: call.ThroughInterface, Violation: call.Violation,
			Contexts: call.Contexts, Execution: call.Execution,
		}
	}
	return data
}
