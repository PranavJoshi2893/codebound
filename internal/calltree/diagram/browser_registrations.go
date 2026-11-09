package diagram

import (
	"fmt"
	"io"
	"sort"

	"github.com/PranavJoshi2893/codebound/internal/calltree/model"
)

type IndexedRegistration struct {
	From, To     int // To is -1 for an unresolved/nil handler, never an invented node.
	Registration *model.RouteRegistration
}

func (index *IndexedGraph) RegistrationOnly(id int) bool {
	return id >= 0 && id < len(index.registeredOnly) && index.registeredOnly[id]
}

func (index *IndexedGraph) RegistrationWarnings() []string {
	if index.graph == nil {
		return nil
	}
	return index.graph.RegistrationWarnings
}

type browserSelection struct {
	visible, calls, registrations        []int
	includedCalls, includedRegistrations map[int]int
}

func (index *IndexedGraph) selectBrowser(options BrowserOptions) browserSelection {
	selection := browserSelection{includedCalls: make(map[int]int), includedRegistrations: make(map[int]int)}
	if options.Focus < 0 {
		for id := range index.Functions {
			if !options.HideRegistrations || !index.registeredOnly[id] {
				selection.visible = append(selection.visible, id)
			}
		}
		if !options.HideRegistrations {
			for id := range index.Registrations {
				selection.registrations = append(selection.registrations, id)
			}
		}
		return selection
	}
	selection.calls = index.incident[index.offsets[options.Focus]:index.offsets[options.Focus+1]]
	visible := map[int]bool{options.Focus: true}
	pairs := make(map[[2]int]bool)
	for _, id := range selection.calls {
		call := index.Calls[id]
		visible[call.From], visible[call.To] = true, true
		pair := [2]int{call.From, call.To}
		if !pairs[pair] {
			pairs[pair] = true
			selection.includedCalls[call.From]++
			if call.From != call.To {
				selection.includedCalls[call.To]++
			}
		}
	}
	if !options.HideRegistrations {
		selection.registrations = index.registrationIncident[options.Focus]
		for _, id := range selection.registrations {
			route := index.Registrations[id]
			visible[route.From] = true
			if route.To >= 0 {
				visible[route.To] = true
				selection.includedRegistrations[route.From]++
				if route.From != route.To {
					selection.includedRegistrations[route.To]++
				}
			}
		}
	}
	for id := range visible {
		selection.visible = append(selection.visible, id)
	}
	sort.Ints(selection.visible)
	return selection
}

// Extend the invocation layout DAG without reversing any of its existing
// feedback decisions. Configuration links may reserve a later geometric rank,
// but a link which would introduce a cycle uses exterior feedback ports instead.
func (index *IndexedGraph) registrationFeedback(calls []browserEdge, registrations []int) map[int]bool {
	outgoing := make(map[int][]int)
	for _, edge := range calls {
		if !edge.backward && edge.from != edge.to {
			outgoing[edge.from] = append(outgoing[edge.from], edge.to)
		}
	}
	feedback := make(map[int]bool)
	pairs := make(map[[2]int]bool)
	for _, id := range registrations {
		route := index.Registrations[id]
		if route.To < 0 {
			continue
		}
		pair := [2]int{route.From, route.To}
		backward, known := pairs[pair]
		if !known {
			pending, seen := []int{route.To}, make(map[int]bool)
			for len(pending) > 0 {
				next := pending[len(pending)-1]
				pending = pending[:len(pending)-1]
				if next == route.From {
					backward = true
					break
				}
				if seen[next] {
					continue
				}
				seen[next] = true
				pending = append(pending, outgoing[next]...)
			}
			pairs[pair] = backward
			if !backward {
				outgoing[route.From] = append(outgoing[route.From], route.To)
			}
		}
		feedback[id] = backward
	}
	return feedback
}

func (index *IndexedGraph) writeRegistrationDOT(output io.Writer, id int, backward bool) error {
	route := index.Registrations[id]
	if route.To < 0 || route.From == route.To {
		return nil
	}
	tail, head := "e", "w"
	if backward {
		tail, head = "w", "e"
	}
	marker := "odot"
	if _, exists := index.sites[route.From][route.Registration.Site]; route.Registration.Site != "" && exists {
		tail, marker = route.Registration.Site+":e", "dot"
		if backward {
			tail = route.Registration.Site + "_w:w"
		}
	}
	if index.Functions[route.To].Flow != nil {
		head = "entry:" + head
	}
	caption := "registers " + route.Registration.Label()
	if backward {
		caption = ""
	} // The exact caption stays at its protected source row.
	_, err := fmt.Fprintf(output, "  n%d -> n%d [label=%q, tooltip=%q, style=solid, color=\"#64748b\", fontcolor=\"#526075\", penwidth=1, arrowsize=0.7, dir=both, arrowtail=%q, arrowhead=odiamond, tailport=%q, headport=%q, constraint=%t, URL=%q];\n",
		route.From, route.To, caption, "registers "+route.Registration.Label()+"\nRoute registration, not a handler invocation\n"+route.Registration.Location, marker, tail, head, !backward, fmt.Sprintf("registration:%d:%d:%d:%s", route.From, route.To, id, route.Registration.Site))
	return err
}

func (index *IndexedGraph) registrationNotes(ids []int, feedback map[int]bool) map[int]map[string][]string {
	notes := make(map[int]map[string][]string)
	for _, id := range ids {
		route := index.Registrations[id]
		if route.To >= 0 && route.To != route.From && route.Registration.Issue == "" && !feedback[id] {
			continue
		}
		if notes[route.From] == nil {
			notes[route.From] = make(map[string][]string)
		}
		text := "registers " + route.Registration.Label()
		if route.From == route.To {
			text = "◇ " + text + " → this function (not a call)"
		} else if feedback[id] {
			text = "◇ " + text + " → " + index.Functions[route.To].Name
		}
		if route.Registration.Issue != "" {
			text += " · " + route.Registration.Issue
		}
		notes[route.From][route.Registration.Site] = append(notes[route.From][route.Registration.Site], text)
	}
	return notes
}
