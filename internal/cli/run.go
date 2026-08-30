// Package cli implements the djot command-line interface: it parses arguments,
// reads djot from stdin or files, and renders to HTML, the text AST, or JSON.
// The logic lives here, behind a testable [Run] function, so the cmd/djot
// entry point stays a thin wrapper around os streams.
//
// It is internal deliberately. [Run] is shaped for a process entry point --
// argv in, exit code out -- rather than for callers, and [version] reads the
// main module's build info, which only names djot-go when cmd/djot is the
// binary being built. Programs that want to render djot should use the root
// package's Parse and RenderHTML instead.
package cli

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"

	"github.com/danielledeleo/djot-go"
)

// Exit codes returned by Run.
const (
	exitOK      = 0 // success
	exitError   = 1 // runtime error (I/O, rendering)
	exitUsage   = 2 // bad flags or arguments
	errorPrefix = "djot: "
)

// Run executes the djot CLI. It reads djot source from the named files (or from
// in when no files are given), renders it in the requested format, and writes
// the result to out (or to the -o file). Diagnostics are written to errw. It
// returns a process exit code; it never calls os.Exit, so it is fully testable.
func Run(args []string, in io.Reader, out, errw io.Writer) int {
	var (
		format    string
		output    string
		sourcepos bool
		showVer   bool
	)

	fs := flag.NewFlagSet("djot", flag.ContinueOnError)
	fs.SetOutput(errw)
	fs.Usage = func() { usage(fs) }

	// Each option has a long and short form sharing one variable.
	fs.StringVar(&format, "to", "html", "output format: html, ast, or json")
	fs.StringVar(&format, "t", "html", "output format (shorthand for --to)")
	fs.StringVar(&output, "output", "", "write output to a file instead of stdout")
	fs.StringVar(&output, "o", "", "write output to a file (shorthand for --output)")
	fs.BoolVar(&sourcepos, "sourcepos", false, "include source positions (ast and json formats)")
	fs.BoolVar(&sourcepos, "p", false, "include source positions (shorthand for --sourcepos)")
	fs.BoolVar(&showVer, "version", false, "print version and exit")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return exitOK
		}
		return exitUsage
	}

	if showVer {
		fmt.Fprintf(out, "djot version %s\n", version())
		return exitOK
	}

	render, ok := renderers[format]
	if !ok {
		fmt.Fprintf(errw, "%sunknown format %q (want html, ast, or json)\n", errorPrefix, format)
		return exitUsage
	}

	src, err := readInput(fs.Args(), in)
	if err != nil {
		fmt.Fprintf(errw, "%s%v\n", errorPrefix, err)
		return exitError
	}

	w, closeOut, err := openOutput(output, out)
	if err != nil {
		fmt.Fprintf(errw, "%s%v\n", errorPrefix, err)
		return exitError
	}

	doc := djot.Parse(src)
	if err := render(w, doc, sourcepos); err != nil {
		closeOut()
		fmt.Fprintf(errw, "%s%v\n", errorPrefix, err)
		return exitError
	}
	if err := closeOut(); err != nil {
		fmt.Fprintf(errw, "%s%v\n", errorPrefix, err)
		return exitError
	}
	return exitOK
}

// renderers maps each --to format to a function that writes it.
var renderers = map[string]func(w io.Writer, doc *djot.Doc, sourcepos bool) error{
	"html": func(w io.Writer, doc *djot.Doc, _ bool) error {
		return djot.RenderHTMLTo(w, doc)
	},
	"ast": func(w io.Writer, doc *djot.Doc, sourcepos bool) error {
		return djot.RenderASTTo(w, doc, sourcepos)
	},
	"json": func(w io.Writer, doc *djot.Doc, sourcepos bool) error {
		return djot.RenderASTJSONTo(w, doc, sourcepos)
	},
}

// readInput returns the concatenated contents of the named files, or all of in
// when no files are given.
func readInput(files []string, in io.Reader) (string, error) {
	if len(files) == 0 {
		data, err := io.ReadAll(in)
		if err != nil {
			return "", fmt.Errorf("reading stdin: %w", err)
		}
		return string(data), nil
	}
	var b strings.Builder
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return "", err
		}
		b.Write(data)
	}
	return b.String(), nil
}

// openOutput returns the writer to render into and a function to finish with.
// An empty path renders to fallback, normally stdout, and the finish function
// only flushes; otherwise it creates the named file, and finishing flushes and
// closes it.
//
// The renderers write in small pieces — a tag, a run of text, a tag — so the
// destination is buffered. Handing them an unbuffered os.Stdout turns each of
// those into its own write syscall, which for a large document costs several
// times what the parse did.
func openOutput(path string, fallback io.Writer) (io.Writer, func() error, error) {
	if path == "" {
		buf := bufio.NewWriter(fallback)
		return buf, buf.Flush, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, err
	}
	buf := bufio.NewWriter(f)
	return buf, func() error {
		if err := buf.Flush(); err != nil {
			f.Close()
			return err
		}
		return f.Close()
	}, nil
}

// version reports the CLI version. When built with `go install …/cmd/djot@VER`
// it returns the module version (the release tag); for local builds it falls
// back to the VCS revision that Go stamps into the binary.
func version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	if v := info.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	var rev, dirty string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			if s.Value == "true" {
				dirty = "-dirty"
			}
		}
	}
	if rev != "" {
		if len(rev) > 12 {
			rev = rev[:12]
		}
		return "devel-" + rev + dirty
	}
	return "(devel)"
}

func usage(fs *flag.FlagSet) {
	out := fs.Output()
	fmt.Fprintf(out, "Usage: djot [options] [file...]\n\n")
	fmt.Fprintf(out, "Convert djot to HTML (default), the text AST, or JSON.\n")
	fmt.Fprintf(out, "Reads from the given files, or from stdin when none are given.\n\n")
	fmt.Fprintf(out, "Options:\n")
	fmt.Fprintf(out, "  -t, --to FORMAT     output format: html, ast, or json (default html)\n")
	fmt.Fprintf(out, "  -o, --output FILE   write to FILE instead of stdout\n")
	fmt.Fprintf(out, "  -p, --sourcepos     include source positions (ast and json)\n")
	fmt.Fprintf(out, "      --version       print version and exit\n")
	fmt.Fprintf(out, "  -h, --help          show this help and exit\n")
}
