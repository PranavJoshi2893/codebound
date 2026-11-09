package diagram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/PranavJoshi2893/codebound/internal/calltree/model"
)

func flowRowIDs(plan flowScopeLayout) [][]int {
	var rows [][]int
	for _, row := range plan.rows {
		var ids []int
		for _, panel := range row {
			ids = append(ids, plan.scope.Blocks[panel.block].ID)
		}
		rows = append(rows, ids)
	}
	return rows
}

func TestFlowLayoutBranchesShareRowsAndJoinsWaitForEveryPath(t *testing.T) {
	// Deliberately not in block-ID order: true should be left of false, and
	// the join must wait for the longer false branch, not its first arrival.
	scope := model.FlowScope{ID: "body", Entry: 10, Blocks: []model.FlowBlock{
		{ID: 50, Exit: "return"},
		{ID: 30, Exit: "jump", Next: []model.FlowBranch{{To: 40}}},
		{ID: 10, Exit: "branch", Next: []model.FlowBranch{{To: 20, Label: "true"}, {To: 30, Label: "false"}}},
		{ID: 40, Exit: "jump", Next: []model.FlowBranch{{To: 50}}},
		{ID: 20, Exit: "jump", Next: []model.FlowBranch{{To: 50}}},
	}}
	before, _ := json.Marshal(scope)
	plan := planFlowScope(scope)
	want := [][]int{{10}, {20, 30}, {40}, {50}}
	if got := flowRowIDs(plan); !reflect.DeepEqual(got, want) || plan.columns != 2 {
		t.Fatalf("branch alternatives should share horizontal space, with the join below: got %v want %v", got, want)
	}
	after, _ := json.Marshal(scope)
	if !bytes.Equal(before, after) {
		t.Fatal("layout must not reorder the underlying execution evidence")
	}
}

func TestFlowLayoutLoopsRecoveryAndIncompleteCycles(t *testing.T) {
	recoverID := 99
	scope := model.FlowScope{ID: "body", Entry: 0, Recover: &recoverID, Blocks: []model.FlowBlock{
		{ID: 0, Exit: "jump", Next: []model.FlowBranch{{To: 1}}},
		{ID: 1, Exit: "branch", Next: []model.FlowBranch{{To: 2, Label: "true"}, {To: 3, Label: "false"}}},
		{ID: 2, Exit: "jump", Next: []model.FlowBranch{{To: 1, BackEdge: true}}},
		{ID: 3, Exit: "return"}, {ID: 99, Exit: "return"}, {ID: 100, Exit: "panic"},
	}}
	if got := flowRowIDs(planFlowScope(scope)); !reflect.DeepEqual(got, [][]int{{0}, {1}, {2, 3}, {99}, {100}}) {
		t.Fatalf("loop alternatives may share a row, but recovery/unconnected entries stay separate: %v", got)
	}
	scope = model.FlowScope{Entry: 0, Blocks: []model.FlowBlock{
		{ID: 0, Next: []model.FlowBranch{{To: 1}}},
		{ID: 1, Next: []model.FlowBranch{{To: 0}}},
	}}
	if got := flowRowIDs(planFlowScope(scope)); !reflect.DeepEqual(got, [][]int{{0}, {1}}) {
		t.Fatalf("incomplete loop evidence must not drop blocks or invent parallelism: %v", got)
	}
}

func TestFlowLayoutLongBlocksUseOrderedContinuationColumns(t *testing.T) {
	block := model.FlowBlock{ID: 7, Label: "entry", Exit: "return"}
	for i := 1; i <= 14; i++ {
		block.Steps = append(block.Steps, model.FlowStep{ID: fmt.Sprintf("s%d", i), Reference: fmt.Sprintf("B7.%d", i), Kind: "call", Label: fmt.Sprintf("call%d()", i)})
	}
	function := Function{Name: "main", Flow: &model.FunctionFlow{Scopes: []model.FlowScope{{ID: "body", Entry: 7, Blocks: []model.FlowBlock{block}}}}}
	before, _ := json.Marshal(function)
	plan := planFlowLayout(function.Flow)
	panels := plan.scopes[0].rows
	if plan.columns != 3 || len(panels) != 1 || !reflect.DeepEqual(panels[0], []flowPanel{{block: 0, first: 0, last: 5}, {block: 0, first: 5, last: 10}, {block: 0, first: 10, last: 14}}) {
		t.Fatalf("long sequential blocks should use contiguous, balanced continuation columns: %+v", panels)
	}
	label := renderFlowLabel("main", function, false, true, nil, plan)
	for _, want := range []string{"steps 1–5", "steps 6–10", "steps 11–14", "continue → B7.6", "continue → B7.11"} {
		if !strings.Contains(label, want) {
			t.Errorf("continuation must preserve explicit reading order: %s", want)
		}
	}
	for i := 1; i <= 14; i++ {
		if strings.Count(label, fmt.Sprintf(`PORT="s%d"`, i)) != 1 {
			t.Fatalf("step %d must appear exactly once", i)
		}
	}
	if strings.Count(label, ">return<") != 1 {
		t.Fatal("only the final continuation may show the actual block exit")
	}
	after, _ := json.Marshal(function)
	if !bytes.Equal(before, after) {
		t.Fatal("visual continuations are not new analysis blocks or reordered steps")
	}
}

func TestFlowLayoutIndependentScopesUseHorizontalSpace(t *testing.T) {
	flow := &model.FunctionFlow{}
	for _, id := range []string{"body", "L1", "L2", "L3"} {
		flow.Scopes = append(flow.Scopes, model.FlowScope{ID: id, Entry: 0, Blocks: []model.FlowBlock{{ID: 0, Exit: "return"}}})
	}
	plan := planFlowLayout(flow)
	if plan.columns != 3 || !reflect.DeepEqual(plan.groups, [][]int{{0, 1, 2}, {3}}) {
		t.Fatalf("independent scopes should share bounded horizontal rows: %+v", plan.groups)
	}
	for _, flow := range []*model.FunctionFlow{nil, {}, {Scopes: []model.FlowScope{{ID: "body"}}}} {
		if plan := planFlowLayout(flow); plan.columns != 1 || len(plan.groups) != 0 {
			t.Fatal("empty flow must not create empty or invalid table panels")
		}
	}
}

func TestFlowLayoutReusesWidthAlreadyNeededByBranches(t *testing.T) {
	plan := planFlowLayout(browserExecutionFixture().Root.Flow)
	want := []flowPanel{{block: 0, first: 0, last: 2}, {block: 0, first: 2, last: 4}}
	if !reflect.DeepEqual(plan.scopes[0].rows[0], want) || plan.scopes[0].columns != 2 {
		t.Fatalf("entry steps should reuse existing branch columns without widening their scope: %+v", plan.scopes[0].rows[0])
	}
}
