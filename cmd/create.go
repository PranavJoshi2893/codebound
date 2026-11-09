package cmd

import (
	"github.com/PranavJoshi2893/codebound/internal/project"
	"github.com/spf13/cobra"
)

func newCreateCommand() *cobra.Command {
	var mod string
	command := &cobra.Command{
		Use:   "create [project-name]",
		Short: "Initialize a new project using CodeBound structure",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			options := project.Options{RootDir: args[0], Mod: mod}
			if err := options.Validate(); err != nil {
				return err
			}
			cmd.SilenceUsage = true
			return application(cmd).Create(cmd.Context(), options)
		},
	}
	command.Flags().StringVarP(&mod, "mod", "m", "", "Go mod path (e.g., github.com/example/module_name)")
	return command
}
