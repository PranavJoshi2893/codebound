package calltree

import "testing"

func TestBoundaryViolation(t *testing.T) {
	for _, test := range []struct {
		caller, callee Function
		invoke         bool
		want           string
	}{
		{Function{Layer: "handler"}, Function{Layer: "service"}, false, ""},
		{Function{Layer: "service"}, Function{Layer: "repository"}, false, ""},
		{Function{Layer: "handler"}, Function{Layer: "repository"}, false, "handler must not call repository"},
		{Function{Layer: "repository"}, Function{Layer: "service"}, false, "repository must not call service"},
		{Function{Layer: "service", Package: "user"}, Function{Layer: "service", Package: "order"}, false, "cross-service call must use an interface"},
		{Function{Layer: "service", Package: "user"}, Function{Layer: "service", Package: "order"}, true, ""},
		{Function{Layer: "service", Package: "user"}, Function{Layer: "service", Package: "user"}, false, ""},
		{Function{}, Function{Layer: "repository"}, false, ""},
	} {
		if got := boundaryViolation(test.caller, test.callee, test.invoke); got != test.want {
			t.Errorf("%+v -> %+v: got %q, want %q", test.caller, test.callee, got, test.want)
		}
	}
}

func TestInferLayer(t *testing.T) {
	for file, want := range map[string]string{
		"internal/handler/user.go":  "handler",
		"internal/usecases/user.go": "service",
		"internal/storage/user.go":  "repository",
		"cmd/api/main.go":           "",
	} {
		if got := inferLayer("example.com/app", file); got != want {
			t.Errorf("%s: got %q, want %q", file, got, want)
		}
	}
}
