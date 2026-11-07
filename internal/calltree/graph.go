// Package calltree analyzes Go source without depending on CLI or HTTP code.
package calltree

type Function struct {
	Name    string `json:"name"`
	Package string `json:"package"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Layer   string `json:"layer,omitempty"`
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
