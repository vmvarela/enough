package main

import (
	"context"
	"github.com/vmvarela/enough/internal/analysis"
	"github.com/vmvarela/enough/internal/cli"
	"os"
	"os/signal"
	"time"
)

var version = "0.1.1"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	os.Exit(cli.Run(ctx, os.Args[1:], os.Stdout, os.Stderr, version, analysis.Local{}, func() time.Time { return time.Now().UTC() }))
}
