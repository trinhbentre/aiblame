// Command aiblame reports how much of a git repository was written with AI.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/trinhbentre/aiblame/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cwd, _ := os.Getwd()
	code := cli.Main(ctx, os.Args[1:], cli.Env{Stdout: os.Stdout, Stderr: os.Stderr, Getenv: os.Getenv, Cwd: cwd})
	stop()
	os.Exit(code)
}
