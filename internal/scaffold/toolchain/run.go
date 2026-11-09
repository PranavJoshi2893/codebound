// Package toolchain runs the external setup tools with bounded diagnostics and
// caller-owned cancellation. Generators can substitute a Runner in tests.
package toolchain

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type Runner interface {
	Run(context.Context, string, string, ...string) error
}

type System struct{}

func (System) Run(ctx context.Context, directory, name string, args ...string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = directory
	command.WaitDelay = 2 * time.Second
	output := diagnosticBuffer{remaining: 64 << 10}
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("failed to run %s %s: %w (output: %s)", name, strings.Join(args, " "), err, output.String())
	}
	return nil
}

// Discard excess diagnostics without blocking the child or retaining unbounded
// output. Stdout and stderr share one writer, which os/exec serializes.
type diagnosticBuffer struct {
	buffer    bytes.Buffer
	remaining int
}

func (b *diagnosticBuffer) String() string { return b.buffer.String() }

func (b *diagnosticBuffer) Write(data []byte) (int, error) {
	n := len(data)
	keep := min(n, b.remaining)
	_, _ = b.buffer.Write(data[:keep])
	b.remaining -= keep
	return n, nil
}
