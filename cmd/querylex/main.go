package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/querylex/querylex/internal/rootcmd"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := rootcmd.RootCmd.ExecuteContext(ctx); err != nil {
		if ctx.Err() != nil {
			// Context was cancelled by signal — defers already ran (lock.Release())
			os.Exit(130) // 128 + SIGINT(2)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
