package model

import "testing"

func TestDeclarationIdentityIgnoresAnnotations(t *testing.T) {
	function := Function{Name: "Run", Package: "app", File: "app.go", Line: 10}
	annotated := function
	annotated.Layer = "service"
	annotated.Context = &ContextInfo{Parameters: []string{"ctx"}}
	annotated.Flow = &FunctionFlow{Scopes: []FlowScope{{ID: "body"}}}
	if function.Identity() != annotated.Identity() {
		t.Fatal("annotations changed declaration identity")
	}
	for _, changed := range []Function{
		{Name: "Other", Package: "app", File: "app.go", Line: 10},
		{Name: "Run", Package: "other", File: "app.go", Line: 10},
		{Name: "Run", Package: "app", File: "other.go", Line: 10},
		{Name: "Run", Package: "app", File: "app.go", Line: 11},
	} {
		if function.Identity() == changed.Identity() {
			t.Fatalf("merged distinct declaration: %+v", changed)
		}
	}
}

func TestOnlyConstructorsCreateContexts(t *testing.T) {
	for _, kind := range []string{"create", "derive", "detach", "forward", "read", "observe", "cancel", ""} {
		want := kind == "create" || kind == "derive" || kind == "detach"
		if got := (ContextOperation{Kind: kind}).CreatesContext(); got != want {
			t.Errorf("%s: %t", kind, got)
		}
	}
}
