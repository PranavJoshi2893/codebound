// Package calltree analyzes Go source without depending on CLI or HTTP code.
package calltree

type Function struct {
	Name    string       `json:"name"`
	Package string       `json:"package"`
	File    string       `json:"file"`
	Line    int          `json:"line"`
	Layer   string       `json:"layer,omitempty"`
	Context *ContextInfo `json:"context,omitempty"`
}

// Declaration identity is shared by call traversal, context flow, and rendering.
type functionID struct {
	name, pkg, file string
	line            int
}

func (f Function) id() functionID {
	return functionID{f.Name, f.Package, f.File, f.Line}
}

// ContextInfo records source-level evidence, not a promise that cancellation
// happens or that a function stops when its context is cancelled.
type ContextInfo struct {
	Parameters []string           `json:"parameters,omitempty"`
	Bindings   []ContextBinding   `json:"bindings,omitempty"`
	Operations []ContextOperation `json:"operations,omitempty"`
}

// ContextBinding carries possible creation-site IDs, not runtime object IDs.
// I-prefixed IDs enter at the selected root; their creation is outside this tree.
// Binding lists follow declared context-parameter order, including blank names.
type ContextBinding struct {
	Parameter  string   `json:"parameter"`
	IDs        []string `json:"ids,omitempty"`
	Unresolved bool     `json:"unresolved,omitempty"`
}

// ContextOperation describes one context action and its possible local origin.
// Kind is create, derive, detach, forward, read, observe, or cancel.
// Context retains the local source description; IDs link it through the graph.
type ContextOperation struct {
	Kind             string   `json:"kind"`
	Context          string   `json:"context"`
	Parent           string   `json:"parent,omitempty"`
	Target           string   `json:"target"`
	Detail           string   `json:"detail,omitempty"`
	Location         string   `json:"location"`
	Scope            string   `json:"scope,omitempty"`
	Execution        string   `json:"execution,omitempty"`
	IDs              []string `json:"ids,omitempty"`
	ParentIDs        []string `json:"parent_ids,omitempty"`
	Unresolved       bool     `json:"unresolved,omitempty"`
	ParentUnresolved bool     `json:"parent_unresolved,omitempty"`
}

type Call struct {
	Caller           Function         `json:"caller"`
	Callee           Function         `json:"callee"`
	ThroughInterface bool             `json:"through_interface"`
	Violation        string           `json:"violation,omitempty"`
	Contexts         []ContextBinding `json:"contexts,omitempty"`
	Execution        string           `json:"execution,omitempty"`
}

type Graph struct {
	Root  Function `json:"root"`
	Calls []Call   `json:"calls"`
}
