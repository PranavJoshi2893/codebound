package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/PranavJoshi2893/codebound/internal/calltree"
	"github.com/PranavJoshi2893/codebound/internal/calltree/viewer"
	"github.com/PranavJoshi2893/codebound/internal/tui"
	"github.com/spf13/cobra"
)

type calltreeOptions struct {
	path    string
	pattern string
	root    string
	format  string
	port    int
}

func defaultCalltreeOptions() calltreeOptions {
	return calltreeOptions{path: ".", pattern: "./...", format: "web"}
}

func (o calltreeOptions) validate() error {
	if strings.TrimSpace(o.path) == "" || strings.TrimSpace(o.pattern) == "" {
		return fmt.Errorf("path and pattern must not be empty")
	}
	switch o.format {
	case "web", "dot", "json":
	default:
		return fmt.Errorf("unsupported format %q; choose web, dot, or json", o.format)
	}
	if o.port < 0 || o.port > 65535 {
		return fmt.Errorf("port must be between 0 and 65535")
	}
	return nil
}

func newCalltreeCommand() *cobra.Command {
	options := defaultCalltreeOptions()
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
			if len(args) == 1 {
				if cmd.Flags().Changed("path") {
					return fmt.Errorf("provide a positional path or --path, not both")
				}
				options.path = args[0]
			}
			if cmd.Flags().Changed("root") && strings.TrimSpace(options.root) == "" {
				return fmt.Errorf("root must not be empty when explicitly specified")
			}
			if err := options.validate(); err != nil {
				return err
			}
			// Runtime errors need a concise diagnostic, not a full flag listing.
			cmd.SilenceUsage = true
			return runCalltreeWithSignals(cmd, options)
		},
	}
	flags := command.Flags()
	flags.StringVar(&options.path, "path", options.path, "Project or package directory")
	flags.StringVar(&options.pattern, "pattern", options.pattern, "Go package pattern")
	flags.StringVar(&options.root, "root", options.root, "Root function or Type.Method (default: discover main automatically)")
	flags.StringVar(&options.format, "format", options.format, "Output format: web, dot, or json")
	flags.IntVar(&options.port, "port", options.port, "Local browser-view port (default: choose an available port)")
	return command
}

func runCalltreeWithSignals(cmd *cobra.Command, options calltreeOptions) error {
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	err := runCalltree(ctx, cmd, options)
	if errors.Is(err, tui.ErrCancelled) {
		return nil
	}
	return err
}

func runCalltree(ctx context.Context, cmd *cobra.Command, options calltreeOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if options.format == "web" {
		if err := viewer.CheckGraphviz(); err != nil {
			return err
		}
	}
	analyzer := calltree.New(options.path)
	analyzer.Pattern = options.pattern
	analysis, err := analyzer.Load(ctx)
	if err != nil {
		return err
	}
	root, err := chooseCalltreeRoot(ctx, cmd, analysis, options)
	if err != nil {
		return err
	}
	graph, err := analysis.Analyze(ctx, root)
	if err != nil {
		return err
	}
	switch options.format {
	case "dot":
		return calltree.WriteDOT(cmd.OutOrStdout(), graph)
	case "json":
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		return encoder.Encode(graph)
	default:
		return serveCalltree(ctx, cmd, graph, options.port)
	}
}

func chooseCalltreeRoot(ctx context.Context, cmd *cobra.Command, analysis *calltree.Analysis, options calltreeOptions) (string, error) {
	if options.root != "" {
		return options.root, nil
	}
	roots := analysis.EntryPoints()
	if len(roots) == 1 {
		return roots[0], nil
	}
	if len(roots) > 1 && options.format == "web" && canPrompt(cmd) {
		return tui.ChooseRoot(ctx, cmd.InOrStdin(), cmd.OutOrStdout(), roots)
	}
	return "", &calltree.EntryPointError{Candidates: roots}
}

func serveCalltree(ctx context.Context, cmd *cobra.Command, graph *calltree.Graph, port int) error {
	svg, err := viewer.RenderSVG(ctx, graph)
	if err != nil {
		return err
	}
	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("listen for call graph: %w", err)
	}
	defer listener.Close()
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Call graph: http://%s\nPress Ctrl+C to stop.\n", listener.Addr()); err != nil {
		return err
	}
	return viewer.Serve(ctx, listener, svg)
}

func init() {
	rootCmd.AddCommand(newCalltreeCommand())
}
