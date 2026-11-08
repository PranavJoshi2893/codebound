/*
Copyright © 2026 Pranav_Joshi <EMAIL ADDRESS>
*/
package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "codebound",
	Short: "Define and enforce code boundaries",
	Long: `CodeBound is an opinionated tool for creating and maintaining
consistent project structures and architectural boundaries.

It helps you establish boundaries between components,
enforce architectural rules, and keep your codebase aligned
with the structure you define.

Run without arguments in a terminal to create a project or view a call tree,
or use create <project-name> for non-interactive creation.`,
	Version: "0.1.4-nightly.4",
	Args:    cobra.NoArgs,
	RunE:    interactiveOrHelp,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
