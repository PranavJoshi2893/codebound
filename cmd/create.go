/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"log/slog"

	"github.com/PranavJoshi2893/codebound/internal/project"
	"github.com/spf13/cobra"
)

type CreateOptions = project.Options

var createCmd = &cobra.Command{
	Use:   "create [project-name]",
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

		if err := project.Create(opts); err != nil {
			slog.Error("failed to initialized", "project", opts.RootDir, "error", err)
			return err
		}

		slog.Info("Project successfully initialized", "project", opts.RootDir)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(createCmd)
	createCmd.Flags().StringP("mod", "m", "", "Go mod path (e.g., github.com/example/module_name)")
}
