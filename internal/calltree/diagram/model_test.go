package diagram

import "github.com/PranavJoshi2893/codebound/internal/calltree/model"

// Short names keep presentation fixtures readable; production code explicitly
// imports the shared model rather than depending on the analyzer.
type (
	Graph            = model.Graph
	Function         = model.Function
	Call             = model.Call
	ContextInfo      = model.ContextInfo
	ContextBinding   = model.ContextBinding
	ContextOperation = model.ContextOperation
	functionID       = model.FunctionID
)
