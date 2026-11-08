package viewer

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/PranavJoshi2893/codebound/internal/calltree"
)

func BenchmarkGraphJSON(b *testing.B) {
	root := calltree.Function{Name: "main", Package: "example/app"}
	graph := &calltree.Graph{Root: root, Calls: make([]calltree.Call, 1000)}
	for index := range graph.Calls {
		graph.Calls[index] = calltree.Call{Caller: root, Callee: calltree.Function{Name: fmt.Sprintf("Task%d", index), Package: "example/worker"}}
	}
	handler := Handler(nil, graph)
	request := httptest.NewRequest("GET", "http://127.0.0.1/graph.json", nil)
	b.ReportAllocs()
	for b.Loop() {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != 200 {
			b.Fatal(response.Code)
		}
	}
}
