// Command crust is the CLI entrypoint for the cRust language: a
// pizza-themed interpreted language built for Advent of Code 2026.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

// version is overridden at build time via:
//
//	go build -ldflags "-X main.version=1.2.3"
var version = "dev"

const usage = `crust - an interpreted language where every keyword is pizza jargon

Usage:
  crust run <file>     run a .crust source file
  crust repl           start an interactive REPL
  crust --version      print the version
  crust --help         show this help
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("crust", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, usage) }

	var showVersion, showHelp bool
	fs.BoolVar(&showVersion, "version", false, "print version and exit")
	fs.BoolVar(&showHelp, "help", false, "show help and exit")
	fs.BoolVar(&showHelp, "h", false, "show help and exit")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	switch {
	case showVersion:
		fmt.Fprintln(stdout, "crust", version)
		return 0
	case showHelp:
		fmt.Fprint(stdout, usage)
		return 0
	}

	rest := fs.Args()
	if len(rest) == 0 {
		fmt.Fprint(stderr, usage)
		return 1
	}

	switch rest[0] {
	case "run", "repl":
		fmt.Fprintf(stderr, "crust %s: not in the oven yet — the interpreter lands in Phase 4 (see TODO.md)\n", rest[0])
		return 1
	default:
		fmt.Fprintf(stderr, "crust: unknown command %q\n\n", rest[0])
		fmt.Fprint(stderr, usage)
		return 1
	}
}
