/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"github.com/PranavJoshi2893/codebound/internal/scaffold/module"
	"github.com/spf13/cobra"
)

type ModuleOptions struct {
	Name string
}

func (o *ModuleOptions) Validate() error {
	return module.ValidateName(o.Name)
}

var moduleCmd = &cobra.Command{
	Use:   "module",
	Short: "Create a new API module using the CodeBound structure",
	Long:  `Create a new API module following the CodeBound project structure.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {

		modOpts := ModuleOptions{
			Name: args[0],
		}

		if err := modOpts.Validate(); err != nil {
			return err
		}

		return module.CreateAPIModule(modOpts.Name)
	},
}

func init() {
	rootCmd.AddCommand(moduleCmd)
}
