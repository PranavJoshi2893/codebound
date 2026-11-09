package model

// CallPath projects a corridor of raw calls, NOT a direct invocation. Entry and
// Exit index Evidence.Edges. Follow edges between them through hidden nodes only
// (never through another visible declaration). This shared representation retains
// alternatives and cycles without enumerating exponentially many walks.
// HTTPDispatch selects walks that cross an HTTP dispatch edge; false selects
// walks without that boundary. Site/Execution on Call describe the entry only;
// Contexts describe arguments at the exit, not bindings from the visible caller.
type CallPath struct {
	Entry        int  `json:"entry"`
	Exit         int  `json:"exit"`
	HTTPDispatch bool `json:"http_dispatch,omitempty"`
}

type CallEvidence struct {
	Nodes []EvidenceFunction `json:"nodes"`
	Edges []EvidenceCall     `json:"edges"`
}

type EvidenceFunction struct {
	ID      string `json:"id"`
	Visible bool   `json:"visible,omitempty"`
	// Hidden creation/operation evidence keeps IDs traceable without adding cards.
	Context *ContextInfo `json:"context,omitempty"`
}

// Instruction is the SSA block/instruction address within From, including
// synthetic instructions without source positions. Location is source evidence,
// while Site is the optional row in a visible function's execution flow.
type EvidenceCall struct {
	From             int    `json:"from"`
	To               int    `json:"to"`
	Instruction      string `json:"instruction,omitempty"`
	Location         string `json:"location,omitempty"`
	Site             string `json:"site,omitempty"`
	Execution        string `json:"execution,omitempty"`
	ThroughInterface bool   `json:"through_interface,omitempty"`
	Dynamic          bool   `json:"dynamic,omitempty"`
	HTTPDispatch     bool   `json:"http_dispatch,omitempty"`
}
