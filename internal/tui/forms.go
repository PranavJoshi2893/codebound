package tui

import (
	"strings"

	"charm.land/huh/v2"
	"github.com/PranavJoshi2893/codebound/internal/project"
)

type answers struct {
	name       string
	custom     bool
	modulePath string
}

func (a *answers) options() project.Options {
	options := project.Options{RootDir: strings.TrimSpace(a.name)}
	if a.custom {
		options.Mod = strings.TrimSpace(a.modulePath)
	}
	return options
}

func projectForm(a *answers) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Key("name").Value(&a.name).
				Title("Project name?").Placeholder("my-project").
				Validate(func(value string) error {
					options := project.Options{RootDir: strings.TrimSpace(value)}
					return options.Validate()
				}),
		),
		huh.NewGroup(
			huh.NewConfirm().Key("custom").Value(&a.custom).
				Title("Use a custom module path?").
				DescriptionFunc(func() string {
					return "Choose No to use " + strings.TrimSpace(a.name) + " as the module path."
				}, &a.name).
				Affirmative("Yes").Negative("No"),
		),
		huh.NewGroup(
			huh.NewInput().Key("module").Value(&a.modulePath).
				Title("Module path?").Placeholder("github.com/username/project").
				Validate(func(value string) error {
					return project.ValidateModulePath(strings.TrimSpace(value))
				}),
		).WithHideFunc(func() bool { return !a.custom }),
	).WithTheme(huh.ThemeFunc(formTheme))
}
