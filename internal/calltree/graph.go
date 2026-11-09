package calltree

import "github.com/PranavJoshi2893/codebound/internal/calltree/model"

// Keep the analyzer's existing result API while allowing presentation packages
// to depend on evidence alone, without importing the Go analysis engine.
type (
	Graph            = model.Graph
	Function         = model.Function
	Call             = model.Call
	CallPath         = model.CallPath
	ContextInfo      = model.ContextInfo
	ContextBinding   = model.ContextBinding
	ContextOperation = model.ContextOperation
	FunctionFlow     = model.FunctionFlow
	FlowScope        = model.FlowScope
	FlowBlock        = model.FlowBlock
	FlowStep         = model.FlowStep
	FlowBranch       = model.FlowBranch
	functionID       = model.FunctionID
)
