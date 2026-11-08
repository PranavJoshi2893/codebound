package calltree

import "sort"

// A zero value is an empty, still-pending set, not an unresolved input.
type contextIDs struct {
	ids        map[string]bool
	unresolved bool
}

func (values *contextIDs) merge(other contextIDs) bool {
	changed := false
	if other.unresolved && !values.unresolved {
		values.unresolved, changed = true, true
	}
	for id := range other.ids {
		if !values.ids[id] {
			if values.ids == nil {
				values.ids = make(map[string]bool)
			}
			values.ids[id], changed = true, true
		}
	}
	return changed
}

func (values contextIDs) names() []string {
	var names []string
	for id := range values.ids {
		names = append(names, id)
	}
	sort.Slice(names, func(i, j int) bool {
		if names[i][0] == names[j][0] && len(names[i]) != len(names[j]) {
			return len(names[i]) < len(names[j])
		}
		return names[i] < names[j]
	})
	return names
}

func (values contextIDs) binding(parameter string) ContextBinding {
	return ContextBinding{Parameter: parameter, IDs: values.names(), Unresolved: values.unresolved || len(values.ids) == 0}
}
