package diagram

import "github.com/PranavJoshi2893/codebound/internal/calltree/model"

// IndexedCall references immutable analysis evidence without copying its two
// function descriptions. Indices are stable in root-first discovery order.
type IndexedCall struct {
	From, To int
	Call     *model.Call
}

// IndexedGraph is a reusable presentation index. Build once after analysis and
// do not mutate its source graph while it is in use.
type IndexedGraph struct {
	Functions             []model.Function
	Calls                 []IndexedCall
	Registrations         []IndexedRegistration
	graph                 *model.Graph
	offsets               []int
	incident              []int
	relations             []int
	sites                 []map[string]indexedStep
	layouts               []flowLayout
	phases                []browserPhase
	registrationIncident  [][]int
	registrationRelations []int
	registeredOnly        []bool
}

type indexedStep struct {
	step  model.FlowStep
	order int // Stable display/reference order, not execution order across blocks.
}

// Evidence remains shared; hidden nodes are not added to the presentation index.
func (index *IndexedGraph) Evidence() *model.CallEvidence {
	if index.graph == nil {
		return nil
	}
	return index.graph.Evidence
}

// IndexGraph builds a root-first identity index and compact incident-call lists.
// Reuse it for presentation queries instead of rebuilding it per selected node.
func IndexGraph(graph *model.Graph) *IndexedGraph {
	index := &IndexedGraph{graph: graph}
	if graph == nil {
		return index
	}
	ids := make(map[model.FunctionID]int)
	add := func(function model.Function) int {
		if id, exists := ids[function.Identity()]; exists {
			return id
		}
		id := len(index.Functions)
		ids[function.Identity()] = id
		index.Functions = append(index.Functions, function)
		return id
	}
	add(graph.Root)
	index.Calls = make([]IndexedCall, len(graph.Calls))
	for i := range graph.Calls {
		call := &graph.Calls[i]
		index.Calls[i] = IndexedCall{From: add(call.Caller), To: add(call.Callee), Call: call}
	}
	invoked := len(index.Functions)
	for i := range graph.Registrations {
		route := &graph.Registrations[i]
		from, to := add(route.Registrar), -1
		if route.Handler != nil {
			to = add(*route.Handler)
		}
		index.Registrations = append(index.Registrations, IndexedRegistration{From: from, To: to, Registration: route})
	}
	index.registeredOnly = make([]bool, len(index.Functions))
	index.registrationIncident = make([][]int, len(index.Functions))
	index.registrationRelations = make([]int, len(index.Functions))
	for i := invoked; i < len(index.Functions); i++ {
		index.registeredOnly[i] = true
	}
	for id, route := range index.Registrations {
		index.registrationIncident[route.From] = append(index.registrationIncident[route.From], id)
		if route.To >= 0 {
			index.registrationRelations[route.From]++
			if route.From != route.To {
				index.registrationIncident[route.To] = append(index.registrationIncident[route.To], id)
				index.registrationRelations[route.To]++
			}
		}
	}
	index.phases = index.lifecyclePhases()
	index.sites = make([]map[string]indexedStep, len(index.Functions))
	index.layouts = make([]flowLayout, len(index.Functions))
	for id, function := range index.Functions {
		index.layouts[id] = planFlowLayout(function.Flow)
		if function.Flow == nil {
			continue
		}
		index.sites[id] = make(map[string]indexedStep)
		for _, scope := range function.Flow.Scopes {
			for _, block := range scope.Blocks {
				for _, step := range block.Steps {
					index.sites[id][step.ID] = indexedStep{step: step, order: len(index.sites[id])}
				}
			}
		}
	}
	callSites := make(map[int]map[string]bool)
	for _, call := range index.Calls {
		if call.From == call.To || call.Call.Site == "" {
			continue
		}
		if callSites[call.From] == nil {
			callSites[call.From] = make(map[string]bool)
		}
		callSites[call.From][call.Call.Site] = true
	}
	for _, route := range index.Registrations {
		if route.To >= 0 && route.Registration.Site != "" {
			if callSites[route.From] == nil {
				callSites[route.From] = make(map[string]bool)
			}
			callSites[route.From][route.Registration.Site] = true
		}
	}
	for id, sites := range callSites {
		index.layouts[id] = index.layouts[id].exposeCallSites(sites)
	}
	index.offsets = make([]int, len(index.Functions)+1)
	index.relations = make([]int, len(index.Functions))
	seen := make(map[[2]int]struct{})
	for _, call := range index.Calls {
		index.offsets[call.From+1]++
		if call.From != call.To {
			index.offsets[call.To+1]++
		}
		pair := [2]int{call.From, call.To}
		if _, exists := seen[pair]; !exists {
			seen[pair] = struct{}{}
			index.relations[call.From]++
			if call.From != call.To {
				index.relations[call.To]++
			}
		}
	}
	for i := 1; i < len(index.offsets); i++ {
		index.offsets[i] += index.offsets[i-1]
	}
	index.incident = make([]int, index.offsets[len(index.Functions)])
	next := append([]int(nil), index.offsets[:len(index.Functions)]...)
	for i, call := range index.Calls {
		index.incident[next[call.From]] = i
		next[call.From]++
		if call.From != call.To {
			index.incident[next[call.To]] = i
			next[call.To]++
		}
	}
	return index
}
