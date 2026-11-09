package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/PranavJoshi2893/codebound/internal/calltree"
	"github.com/PranavJoshi2893/codebound/internal/project"
)

type fakeAnalysis struct {
	roots []string
	graph *calltree.Graph
	root  string
	err   error
}

func (a *fakeAnalysis) EntryPoints() []string { return a.roots }
func (a *fakeAnalysis) Analyze(_ context.Context, root string) (*calltree.Graph, error) {
	a.root = root
	return a.graph, a.err
}

type fakeBrowser struct {
	checkError    error
	showError     error
	checks, shows int
	port          int
	graph         *calltree.Graph
}

func (b *fakeBrowser) Check() error { b.checks++; return b.checkError }
func (b *fakeBrowser) Show(_ context.Context, _ io.Writer, graph *calltree.Graph, port int) error {
	b.shows++
	b.graph, b.port = graph, port
	return b.showError
}

type fakePrompter struct {
	action  Action
	options project.Options
	path    string
	roots   []string
	choice  string
	err     error
}

func (p *fakePrompter) ChooseAction(context.Context) (Action, error)     { return p.action, p.err }
func (p *fakePrompter) Project(context.Context) (project.Options, error) { return p.options, p.err }
func (p *fakePrompter) CalltreePath(context.Context) (string, error)     { return p.path, p.err }
func (p *fakePrompter) ChooseRoot(_ context.Context, roots []string) (string, error) {
	p.roots = roots
	return p.choice, p.err
}

func TestCalltreeRootPolicy(t *testing.T) {
	for _, test := range []struct {
		name, format, explicit, want         string
		roots                                []string
		interactive, wantPrompt, wantChoices bool
	}{
		{name: "single", format: "json", roots: []string{"app.main"}, want: "app.main"},
		{name: "explicit library", format: "json", explicit: "app.Run", want: "app.Run"},
		{name: "explicit skips selector", format: "web", explicit: "worker.main", roots: []string{"api.main", "worker.main"}, interactive: true, want: "worker.main"},
		{name: "browser selector", format: "web", roots: []string{"api.main", "worker.main"}, interactive: true, wantPrompt: true, want: "worker.main"},
		{name: "json never prompts", format: "json", roots: []string{"api.main", "worker.main"}, interactive: true, wantChoices: true},
		{name: "dot never prompts", format: "dot", roots: []string{"api.main", "worker.main"}, interactive: true, wantChoices: true},
		{name: "redirected browser", format: "web", roots: []string{"api.main", "worker.main"}, wantChoices: true},
		{name: "library", format: "web", interactive: true, wantChoices: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			analysis := &fakeAnalysis{roots: test.roots, graph: &calltree.Graph{}}
			browser := &fakeBrowser{}
			prompt := &fakePrompter{choice: "worker.main"}
			dependencies := Dependencies{
				Output: io.Discard, Browser: browser,
				Load: func(context.Context, string, string) (Analysis, error) { return analysis, nil },
			}
			if test.interactive {
				dependencies.Prompter = prompt
			}
			options := DefaultCalltreeOptions()
			options.Format, options.Root = test.format, test.explicit
			err := New(dependencies).Calltree(context.Background(), options)
			if test.wantChoices {
				var choices *calltree.EntryPointError
				if !errors.As(err, &choices) || !reflect.DeepEqual(choices.Candidates, test.roots) {
					t.Fatalf("expected choices %v, got %v", test.roots, err)
				}
			} else if err != nil || analysis.root != test.want {
				t.Fatalf("root=%q error=%v, want %q", analysis.root, err, test.want)
			}
			if (prompt.roots != nil) != test.wantPrompt {
				t.Fatalf("unexpected selector call: %v", prompt.roots)
			}
			if test.format != "web" && (browser.checks != 0 || browser.shows != 0) {
				t.Fatal("export used browser dependency")
			}
		})
	}
}

func TestCalltreeValidatesBeforeDependencies(t *testing.T) {
	for _, options := range []CalltreeOptions{
		{Path: "", Pattern: "./...", Format: "json"},
		{Path: ".", Pattern: " ", Format: "json"},
		{Path: ".", Pattern: "./...", Format: "invalid"},
		{Path: ".", Pattern: "./...", Format: "web", Port: 65536},
	} {
		if err := New(Dependencies{}).Calltree(context.Background(), options); err == nil {
			t.Fatalf("accepted %+v", options)
		}
	}
}

func TestCalltreeDependencyFailures(t *testing.T) {
	failure := errors.New("dependency failure")
	for _, stage := range []string{"check", "load", "analyze", "show"} {
		t.Run(stage, func(t *testing.T) {
			browser := &fakeBrowser{}
			analysis := &fakeAnalysis{roots: []string{"main"}, graph: &calltree.Graph{}}
			loads := 0
			if stage == "check" {
				browser.checkError = failure
			}
			if stage == "analyze" {
				analysis.err = failure
			}
			if stage == "show" {
				browser.showError = failure
			}
			a := New(Dependencies{Output: io.Discard, Browser: browser, Load: func(context.Context, string, string) (Analysis, error) {
				loads++
				if stage == "load" {
					return nil, failure
				}
				return analysis, nil
			}})
			if err := a.Calltree(context.Background(), DefaultCalltreeOptions()); !errors.Is(err, failure) {
				t.Fatalf("lost failure: %v", err)
			}
			if stage == "check" && loads != 0 {
				t.Fatal("loaded source before checking optional dependency")
			}
			if stage != "show" && browser.shows != 0 {
				t.Fatal("show called after failure")
			}
		})
	}
}

func TestCalltreeExportsEvidenceAndLoadOptions(t *testing.T) {
	graph := &calltree.Graph{Root: calltree.Function{Name: "Run", Package: "app"}}
	var output bytes.Buffer
	key := struct{}{}
	ctx := context.WithValue(context.Background(), key, "caller")
	a := New(Dependencies{Output: &output, Load: func(got context.Context, path, pattern string) (Analysis, error) {
		if got != ctx || path != "project" || pattern != "./cmd/api" {
			t.Fatal("request changed at application boundary")
		}
		return &fakeAnalysis{graph: graph}, nil
	}})
	options := CalltreeOptions{Path: "project", Pattern: "./cmd/api", Root: "app.Run", Format: "json"}
	if err := a.Calltree(ctx, options); err != nil {
		t.Fatal(err)
	}
	var result calltree.Graph
	if err := json.Unmarshal(output.Bytes(), &result); err != nil || !reflect.DeepEqual(&result, graph) {
		t.Fatalf("changed evidence: %s, %v", output.String(), err)
	}
}

func TestInteractiveCreateSharesUseCase(t *testing.T) {
	options := project.Options{RootDir: "example", Mod: "example.com/app"}
	prompt := &fakePrompter{action: CreateProject, options: options}
	var output bytes.Buffer
	creates := 0
	a := New(Dependencies{Prompter: prompt, Output: &output, CreateProject: func(_ context.Context, got project.Options) error {
		creates++
		if got != options {
			t.Fatalf("changed options: %+v", got)
		}
		return nil
	}})
	if err := a.Interactive(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := a.Create(context.Background(), options); err != nil {
		t.Fatal(err)
	}
	if creates != 2 || strings.Count(output.String(), "Project example created.") != 2 {
		t.Fatalf("different creation paths: %d, %s", creates, output.String())
	}
}

func TestInteractiveCalltreeSharesUseCase(t *testing.T) {
	browser := &fakeBrowser{}
	prompt := &fakePrompter{action: ViewCalltree, path: "chosen-project"}
	a := New(Dependencies{Prompter: prompt, Output: io.Discard, Browser: browser, Load: func(_ context.Context, path, pattern string) (Analysis, error) {
		if path != prompt.path || pattern != "./..." {
			t.Fatal("wizard changed analysis defaults")
		}
		return &fakeAnalysis{roots: []string{"main"}, graph: &calltree.Graph{}}, nil
	}})
	if err := a.Interactive(context.Background()); err != nil {
		t.Fatal(err)
	}
	if browser.checks != 1 || browser.shows != 1 {
		t.Fatal("wizard bypassed browser workflow")
	}
}

func TestCancelledWorkDoesNotTouchDependencies(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a := New(Dependencies{})
	for _, run := range []func() error{
		func() error { return a.Interactive(ctx) },
		func() error { return a.Create(ctx, project.Options{}) },
		func() error { return a.Module(ctx, "users") },
		func() error { return a.Calltree(ctx, DefaultCalltreeOptions()) },
	} {
		if err := run(); !errors.Is(err, context.Canceled) {
			t.Fatalf("expected cancellation, got %v", err)
		}
	}
}

func TestInteractiveCancellationAndUnknownAction(t *testing.T) {
	for _, prompt := range []*fakePrompter{{err: ErrCancelled}, {action: "unknown"}} {
		if err := New(Dependencies{Prompter: prompt}).Interactive(context.Background()); err == nil {
			t.Fatal("silently accepted aborted or unknown action")
		}
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestOutputFailuresAreReturned(t *testing.T) {
	a := New(Dependencies{Output: failingWriter{}, CreateProject: func(context.Context, project.Options) error { return nil }, Load: func(context.Context, string, string) (Analysis, error) {
		return &fakeAnalysis{roots: []string{"main"}, graph: &calltree.Graph{}}, nil
	}})
	if err := a.Create(context.Background(), project.Options{RootDir: "example"}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	for _, format := range []string{"dot", "json"} {
		options := DefaultCalltreeOptions()
		options.Format = format
		if err := a.Calltree(context.Background(), options); !errors.Is(err, io.ErrClosedPipe) {
			t.Fatal(err)
		}
	}
}
