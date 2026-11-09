package cmd

import (
	"os"

	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
)

func canPrompt(cmd *cobra.Command) bool {
	input, inputOK := cmd.InOrStdin().(*os.File)
	output, outputOK := cmd.OutOrStdout().(*os.File)
	return inputOK && outputOK && term.IsTerminal(input.Fd()) && term.IsTerminal(output.Fd())
}

func interactiveOrHelp(cmd *cobra.Command, _ []string) error {
	if !canPrompt(cmd) {
		return cmd.Help()
	}
	cmd.SilenceUsage = true
	return commandError(application(cmd).Interactive(cmd.Context()))
}
