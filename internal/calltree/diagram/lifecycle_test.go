package diagram

import (
	"reflect"
	"testing"
)

func TestContextCancellationHandlesCyclesAndDetachedParents(t *testing.T) {
	operations := []*ContextOperation{
		{Target: "context.Background", IDs: []string{"C0"}},
		{Target: "context.WithValue", IDs: []string{"C1"}, ParentIDs: []string{"C2"}},
		{Target: "context.WithValue", IDs: []string{"C2"}, ParentIDs: []string{"C1", "C0"}},
		{Target: "context.WithCancel", IDs: []string{"C3"}, ParentIDs: []string{"I1"}},
		{Target: "context.WithoutCancel", IDs: []string{"C4"}, ParentIDs: []string{"C3"}},
		{Target: "context.WithValue", IDs: []string{"C5"}, ParentIDs: []string{"C4"}},
		{Target: "context.WithValue", IDs: []string{"C6"}, ParentIDs: []string{"I2"}},
	}
	byID := map[string]int{"C0": 0, "C1": 1, "C2": 2, "C3": 3, "C4": 4, "C5": 5, "C6": 6}
	want := []bool{false, false, false, true, true, false, true}
	if got := contextParentCancellation(operations, byID); !reflect.DeepEqual(got, want) {
		t.Fatalf("wrong cancellation inheritance: got %v, want %v", got, want)
	}
	// A cancellable source can reach the cycle, but not cross WithoutCancel.
	operations[2].ParentIDs = append(operations[2].ParentIDs, "C3")
	want[1], want[2] = true, true
	if got := contextParentCancellation(operations, byID); !reflect.DeepEqual(got, want) {
		t.Fatalf("cancellation failed to propagate through the cycle: got %v, want %v", got, want)
	}
}

func TestContextCancellationUnknownParent(t *testing.T) {
	operations := []*ContextOperation{
		{Target: "context.WithValue", IDs: []string{"C1"}, Parent: "unresolved", ParentUnresolved: true},
		{Target: "context.WithValue", IDs: []string{"C2"}, ParentIDs: []string{"C1"}},
	}
	if got := contextParentCancellation(operations, map[string]int{"C1": 0, "C2": 1}); !reflect.DeepEqual(got, []bool{true, true}) {
		t.Fatalf("unresolved parent must remain possibly cancellable: %v", got)
	}
}
