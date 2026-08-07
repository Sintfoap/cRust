// `crust fmt <file.crust>` — internal/format's canonical printer, run
// from the command line. Prints the formatted source to stdout by
// default (gofmt's own default, not rustfmt's overwrite-by-default) —
// `-w` writes the result back to the file in place instead.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Sintfoap/cRust/internal/format"
	"github.com/Sintfoap/cRust/internal/lexer"
	"github.com/Sintfoap/cRust/internal/parser"
)

// parseFmtArgs pulls a file path and an optional -w/--write flag out
// of args, in either order — the same "positional argument in any
// position" flexibility parseRunArgs already gives `crust run`.
func parseFmtArgs(args []string) (path string, write bool, err error) {
	for _, a := range args {
		switch {
		case a == "-w" || a == "--write":
			write = true
		case strings.HasPrefix(a, "-"):
			return "", false, fmt.Errorf("unknown flag: %s", a)
		default:
			if path != "" {
				return "", false, fmt.Errorf("unexpected extra argument: %s", a)
			}
			path = a
		}
	}
	if path == "" {
		return "", false, fmt.Errorf("missing <file.crust>")
	}
	return path, write, nil
}

// runFmt reads path, formats it, and either prints the result to
// stdout or writes it back to path, per write. A parse error is
// reported the same way `crust run`'s reportRuntimeError-adjacent path
// does (run.go) — every error, one per line, indented — since
// formatting an unparseable file is exactly as impossible as running
// one.
func runFmt(path string, write bool, stdout, stderr io.Writer) int {
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "crust fmt: %s\n", err)
		return 1
	}

	l := lexer.New(string(src))
	p := parser.New(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		fmt.Fprintf(stderr, "crust fmt: %d parse error(s):\n", len(errs))
		for _, e := range errs {
			fmt.Fprintf(stderr, "  %s\n", e)
		}
		return 1
	}

	formatted := format.Format(program, string(src))

	if !write {
		fmt.Fprint(stdout, formatted)
		return 0
	}
	if formatted == string(src) {
		return 0 // already canonical; skip the write so mtime/git status stay untouched
	}
	if err := os.WriteFile(path, []byte(formatted), 0o644); err != nil {
		fmt.Fprintf(stderr, "crust fmt: %s\n", err)
		return 1
	}
	return 0
}
