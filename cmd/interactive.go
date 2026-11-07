package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/PranavJoshi2893/codebound/internal/project"
	"github.com/PranavJoshi2893/codebound/internal/tui"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
)

func runInteractive(cmd *cobra.Command, _ []string) error {
	options, err := tui.Run(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout())
	if errors.Is(err, tui.ErrCancelled) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := project.Create(options); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Project %s created.\n\n  cd %s\n  go run ./cmd/api\n", options.RootDir, options.RootDir)
	return nil
}

// Keep redirected invocations non-interactive rather than waiting for prompts.
func interactiveOrHelp(cmd *cobra.Command, args []string) error {
	input, ok := cmd.InOrStdin().(*os.File)
	if !ok || !term.IsTerminal(input.Fd()) {
		return cmd.Help()
	}
	return runInteractive(cmd, args)
}
