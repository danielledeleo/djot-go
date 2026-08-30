// Command djot converts djot markup to HTML, the text AST, or JSON.
//
// Usage:
//
//	djot [options] [file...]
//
// It reads from the given files, or from stdin when none are given. See
// `djot --help` for options. All logic lives in the cli package; this is a thin
// wrapper that wires os streams to cli.Run.
package main

import (
	"os"

	"github.com/danielledeleo/djot-go/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
