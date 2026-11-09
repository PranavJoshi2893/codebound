package cmd

import (
	"fmt"
	"strings"

	"github.com/PranavJoshi2893/codebound/internal/app"
	"github.com/spf13/cobra"
)

func newCalltreeCommand() *cobra.Command {
	options := app.DefaultCalltreeOptions()
	command := &cobra.Command{
		Use:   "calltree [path]",
		Short: "Analyze Go calls and architectural boundaries",
		Long:  "Analyze module-local calls reachable from a Go function. Serve a browser graph, or export DOT/JSON. The browser view requires Graphviz (dot).",
		Args:  cobra.MaximumNArgs(1),
		Example: `  codebound calltree
  codebound calltree ~/Projects/my-project
  codebound calltree --root Server.Start --format dot > calltree.dot
  codebound calltree --root example.com/app.Run --format json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			request := options
			if len(args) == 1 {
				if cmd.Flags().Changed("path") {
					return fmt.Errorf("provide a positional path or --path, not both")
				}
				request.Path = args[0]
			}
			if cmd.Flags().Changed("root") && strings.TrimSpace(request.Root) == "" {
				return fmt.Errorf("root must not be empty when explicitly specified")
			}
			if err := request.Validate(); err != nil {
				return err
			}
			cmd.SilenceUsage = true
			return commandError(application(cmd).Calltree(cmd.Context(), request))
		},
	}
	flags := command.Flags()
	flags.StringVar(&options.Path, "path", options.Path, "Project or package directory")
	flags.StringVar(&options.Pattern, "pattern", options.Pattern, "Go package pattern")
	flags.StringVar(&options.Root, "root", options.Root, "Root function or Type.Method (default: discover main automatically)")
	flags.StringVar(&options.Format, "format", options.Format, "Output format: web, dot, or json")
	flags.IntVar(&options.Port, "port", options.Port, "Local browser-view port (default: choose an available port)")
	return command
}
