package viewer

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"

	"github.com/PranavJoshi2893/codebound/internal/calltree/model"
)

// Browser owns browser-view startup, loopback binding, and listener lifetime.
type Browser struct{}

func (Browser) Check() error { return CheckGraphviz() }

func (Browser) Show(ctx context.Context, output io.Writer, graph *model.Graph, port int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if port < 0 || port > 65535 {
		return fmt.Errorf("port must be between 0 and 65535")
	}
	svg, err := RenderSVG(ctx, graph)
	if err != nil {
		return err
	}
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return fmt.Errorf("listen for call graph: %w", err)
	}
	defer listener.Close()
	if _, err := fmt.Fprintf(output, "Call graph: http://%s\nPress Ctrl+C to stop.\n", listener.Addr()); err != nil {
		return err
	}
	return Serve(ctx, listener, svg, graph)
}
