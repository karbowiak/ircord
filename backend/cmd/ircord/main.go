package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"ircord/backend/internal/app"
	"ircord/backend/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := cli.Execute(ctx, os.Args[1:], os.Stdout, os.Stderr, app.Run)
	stop()
	os.Exit(code)
}
