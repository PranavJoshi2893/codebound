package diagram

// Lifecycle is presentation reachability, not an inferred function role or a
// runtime schedule. Only recorded HTTP dispatch boundaries introduce request
// reachability. A helper reached on both sides remains explicitly shared.
type browserPhase uint8

const (
	phaseEntry browserPhase = 1 << iota
	phaseHTTP
)

func (phase browserPhase) caption() string {
	switch phase {
	case phaseEntry:
		return "ENTRY / LIFECYCLE"
	case phaseHTTP:
		return "HTTP REQUEST"
	case phaseEntry | phaseHTTP:
		return "SHARED · ENTRY + HTTP"
	default:
		return ""
	}
}

func (phase browserPhase) fill() string {
	switch phase {
	case phaseEntry:
		return "#eff6ff"
	case phaseHTTP:
		return "#f0fdfa"
	case phaseEntry | phaseHTTP:
		return "#fff7ed"
	default:
		return "#f8fafc"
	}
}

func (index *IndexedGraph) lifecyclePhases() []browserPhase {
	hasDispatch := false
	for _, call := range index.Calls {
		hasDispatch = hasDispatch || (call.Call.Path != nil && call.Call.Path.HTTPDispatch)
	}
	if !hasDispatch || len(index.Functions) == 0 {
		return nil
	}
	outgoing := make([][]IndexedCall, len(index.Functions))
	for _, call := range index.Calls {
		outgoing[call.From] = append(outgoing[call.From], call)
	}
	phases := make([]browserPhase, len(index.Functions))
	type state struct {
		node  int
		phase browserPhase
	}
	pending := []state{{0, phaseEntry}}
	phases[0] = phaseEntry
	for next := 0; next < len(pending); next++ {
		current := pending[next]
		for _, call := range outgoing[current.node] {
			phase := current.phase
			if call.Call.Path != nil && call.Call.Path.HTTPDispatch {
				phase = phaseHTTP
			}
			if phases[call.To]&phase != 0 {
				continue
			}
			phases[call.To] |= phase
			pending = append(pending, state{call.To, phase})
		}
	}
	return phases
}

func (index *IndexedGraph) phase(id int) browserPhase {
	if len(index.phases) == 0 {
		return 0
	}
	return index.phases[id]
}

type browserGroup struct {
	pkg   string
	phase browserPhase
	nodes []int
}

// Split package groups at dispatch boundaries. Keeping constructors and request
// handlers inside one package-wide rectangle forces huge empty routing regions.
// Classification is cached from the full graph, so focused views retain meaning.
func (index *IndexedGraph) browserGroups(visible []int) []browserGroup {
	type key struct {
		pkg   string
		phase browserPhase
	}
	var groups []browserGroup
	ids := make(map[key]int)
	for _, id := range visible {
		group := key{index.Functions[id].Package, index.phase(id)}
		position, exists := ids[group]
		if !exists {
			position = len(groups)
			ids[group] = position
			groups = append(groups, browserGroup{pkg: group.pkg, phase: group.phase})
		}
		groups[position].nodes = append(groups[position].nodes, id)
	}
	return groups
}
