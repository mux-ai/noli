// Command noli is the primary Noli CLI for Open Knowledge Format bundles. It
// emits deterministic JSON on stdout and reserves stderr for diagnostics.
// This is the only primary-command entry point that calls os.Exit.
package main

import (
	"os"

	"noli/internal/cli"
)

func main() {
	app := cli.New(os.Stdout, os.Stderr)
	os.Exit(app.Run(os.Args[1:]))
}
