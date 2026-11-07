// Package tui collects project options. It never creates files or runs commands.
package tui

import (
	"context"
	"errors"
	"io"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/PranavJoshi2893/codebound/internal/project"
)

var ErrCancelled = errors.New("project setup cancelled")

// Run returns validated options after submission or ErrCancelled on cancellation.
// File creation belongs to the caller, after the terminal has been restored.
func Run(ctx context.Context, input io.Reader, output io.Writer) (project.Options, error) {
	final, err := tea.NewProgram(initialModel(), tea.WithContext(ctx),
		tea.WithInput(input), tea.WithOutput(output)).Run()
	if err != nil {
		return project.Options{}, err
	}
	m := final.(model)
	if m.form.State != huh.StateCompleted {
		return project.Options{}, ErrCancelled
	}
	options := m.answers.options()
	return options, options.Validate()
}

type model struct {
	form       *huh.Form
	answers    *answers
	windowSize tea.WindowSizeMsg
}

func initialModel() model {
	a := &answers{}
	return model{form: projectForm(a), answers: a}
}

func (m model) Init() tea.Cmd { return m.form.Init() }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.windowSize = size
		msg = m.formSize()
	}
	if key, ok := msg.(tea.KeyPressMsg); ok && key.String() == "ctrl+c" {
		return m, tea.Quit
	}
	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}
	if m.form.State == huh.StateCompleted || m.form.State == huh.StateAborted {
		return m, tea.Quit
	}
	if m.windowSize.Width > 0 {
		_, sizeCmd := m.form.Update(m.formSize())
		cmd = tea.Batch(cmd, sizeCmd)
	}
	return m, cmd
}

func (m model) formSize() tea.WindowSizeMsg {
	width := max(1, m.windowSize.Width-appStyle.GetHorizontalFrameSize())
	header, history := m.heading()
	prefix := lipgloss.NewStyle().Width(width).Render(header + history)
	return tea.WindowSizeMsg{
		Width:  width,
		Height: max(1, m.windowSize.Height-appStyle.GetVerticalFrameSize()-lipgloss.Height(prefix)+1),
	}
}

func (m model) heading() (string, string) {
	header := titleStyle.Render("Codebound") + "  " + mutedStyle.Render("New Go project") + "\n\n"
	var history string
	done := m.form.State == huh.StateCompleted
	field := m.form.GetFocusedField().GetKey()
	if done || field != "name" {
		history += completedAnswer("Project name", strings.TrimSpace(m.answers.name))
	}
	if done || field == "module" {
		answer := "No"
		if m.answers.custom {
			answer = "Yes"
		}
		history += completedAnswer("Custom module", answer)
	}
	return header, history
}

func (m model) View() tea.View {
	header, history := m.heading()
	body := m.form.View()
	if m.form.State == huh.StateCompleted {
		options := m.answers.options()
		path := options.Mod
		if path == "" {
			path = options.RootDir
		}
		body = completedAnswer("Module path", path)
	}
	return tea.NewView(appStyle.Render(header + history + body))
}
