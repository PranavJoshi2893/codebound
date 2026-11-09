// Package model contains the source-analysis evidence shared by the analyzer,
// exporters, and viewer. It has no dependency on SSA or presentation code.
package model

type Function struct {
	Name    string        `json:"name"`
	Package string        `json:"package"`
	File    string        `json:"file"`
	Line    int           `json:"line"`
	Layer   string        `json:"layer,omitempty"`
	Context *ContextInfo  `json:"context,omitempty"`
	Flow    *FunctionFlow `json:"flow,omitempty"`
}

// Declaration identity is shared by call traversal, context flow, and rendering.
type FunctionID struct {
	name, pkg, file string
	line            int
}

func (f Function) Identity() FunctionID {
	return FunctionID{f.Name, f.Package, f.File, f.Line}
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
	Site             string   `json:"site,omitempty"`
}

func (operation ContextOperation) CreatesContext() bool {
	switch operation.Kind {
	case "create", "derive", "detach":
		return true
	default:
		return false
	}
}

type Call struct {
	Caller           Function         `json:"caller"`
	Callee           Function         `json:"callee"`
	ThroughInterface bool             `json:"through_interface"`
	Dynamic          bool             `json:"dynamic,omitempty"`
	Violation        string           `json:"violation,omitempty"`
	Contexts         []ContextBinding `json:"contexts,omitempty"`
	Execution        string           `json:"execution,omitempty"`
	Site             string           `json:"site,omitempty"`
	Path             *CallPath        `json:"path,omitempty"`
}

type Graph struct {
	Root                 Function            `json:"root"`
	Calls                []Call              `json:"calls"`
	Evidence             *CallEvidence       `json:"evidence,omitempty"`
	Registrations        []RouteRegistration `json:"registrations,omitempty"`
	RegistrationWarnings []string            `json:"registration_warnings,omitempty"`
}
