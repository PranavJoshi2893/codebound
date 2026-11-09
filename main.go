/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/PranavJoshi2893/codebound/cmd"
)

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := cmd.NewCommand().ExecuteContext(ctx); err != nil {
		return 1
	}
	return 0
}
