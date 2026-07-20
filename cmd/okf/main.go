// Command okf is the deprecated compatibility alias for the noli CLI.
package main

import (
	"os"

	"noli/internal/cli"
)

func main() {
	app := cli.New(os.Stdout, os.Stderr)
	os.Exit(app.Run(os.Args[1:]))
}
