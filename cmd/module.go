package cmd

import (
	"github.com/PranavJoshi2893/codebound/internal/scaffold/module"
	"github.com/spf13/cobra"
)

func newModuleCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "module [name]",
		Short: "Create a new API module using the CodeBound structure",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := module.ValidateName(args[0]); err != nil {
				return err
			}
			cmd.SilenceUsage = true
			return application(cmd).Module(cmd.Context(), args[0])
		},
	}
}
