// Package project defines the shared project creation boundary for CLI and TUI.
package project

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/PranavJoshi2893/codebound/internal/scaffold/create"
	"golang.org/x/mod/module"
)

type Options struct {
	RootDir string
	Mod     string
}

// ValidateName preserves the naming rules of the existing create command.
func ValidateName(name string) error {
	if len(name) < 5 {
		return fmt.Errorf("project name must be at least 5 characters long")
	}
	for i, r := range name {
		if i == 0 && !unicode.IsLetter(r) {
			return fmt.Errorf("project name must start with an English letter")
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' {
			return fmt.Errorf("project name can only contain letters, numbers, hyphens (-), and underscores (_) (invalid character: %c)", r)
		}
	}
	return nil
}

func ValidateModulePath(path string) error {
	if path == "" {
		return fmt.Errorf("module path is required")
	}
	if strings.HasPrefix(path, "-") {
		return fmt.Errorf("module path must not start with a dash")
	}
	// Local module names are supported by go mod init too.
	first, _, _ := strings.Cut(path, "/")
	if !strings.Contains(first, ".") {
		path = "local.invalid/" + path
	}
	if err := module.CheckPath(path); err != nil {
		return fmt.Errorf("invalid module path: %w", err)
	}
	return nil
}

func (o Options) Validate() error {
	if err := ValidateName(o.RootDir); err != nil {
		return err
	}
	path := o.Mod
	if path == "" {
		path = o.RootDir
	}
	return ValidateModulePath(path)
}

func Create(options Options) error {
	if err := options.Validate(); err != nil {
		return err
	}
	return create.InitializeProject(options.RootDir, options.Mod)
}
