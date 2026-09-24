/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"unicode"

	"github.com/PranavJoshi2893/codebound/internal/scaffold"
	"github.com/spf13/cobra"
)

type ModuleOptions struct {
	Name string
}

func (o *ModuleOptions) Validate() error {
	if len(o.Name) < 5 {
		return fmt.Errorf("api name must be at least 5 characters long")
	}

	for _, r := range o.Name {
		if !unicode.IsLetter(r) {
			return fmt.Errorf("api name can only contain letters: %c)", r)
		}
	}

	return nil
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

		scaffold.CreateAPIModule(modOpts.Name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(moduleCmd)
}
