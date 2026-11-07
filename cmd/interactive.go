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
	err := runInteractiveAction(cmd)
	if errors.Is(err, tui.ErrCancelled) {
		return nil
	}
	return err
}

func runInteractiveAction(cmd *cobra.Command) error {
	action, err := tui.ChooseAction(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout())
	if err != nil {
		return err
	}
	if action == tui.ViewCalltree {
		path, err := tui.CalltreePath(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout())
		if err != nil {
			return err
		}
		options := defaultCalltreeOptions()
		options.path = path
		cmd.SilenceUsage = true
		return runCalltreeWithSignals(cmd, options)
	}
	return runCreateWizard(cmd)
}

func runCreateWizard(cmd *cobra.Command) error {
	options, err := tui.Run(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout())
	if err != nil {
		return err
	}
	if err := project.Create(options); err != nil {
		return err
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "Project %s created.\n\n  cd %s\n  go run ./cmd/api\n", options.RootDir, options.RootDir)
	return err
}

func canPrompt(cmd *cobra.Command) bool {
	input, inputOK := cmd.InOrStdin().(*os.File)
	output, outputOK := cmd.OutOrStdout().(*os.File)
	return inputOK && outputOK && term.IsTerminal(input.Fd()) && term.IsTerminal(output.Fd())
}

// Keep redirected invocations non-interactive rather than waiting for prompts.
func interactiveOrHelp(cmd *cobra.Command, args []string) error {
	if !canPrompt(cmd) {
		return cmd.Help()
	}
	return runInteractive(cmd, args)
}
