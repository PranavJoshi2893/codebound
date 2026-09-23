/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"log/slog"
	"unicode"

	"github.com/PranavJoshi2893/codebound/internal/scaffold"
	"github.com/spf13/cobra"
)

type CreateOptions struct {
	RootDir string
	Mod     string
}

func (o *CreateOptions) Validate() error {
	if len(o.RootDir) < 5 {
		return fmt.Errorf("project name must be at least 5 characters long")
	}

	for i, r := range o.RootDir {
		if i == 0 && !unicode.IsLetter(r) {
			return fmt.Errorf("project name must start with an English letter")
		}

		if !unicode.IsLetter(r) &&
			!unicode.IsDigit(r) &&
			r != '_' &&
			r != '-' {
			return fmt.Errorf("project name can only contain letters, numbers, hyphens (-), and underscores (_) (invalid character: %c)", r)
		}
	}

	return nil
}

var initCmd = &cobra.Command{
	Use:   "init [project-name]",
	Short: "Initialize a new project using CodeBound structure",
	Long: `
Initialize a new project layout adhering to the opinionated 
architecture and boundary rules defined by CodeBound.
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		modFlag, err := cmd.Flags().GetString("mod")
		if err != nil {
			slog.Error("Failed to read mod flag", "error", err)
			return fmt.Errorf("failed to read mod flag: %w", err)
		}

		opts := CreateOptions{
			RootDir: args[0],
			Mod:     modFlag,
		}

		if err := opts.Validate(); err != nil {
			slog.Error("Validation failed for project initialization", "error", err)
			return err
		}

		slog.Info("Initializing project structure", "project", opts.RootDir)

		if err := scaffold.InitializeProject(opts.RootDir, opts.Mod); err != nil {
			slog.Error("failed to initialized", "project", opts.RootDir, "error", err)
			return err
		}

		slog.Info("Project successfully initialized", "project", opts.RootDir)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().StringP("mod", "m", "", "Go mod path (e.g., github.com/example/module_name)")
}
