// Package viewer renders and serves call graphs without depending on Cobra.
package viewer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/PranavJoshi2893/codebound/internal/calltree"
)

const (
	renderTimeout      = 30 * time.Second
	maxDOTBytes        = 8 << 20
	maxSVGBytes        = 32 << 20
	maxDiagnosticBytes = 64 << 10
)

var errOutputLimit = errors.New("graph rendering output exceeds size limit")

// CheckGraphviz checks the optional browser-view dependency before analysis.
func CheckGraphviz() error {
	if _, err := exec.LookPath("dot"); err != nil {
		return errors.New("Graphviz (dot) is required for browser view; install Graphviz or use --format dot/json")
	}
	return nil
}

type limitedBuffer struct {
	buffer bytes.Buffer
	limit  int
}

func (b *limitedBuffer) Bytes() []byte  { return b.buffer.Bytes() }
func (b *limitedBuffer) String() string { return b.buffer.String() }

func (b *limitedBuffer) Write(data []byte) (int, error) {
	remaining := b.limit - b.buffer.Len()
	if len(data) > remaining {
		n, _ := b.buffer.Write(data[:remaining])
		return n, errOutputLimit
	}
	return b.buffer.Write(data)
}

// RenderSVG requires Graphviz's dot executable. DOT/JSON exports do not.
func RenderSVG(ctx context.Context, graph *calltree.Graph) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, renderTimeout)
	defer cancel()
	dot := limitedBuffer{limit: maxDOTBytes}
	svg := limitedBuffer{limit: maxSVGBytes}
	stderr := limitedBuffer{limit: maxDiagnosticBytes}
	if err := calltree.WriteDOT(&dot, graph); err != nil {
		return nil, fmt.Errorf("encode DOT: %w", err)
	}
	command := exec.CommandContext(ctx, "dot", "-Tsvg")
	command.Stdin = bytes.NewReader(dot.Bytes())
	command.Stdout = &svg
	command.Stderr = &stderr
	// Bound waiting for inherited pipes if a renderer child outlives dot.
	command.WaitDelay = 2 * time.Second
	err := command.Run()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("render graph with Graphviz (install dot or use --format dot/json): %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return svg.Bytes(), nil
}
