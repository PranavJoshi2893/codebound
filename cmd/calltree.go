package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/PranavJoshi2893/codebound/internal/calltree"
	"github.com/PranavJoshi2893/codebound/internal/calltree/viewer"
	"github.com/spf13/cobra"
)

type calltreeOptions struct {
	path    string
	pattern string
	root    string
	format  string
	port    int
}

func (o calltreeOptions) validate() error {
	if strings.TrimSpace(o.path) == "" || strings.TrimSpace(o.pattern) == "" || strings.TrimSpace(o.root) == "" {
		return fmt.Errorf("path, pattern, and root must not be empty")
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
	var options calltreeOptions
	command := &cobra.Command{
		Use:   "calltree",
		Short: "Analyze Go calls and architectural boundaries",
		Long:  "Analyze module-local calls reachable from a Go function. Serve a browser graph, or export DOT/JSON. The browser view requires Graphviz (dot).",
		Args:  cobra.NoArgs,
		Example: `  codebound calltree --path . --pattern ./... --root main
  codebound calltree --root Server.Start --format dot > calltree.dot
  codebound calltree --root example.com/app.Run --format json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := options.validate(); err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return runCalltree(ctx, cmd, options)
		},
	}
	flags := command.Flags()
	flags.StringVar(&options.path, "path", ".", "Project or package directory")
	flags.StringVar(&options.pattern, "pattern", ".", "Go package pattern (use ./... for all packages)")
	flags.StringVar(&options.root, "root", "main", "Root function, Type.Method, or package-qualified name")
	flags.StringVar(&options.format, "format", "web", "Output format: web, dot, or json")
	flags.IntVar(&options.port, "port", 3000, "Local browser-view port (0 chooses an available port)")
	return command
}

func runCalltree(ctx context.Context, cmd *cobra.Command, options calltreeOptions) error {
	analyzer := calltree.New(options.path)
	analyzer.Pattern = options.pattern
	graph, err := analyzer.Analyze(ctx, options.root)
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
