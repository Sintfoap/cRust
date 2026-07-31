package lsp

import (
	"github.com/Sintfoap/cRust/internal/lexer"
	"github.com/Sintfoap/cRust/internal/parser"
	"github.com/Sintfoap/cRust/internal/token"
)

// computeDiagnostics lexes and parses text fresh — no incremental
// reuse, the same "start from scratch every call" approach `crust
// tokens`/`crust parse` already use — and turns every ILLEGAL token
// and parser.ParseError into an LSP Diagnostic, positioned with
// encoding-correct character offsets (see position.go).
func computeDiagnostics(text string, encoding string) []Diagnostic {
	lines := splitLines(text)
	diags := []Diagnostic{}

	type pos struct{ line, col int }
	illegal := map[pos]bool{}

	l := lexer.New(text)
	for {
		tok := l.NextToken()
		if tok.Type == token.ILLEGAL {
			illegal[pos{tok.Line, tok.Col}] = true
			diags = append(diags, positionalDiagnostic(lines, tok.Line, tok.Col, tok.Literal, encoding))
		}
		if tok.Type == token.EOF {
			break
		}
	}

	// A malformed token trips the lexer first and the parser second —
	// the parser has nothing to do with an ILLEGAL token but fail on
	// it too, at the exact same position. Reporting both would just be
	// the same typo twice with a more confusing second message ("no
	// prefix parse function for ILLEGAL found"), so parse errors that
	// land exactly on an already-reported ILLEGAL position are
	// dropped.
	p := parser.New(lexer.New(text))
	p.ParseProgram()
	for _, e := range p.ParseErrors() {
		if illegal[pos{e.Line, e.Col}] {
			continue
		}
		diags = append(diags, positionalDiagnostic(lines, e.Line, e.Col, e.Message, encoding))
	}

	return diags
}

// positionalDiagnostic builds a one-character-wide Diagnostic at
// (line, col) — precise enough to place the squiggle at the right spot
// without needing to reconstruct each error's exact source span
// (which, for a lexer error especially, may not even correspond to the
// literal text at that position — e.g. an unterminated-string error's
// "literal" is the error message itself, not source text).
func positionalDiagnostic(lines []string, line, col int, message string, encoding string) Diagnostic {
	start := toPosition(lines, line, col, encoding)
	end := start
	end.Character++
	return Diagnostic{
		Range:    Range{Start: start, End: end},
		Severity: SeverityError,
		Source:   "crust",
		Message:  message,
	}
}
