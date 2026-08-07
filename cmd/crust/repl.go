// `crust repl` — an interactive read-eval-print loop: one persistent
// Interpreter and Environment for the whole session, so a variable or
// recipe defined on one line is still there on the next. No `let`/
// `const` ceremony here either — the same "just assign" model SPEC.md
// describes for a whole file, just typed one chunk at a time.
package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/Sintfoap/cRust/internal/ast"
	"github.com/Sintfoap/cRust/internal/interpreter"
	"github.com/Sintfoap/cRust/internal/lexer"
	"github.com/Sintfoap/cRust/internal/object"
	"github.com/Sintfoap/cRust/internal/parser"
)

const (
	replPrompt         = "crust> "
	replContinuePrompt = "  ...> "
)

// runREPL drives the loop until stdin closes (Ctrl+D) or a read error
// occurs. unbox() (with no argument) still reads from the same stdin
// the REPL's own line-by-line scanner is consuming — a real but
// niche wrinkle: on an interactive terminal each Scan() only pulls
// however much the tty currently has queued (typically just the line
// just entered), so the two readers don't fight in practice, but with
// piped/redirected input a call to unbox() mid-session can observe
// fewer bytes than expected, since bufio.Scanner may already have
// buffered ahead of whatever line is currently being evaluated. Not
// solved here — running a whole file (`crust run`) is the intended way
// to process real stdin input; the REPL is for trying things out.
func runREPL(stdin io.Reader, stdout, stderr io.Writer) int {
	fmt.Fprintln(stdout, "crust repl — Ctrl+D to quit")

	interp := interpreter.New(stdout, stdin)
	env := object.NewEnvironment()
	scanner := bufio.NewScanner(stdin)

	var buf strings.Builder
	prompt := replPrompt
	for {
		fmt.Fprint(stdout, prompt)
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				fmt.Fprintf(stderr, "crust repl: %v\n", err)
				return 1
			}
			fmt.Fprintln(stdout)
			return 0
		}

		if buf.Len() > 0 {
			buf.WriteByte('\n')
		}
		buf.WriteString(scanner.Text())

		l := lexer.New(buf.String())
		p := parser.New(l)
		program := p.ParseProgram()

		if errs := p.Errors(); len(errs) > 0 {
			if needsMoreInput(errs) {
				prompt = replContinuePrompt
				continue
			}
			for _, e := range errs {
				fmt.Fprintf(stderr, "  %s\n", e)
			}
			buf.Reset()
			prompt = replPrompt
			continue
		}

		buf.Reset()
		prompt = replPrompt

		result := interp.Eval(program, env)
		if errObj, ok := result.(*object.Error); ok {
			fmt.Fprintf(stderr, "%d:%d: %s\n", errObj.Line, errObj.Col, errObj.Message)
			continue
		}
		if result != object.NULL && lastStatementIsExpression(program) {
			fmt.Fprintln(stdout, result.Inspect())
		}
	}
}

// lastStatementIsExpression reports whether program's last top-level
// statement is a bare expression (`1 + 2`, `foo()`, a recipe literal,
// ...) rather than an assignment, loop, conditional, or other
// statement whose entire point is a side effect — the same rule most
// REPLs use to decide whether evaluating a chunk is worth echoing a
// value for. A chunk with no statements at all (a blank line) isn't.
func lastStatementIsExpression(program *ast.Program) bool {
	if len(program.Statements) == 0 {
		return false
	}
	_, ok := program.Statements[len(program.Statements)-1].(*ast.ExpressionStatement)
	return ok
}

// needsMoreInput reports whether errs looks like parsing ran out of
// input mid-construct (an unclosed `{`, `(`, `[`, or a trailing binary
// operator with nothing after it) rather than hitting a genuine syntax
// error — the signal the REPL uses to switch to the continuation
// prompt and keep accumulating lines instead of reporting failure
// immediately. internal/parser's own error messages substitute the
// offending token's Type into every message that could fire this way
// (peekError's "got %s (%q) instead", noPrefixParseFnError's "for %s
// found", the block/statement-terminator errors below them) — and
// token.EOF's Type literally is the string "EOF", which nothing a
// human would actually type as part of an error-triggering token's own
// text ever collides with. Checking only the last collected error
// matters: synchronize() stops advancing the moment it reaches EOF, so
// a real error earlier in the buffered chunk (which can't be fixed by
// more input) still gets its own, earlier entry and this won't
// mistake the pair for "just needs more".
func needsMoreInput(errs []string) bool {
	if len(errs) == 0 {
		return false
	}
	return strings.Contains(errs[len(errs)-1], "EOF")
}
