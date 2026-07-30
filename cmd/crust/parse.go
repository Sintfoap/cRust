package main

import (
	"fmt"
	"io"
	"os"

	"github.com/Sintfoap/cRust/internal/lexer"
	"github.com/Sintfoap/cRust/internal/parser"
)

// runParse reads path, parses it, and prints the resulting AST — one
// numbered line per top-level statement, using each node's String()
// (which fully parenthesizes expressions, so operator precedence and
// associativity are visible directly in the output rather than needing
// a separate tree-printer). A debug aid for the parser the same way
// runTokens is for the lexer, since `crust run` doesn't exist yet to
// exercise it any other way. Parse errors are printed to stderr and
// cause a non-zero exit, but whatever statements did parse are still
// printed to stdout first — mirrors runTokens' "print what you got,
// then flag the problem" behavior for ILLEGAL tokens.
func runParse(path string, stdout, stderr io.Writer) int {
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "crust parse: %s\n", err)
		return 1
	}

	l := lexer.New(string(src))
	p := parser.New(l)
	program := p.ParseProgram()

	for i, stmt := range program.Statements {
		fmt.Fprintf(stdout, "#%d: %s\n", i+1, stmt.String())
	}

	if errs := p.Errors(); len(errs) > 0 {
		fmt.Fprintf(stderr, "crust parse: %d parse error(s):\n", len(errs))
		for _, e := range errs {
			fmt.Fprintf(stderr, "  %s\n", e)
		}
		return 1
	}
	return 0
}
