package tui

import (
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
)

var (
	accentColor    = lipgloss.Color("#00875F")
	appStyle       = lipgloss.NewStyle().Margin(1, 2)
	titleStyle     = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
	mutedStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	labelStyle     = mutedStyle.Width(14)
	completionMark = formTheme(false).Focused.SelectedPrefix.String()
)

func formTheme(isDark bool) *huh.Styles {
	theme := huh.ThemeCharm(isDark)
	theme.Focused.Base = theme.Focused.Base.
		BorderStyle(lipgloss.NormalBorder()).BorderForeground(accentColor)
	theme.Focused.Title = theme.Focused.Title.Foreground(accentColor)
	theme.Focused.FocusedButton = theme.Focused.FocusedButton.Background(accentColor)
	theme.Focused.SelectedPrefix = theme.Focused.SelectedPrefix.Foreground(accentColor)
	theme.Focused.TextInput.Cursor = theme.Focused.TextInput.Cursor.Foreground(accentColor)
	return theme
}

func completedAnswer(label, value string) string {
	return completionMark + labelStyle.Render(label) + value + "\n"
}
