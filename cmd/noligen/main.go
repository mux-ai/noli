// Command noligen is Noli's extended ingestion and extraction CLI.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"noli/internal/studiocli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := studiocli.Run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
