// Package viewer renders and serves call graphs without depending on Cobra.
package viewer

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/PranavJoshi2893/codebound/internal/calltree"
)

// RenderSVG requires Graphviz's dot executable. DOT/JSON exports do not.
func RenderSVG(ctx context.Context, graph *calltree.Graph) ([]byte, error) {
	var dot, stderr bytes.Buffer
	if err := calltree.WriteDOT(&dot, graph); err != nil {
		return nil, fmt.Errorf("encode DOT: %w", err)
	}
	command := exec.CommandContext(ctx, "dot", "-Tsvg")
	command.Stdin = &dot
	command.Stderr = &stderr
	svg, err := command.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("render graph with Graphviz (install dot or use --format dot/json): %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return svg, nil
}
