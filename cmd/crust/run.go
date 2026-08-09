package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Sintfoap/cRust/internal/runner"
)

// parseRunArgs pulls a file path and an optional --store=<name> out of
// args, in either order (`crust run day01.crust --store=part1` and
// `crust run --store=part1 day01.crust` both work) — Go's flag package
// can't do this on its own, since it stops parsing flags at the first
// positional argument it sees.
func parseRunArgs(args []string) (path, store string, err error) {
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "--store="):
			store = strings.TrimPrefix(a, "--store=")
		case a == "--store":
			return "", "", fmt.Errorf("--store requires a value, e.g. --store=part1")
		case strings.HasPrefix(a, "-"):
			return "", "", fmt.Errorf("unknown flag: %s", a)
		default:
			if path != "" {
				return "", "", fmt.Errorf("unexpected extra argument: %s", a)
			}
			path = a
		}
	}
	return path, store, nil
}

// runFile reads path and hands it to internal/runner.Run, which does
// the actual lex/parse/Eval/entry-point work shared with `develop`'s
// Run tab and the web playground. storeFlag is the --store value (""
// selects the bare `store`).
func runFile(path, storeFlag string, stdin io.Reader, stdout, stderr io.Writer) int {
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "crust run: %s\n", err)
		return 1
	}
	return runner.Run(string(src), storeFlag, path, "crust run: ", stdin, stdout, stderr)
}
