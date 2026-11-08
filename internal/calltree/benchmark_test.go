package calltree

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkWriteDOT(b *testing.B) {
	root := Function{Name: "main", Package: "app", File: "main.go", Line: 1}
	graph := &Graph{Root: root, Calls: make([]Call, 10000)}
	for i := range graph.Calls {
		graph.Calls[i] = Call{
			Caller: root,
			Callee: Function{Name: fmt.Sprintf("Task%d", i), Package: "app", File: "task.go", Line: i + 1},
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := WriteDOT(io.Discard, graph); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBrowserDOT(b *testing.B) {
	benchmarkBrowserDOT(b, false)
}

func BenchmarkIndexedBrowserDOT(b *testing.B) {
	benchmarkBrowserDOT(b, true)
}

func benchmarkBrowserDOT(b *testing.B, reuse bool) {
	b.Helper()
	root := Function{Name: "main", Package: "example/app"}
	graph := &Graph{Root: root, Calls: make([]Call, 10000)}
	previous := root
	for index := range graph.Calls {
		next := Function{Name: fmt.Sprintf("Task%d", index), Package: "example/worker", Line: index + 1}
		graph.Calls[index] = Call{Caller: previous, Callee: next}
		previous = next
	}
	indexed := IndexGraph(graph)
	for _, test := range []struct {
		name  string
		focus int
	}{{"Whole", -1}, {"Focused", 5000}} {
		b.Run(test.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				var err error
				if reuse {
					err = indexed.WriteBrowserDOT(io.Discard, BrowserOptions{Focus: test.focus})
				} else {
					err = IndexGraph(graph).WriteBrowserDOT(io.Discard, BrowserOptions{Focus: test.focus})
				}
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkAnalyzeLoaded(b *testing.B) {
	dir := b.TempDir()
	b.Setenv("GOWORK", "off")
	for name, content := range map[string]string{
		"go.mod":  "module example.com/bench\n\ngo 1.24\n",
		"main.go": callChainSource(1000),
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			b.Fatal(err)
		}
	}
	analysis, err := New(dir).Load(context.Background())
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := analysis.Analyze(context.Background(), "main"); err != nil {
			b.Fatal(err)
		}
	}
}
