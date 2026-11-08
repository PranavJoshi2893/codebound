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

// ContextInfo records source-level evidence, not a promise that cancellation
// happens or that a function stops when its context is cancelled.
type ContextInfo struct {
	Parameters []string           `json:"parameters,omitempty"`
	Operations []ContextOperation `json:"operations,omitempty"`
}

// ContextOperation describes one context action and its possible local origin.
// Kind is create, derive, detach, forward, read, observe, or cancel.
type ContextOperation struct {
	Kind      string `json:"kind"`
	Context   string `json:"context"`
	Parent    string `json:"parent,omitempty"`
	Target    string `json:"target"`
	Detail    string `json:"detail,omitempty"`
	Location  string `json:"location"`
	Scope     string `json:"scope,omitempty"`
	Execution string `json:"execution,omitempty"`
}

type Call struct {
	Caller           Function `json:"caller"`
	Callee           Function `json:"callee"`
	ThroughInterface bool     `json:"through_interface"`
	Violation        string   `json:"violation,omitempty"`
}

type Graph struct {
	Root  Function `json:"root"`
	Calls []Call   `json:"calls"`
}
