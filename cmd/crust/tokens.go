package main

import (
	"fmt"
	"io"
	"os"

	"github.com/Sintfoap/cRust/internal/lexer"
	"github.com/Sintfoap/cRust/internal/token"
)

// runTokens reads path, lexes it, and prints every token to stdout —
// a debug aid for the lexer (and later phases) since `crust run`
// doesn't exist yet to exercise it any other way. Returns a non-zero
// exit code if the file can't be read or the lexer produces any
// ILLEGAL token, but still prints every token it got first.
func runTokens(path string, stdout, stderr io.Writer) int {
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "crust tokens: %s\n", err)
		return 1
	}

	l := lexer.New(string(src))
	hadIllegal := false
	for {
		tok := l.NextToken()
		printToken(stdout, tok)
		if tok.Type == token.ILLEGAL {
			hadIllegal = true
		}
		if tok.Type == token.EOF {
			break
		}
	}

	if hadIllegal {
		fmt.Fprintln(stderr, "crust tokens: input contains at least one ILLEGAL token (see above)")
		return 1
	}
	return 0
}

func printToken(w io.Writer, tok token.Token) {
	lit := tok.Literal
	switch tok.Type {
	case token.NEWLINE:
		lit = `\n`
	case token.STRING, token.ILLEGAL:
		lit = fmt.Sprintf("%q", tok.Literal)
	}
	fmt.Fprintf(w, "%4d:%-4d %-10s %s\n", tok.Line, tok.Col, tok.Type, lit)
}
