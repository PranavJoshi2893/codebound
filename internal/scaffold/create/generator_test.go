package create

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type recordingTools struct {
	calls     [][]string
	directory string
	failAt    int
	err       error
	cancel    context.CancelFunc
}

func (r *recordingTools) Run(_ context.Context, directory, name string, args ...string) error {
	r.directory = directory
	r.calls = append(r.calls, append([]string{name}, args...))
	if r.cancel != nil {
		r.cancel()
	}
	if len(r.calls) == r.failAt {
		return r.err
	}
	return nil
}

func TestGeneratorToolSequence(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "project")
	tools := &recordingTools{}
	if err := (Generator{Tools: tools}).Create(context.Background(), dir, "example.com/app"); err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"go", "mod", "init", "--", "example.com/app"}, {"go", "mod", "tidy"}, {"git", "init"}}
	if !reflect.DeepEqual(tools.calls, want) || tools.directory != dir {
		t.Fatalf("unexpected tool sequence: %v", tools.calls)
	}
	if _, err := os.Stat(filepath.Join(dir, "cmd/api/main.go")); err != nil {
		t.Fatal(err)
	}
}

func TestGeneratorStopsAtToolFailure(t *testing.T) {
	failure := errors.New("tool failure")
	for stage := 1; stage <= 3; stage++ {
		dir := filepath.Join(t.TempDir(), "project")
		tools := &recordingTools{failAt: stage, err: failure}
		if err := (Generator{Tools: tools}).Create(context.Background(), dir, "example.com/app"); !errors.Is(err, failure) {
			t.Fatalf("stage %d: %v", stage, err)
		}
		if len(tools.calls) != stage {
			t.Fatal("continued after tool failure")
		}
	}
}

func TestGeneratorCancellationBeforeAndDuringSetup(t *testing.T) {
	for _, before := range []bool{true, false} {
		dir := filepath.Join(t.TempDir(), "project")
		ctx, cancel := context.WithCancel(context.Background())
		tools := &recordingTools{cancel: cancel}
		if before {
			cancel()
		}
		err := (Generator{Tools: tools}).Create(ctx, dir, "example.com/app")
		cancel()
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
		if before {
			if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("cancelled generation created directory")
			}
		} else {
			if len(tools.calls) != 1 {
				t.Fatal("continued running tools after cancellation")
			}
			if _, err := os.Stat(filepath.Join(dir, ".env")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("wrote files after cancellation")
			}
		}
	}
}
