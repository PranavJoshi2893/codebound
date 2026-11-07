package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
)

func TestAnswersOptions(t *testing.T) {
	for _, custom := range []bool{false, true} {
		a := answers{name: " project ", custom: custom, modulePath: " github.com/user/project "}
		options := a.options()
		want := ""
		if custom {
			want = "github.com/user/project"
		}
		if options.RootDir != "project" || options.Mod != want {
			t.Fatalf("unexpected options: %+v", options)
		}
		if err := options.Validate(); err != nil {
			t.Fatal(err)
		}
	}
}

// Advance through Huh's public navigation API, stopping before executing
// the new field's cursor commands. This keeps the test independent of timers.
func nextGroup(t *testing.T, m model) model {
	t.Helper()
	previous := m.form.GetFocusedField().GetKey()
	updated, cmd := m.Update(huh.NextField())
	m = updated.(model)
	queue := []tea.Cmd{cmd}
	for count := 0; len(queue) > 0 && count < 100; count++ {
		if m.form.State == huh.StateCompleted || m.form.GetFocusedField().GetKey() != previous {
			return m
		}
		cmd, queue = queue[0], queue[1:]
		if cmd == nil {
			continue
		}
		msg := cmd()
		if batch, ok := msg.(tea.BatchMsg); ok {
			queue = append(queue, batch...)
			continue
		}
		updated, cmd = m.Update(msg)
		m = updated.(model)
		queue = append(queue, cmd)
	}
	t.Fatal("form did not advance")
	return m
}

func TestConditionalModuleGroup(t *testing.T) {
	for _, custom := range []bool{false, true} {
		a := &answers{name: "project", custom: custom, modulePath: "github.com/user/project"}
		m := model{answers: a, form: projectForm(a)}
		original := m.form
		m = nextGroup(t, m)
		if key := m.form.GetFocusedField().GetKey(); key != "custom" {
			t.Fatalf("expected confirmation, got %q", key)
		}
		m = nextGroup(t, m)
		if custom {
			if key := m.form.GetFocusedField().GetKey(); key != "module" {
				t.Fatalf("expected custom module field, got %q", key)
			}
			m = nextGroup(t, m)
		}
		if m.form.State != huh.StateCompleted || m.form != original {
			t.Fatal("expected the same form to complete")
		}
	}
}

func TestCancellationDoesNotCompleteForm(t *testing.T) {
	m := initialModel()
	updated, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if updated.(model).form.State == huh.StateCompleted || cmd == nil {
		t.Fatal("cancellation submitted the form")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("cancellation did not quit")
	}
}

func TestFormSize(t *testing.T) {
	m := initialModel()
	m.windowSize = tea.WindowSizeMsg{Width: 80, Height: 24}
	if size := m.formSize(); size.Width != 76 || size.Height >= 24 || size.Height < 1 {
		t.Fatalf("unexpected size: %+v", size)
	}
	m.windowSize = tea.WindowSizeMsg{Width: 1, Height: 1}
	if size := m.formSize(); size.Width != 1 || size.Height != 1 {
		t.Fatalf("unexpected tiny terminal size: %+v", size)
	}
}
