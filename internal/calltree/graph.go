package calltree

import "github.com/PranavJoshi2893/codebound/internal/calltree/model"

// Keep the analyzer's existing result API while allowing presentation packages
// to depend on evidence alone, without importing the Go analysis engine.
type (
	Graph            = model.Graph
	Function         = model.Function
	Call             = model.Call
	ContextInfo      = model.ContextInfo
	ContextBinding   = model.ContextBinding
	ContextOperation = model.ContextOperation
	functionID       = model.FunctionID
)
