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

const usageBody = `Usage:
  crust <file.crust> [--store=<name>]         interpret a .crust source file
  crust run <file.crust> [--store=<name>]     interpret, explicitly
  crust repl                                  start an interactive REPL
  crust tokens <file.crust>                   print the lexer's token stream and exit
  crust parse <file.crust>                    print the parsed AST and exit
  crust develop <file.crust> [--store=<name>] step through a run: time/memory per function
  crust fmt <file.crust> [-w]                 print (or write, with -w) canonically formatted source
  crust bake documentation [-p <port>]        serve the docs site, open a browser
  crust bake playground [-p <port>]           serve an in-browser cRust sandbox (WASM)
  crust lsp                                   start a language server on stdin/stdout
  crust --version                             print the version
  crust --help | -h                           show this help

Run flags:
  --store <name>  which store/store_<name> recipe to run as the entry
                  point (default: the bare "store", if the file has one)

Pizza flags (banner customization):
  --toppings <list>  comma-separated: pepperoni, basil, all, plain (default: pepperoni,basil)
  --no-banner        skip the banner
  --no-color         disable ANSI colors (also respects $NO_COLOR)

Examples:
  crust day01.crust                 run a solution file
  crust day01.crust --store=part2   run its store_part2 entry point
  crust tokens day01.crust          debug: see how it lexes
  crust parse day01.crust           debug: see how it parses
  crust develop day01.crust         step through a run, see time/memory per function
  crust fmt day01.crust             preview canonically formatted source
  crust fmt -w day01.crust          reformat the file in place
  crust bake documentation          open the docs site in a browser
  crust bake playground             open the in-browser sandbox (nothing sent anywhere)
  crust lsp                         debug: run the language server by hand
  crust --toppings=all --help       preview the fully-loaded pizza
  crust --no-color --help           plain-text help, no ANSI
`

func main() {
	colorDefault := os.Getenv("NO_COLOR") == "" && isTerminal(os.Stdout)
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, colorDefault))
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer, colorDefault bool) int {
	fs := flag.NewFlagSet("crust", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var showVersion, showHelp, noBanner, noColor bool
	var toppingsFlag string
	fs.BoolVar(&showVersion, "version", false, "print version and exit")
	fs.BoolVar(&showHelp, "help", false, "show help and exit")
	fs.BoolVar(&showHelp, "h", false, "show help and exit")
	fs.StringVar(&toppingsFlag, "toppings", "pepperoni,basil", "toppings shown on the banner pizza")
	fs.BoolVar(&noBanner, "no-banner", false, "skip the banner")
	fs.BoolVar(&noColor, "no-color", false, "disable ANSI colors")
	fs.Usage = func() { fmt.Fprint(stderr, usageBody) }

	if err := fs.Parse(args); err != nil {
		return 2
	}

	toppings, err := parseToppings(toppingsFlag)
	if err != nil {
		fmt.Fprintf(stderr, "crust: %s\n", err)
		return 2
	}
	color := colorDefault && !noColor

	printHelp := func(w io.Writer) {
		if !noBanner {
			fmt.Fprint(w, banner(toppings, color))
		}
		fmt.Fprint(w, usageBody)
	}

	switch {
	case showVersion:
		fmt.Fprintln(stdout, "crust", version)
		return 0
	case showHelp:
		printHelp(stdout)
		return 0
	}

	rest := fs.Args()
	if len(rest) == 0 {
		printHelp(stderr)
		return 1
	}

	switch rest[0] {
	case "repl":
		if len(rest) > 1 {
			fmt.Fprintf(stderr, "crust repl: takes no arguments (got %q)\n", rest[1])
			return 2
		}
		return runREPL(stdin, stdout, stderr)
	case "run":
		path, storeFlag, err := parseRunArgs(rest[1:])
		if err != nil {
			fmt.Fprintf(stderr, "crust run: %s\n", err)
			return 2
		}
		if path == "" {
			fmt.Fprintln(stderr, "crust run: missing <file.crust>")
			return 1
		}
		return runFile(path, storeFlag, stdin, stdout, stderr)
	case "tokens":
		if len(rest) < 2 {
			fmt.Fprintln(stderr, "crust tokens: missing <file.crust>")
			return 1
		}
		return runTokens(rest[1], stdout, stderr)
	case "parse":
		if len(rest) < 2 {
			fmt.Fprintln(stderr, "crust parse: missing <file.crust>")
			return 1
		}
		return runParse(rest[1], stdout, stderr)
	case "develop":
		path, opts, err := parseDebugArgs(rest[1:])
		if err != nil {
			fmt.Fprintf(stderr, "crust develop: %s\n", err)
			return 2
		}
		return runDebug(path, opts, stdin, stdout, stderr)
	case "fmt":
		path, write, err := parseFmtArgs(rest[1:])
		if err != nil {
			fmt.Fprintf(stderr, "crust fmt: %s\n", err)
			return 2
		}
		return runFmt(path, write, stdout, stderr)
	case "bake":
		if len(rest) < 2 {
			fmt.Fprintln(stderr, `crust bake: expected "documentation" or "playground" (crust bake documentation|playground [-p <port>])`)
			return 2
		}
		switch rest[1] {
		case "documentation":
			port, err := parseBakeArgs(rest[2:])
			if err != nil {
				fmt.Fprintf(stderr, "crust bake documentation: %s\n", err)
				return 2
			}
			return runBakeDocumentation(port, stdout, stderr)
		case "playground":
			port, err := parsePlaygroundArgs(rest[2:])
			if err != nil {
				fmt.Fprintf(stderr, "crust bake playground: %s\n", err)
				return 2
			}
			return runBakePlayground(port, stdout, stderr)
		default:
			fmt.Fprintln(stderr, `crust bake: expected "documentation" or "playground" (crust bake documentation|playground [-p <port>])`)
			return 2
		}
	case "lsp":
		return runLSP(stdin, stdout, stderr)
	default:
		// Bare-file shorthand: `crust foo.crust [--store=<name>]`
		// behaves like `crust run foo.crust [--store=<name>]`.
		path, storeFlag, err := parseRunArgs(rest)
		if err != nil {
			fmt.Fprintf(stderr, "crust: %s\n", err)
			return 2
		}
		return runFile(path, storeFlag, stdin, stdout, stderr)
	}
}
