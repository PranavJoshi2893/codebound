package model

// FunctionFlow records static control flow, not a runtime trace. Blocks are
// addressed by ID, not executed in slice order. Only steps within one block are
// sequential. Closure scopes have separate entries and do not run on declaration.
type FunctionFlow struct {
	Scopes []FlowScope `json:"scopes"`
}

type FlowScope struct {
	ID       string      `json:"id"`
	Location string      `json:"location,omitempty"`
	Entry    int         `json:"entry"`
	Recover  *int        `json:"recover,omitempty"` // Entry only if a panic is recovered.
	Blocks   []FlowBlock `json:"blocks"`
}

type FlowBlock struct {
	ID        int          `json:"id"`
	Label     string       `json:"label"`
	Steps     []FlowStep   `json:"steps,omitempty"`
	Condition string       `json:"condition,omitempty"`
	Exit      string       `json:"exit"` // jump, branch, return, panic, or stop
	Next      []FlowBranch `json:"next,omitempty"`
}

// Site IDs link call edges and context evidence to their exact local step.
// Calls with the same caller/callee retain distinct sites; interface targets at
// one invocation share a site. Source positions are descriptive, not ordering keys.
type FlowStep struct {
	ID        string `json:"id"`
	Reference string `json:"reference"`
	Kind      string `json:"kind"` // call, go, defer, run_defers, receive, send, select
	Label     string `json:"label"`
	Location  string `json:"location,omitempty"`
	Closure   string `json:"closure,omitempty"`
}

type FlowBranch struct {
	To       int    `json:"to"`
	Label    string `json:"label,omitempty"`
	BackEdge bool   `json:"back_edge,omitempty"`
}
