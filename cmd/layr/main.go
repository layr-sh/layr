// Package main provides the executable entrypoint for the Layr binary.
package main

import (
	"os"

	"layr.sh/cli"
)

var (
	cliRunFunc = cli.Run
	exitFunc   = os.Exit
)

func main() {
	if err := cliRunFunc(os.Args[1:]); err != nil {
		exitFunc(1)
	}
}
