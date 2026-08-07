package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Sintfoap/cRust/internal/ast"
	"github.com/Sintfoap/cRust/internal/lexer"
	"github.com/Sintfoap/cRust/internal/parser"
)

// parseForTest parses src and fails the test on any parse error --
// only lastStatementIsExpression's own table test uses this, and every
// case in it is deliberately valid cRust, so a parse error there means
// the test fixture itself is wrong, not the code under test.
func parseForTest(t *testing.T, src string) *ast.Program {
	t.Helper()
	p := parser.New(lexer.New(src))
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parseForTest(%q): unexpected parse errors: %v", src, errs)
	}
	return program
}

func TestREPLEchoesExpressionResult(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runREPL(strings.NewReader("1 + 2\n"), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "3\n") {
		t.Errorf("stdout = %q, want it to echo 3", stdout.String())
	}
}

// TestREPLSuppressesAssignmentEcho confirms `x = 1` doesn't print
// anything of its own -- an assignment's whole point is the side
// effect (binding x), the same reason Python's REPL doesn't echo one
// either.
func TestREPLSuppressesAssignmentEcho(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runREPL(strings.NewReader("x = 1\n"), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "1\n") {
		t.Errorf("stdout = %q, want no echoed value for a plain assignment", stdout.String())
	}
}

// TestREPLPersistsStateAcrossLines is the entire point of a REPL over
// running one-off snippets: a variable bound on one line must still be
// visible on the next.
func TestREPLPersistsStateAcrossLines(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runREPL(strings.NewReader("x = 40\nx + 2\n"), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "42\n") {
		t.Errorf("stdout = %q, want it to echo 42 using the x bound on the previous line", stdout.String())
	}
}

// TestREPLRecipeDefinitionPersists confirms a recipe defined on one
// line can be called on a later one, in the same session.
func TestREPLRecipeDefinitionPersists(t *testing.T) {
	src := "recipe double(n) { serve n * 2 }\ndouble(21)\n"
	var stdout, stderr bytes.Buffer
	code := runREPL(strings.NewReader(src), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "42\n") {
		t.Errorf("stdout = %q, want it to echo 42 from calling the recipe defined on the previous line", stdout.String())
	}
}

// TestREPLMultiLineContinuation confirms an unclosed `{` prompts for
// more input (the continuation prompt) instead of reporting a syntax
// error immediately, and that the construct evaluates correctly once
// closed on a later line.
func TestREPLMultiLineContinuation(t *testing.T) {
	src := "recipe triple(n) {\nserve n * 3\n}\ntriple(4)\n"
	var stdout, stderr bytes.Buffer
	code := runREPL(strings.NewReader(src), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), replContinuePrompt) {
		t.Errorf("stdout = %q, want the continuation prompt while the recipe body is still open", stdout.String())
	}
	if !strings.Contains(stdout.String(), "12\n") {
		t.Errorf("stdout = %q, want it to echo 12 once the recipe is complete and called", stdout.String())
	}
	if stderr.String() != "" {
		t.Errorf("stderr = %q, want no error for input that just needed more lines", stderr.String())
	}
}

// TestREPLGenuineSyntaxErrorReportsImmediately confirms a real syntax
// error (not just "needs more input") surfaces right away rather than
// leaving the REPL stuck waiting for a continuation that could never
// fix it.
func TestREPLGenuineSyntaxErrorReportsImmediately(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runREPL(strings.NewReader("1 +++ 2\n"), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if stderr.String() == "" {
		t.Error("stderr is empty, want a reported syntax error")
	}
	if strings.Count(stdout.String(), replContinuePrompt) > 0 {
		t.Errorf("stdout = %q, want no continuation prompt for a genuine syntax error", stdout.String())
	}
}

// TestREPLRuntimeErrorDoesNotEndSession confirms a runtime error (e.g.
// dividing by zero) is reported but the session keeps going -- state
// bound before the error is still usable afterward.
func TestREPLRuntimeErrorDoesNotEndSession(t *testing.T) {
	src := "x = 5\n1 / 0\nx + 1\n"
	var stdout, stderr bytes.Buffer
	code := runREPL(strings.NewReader(src), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "division by zero") && !strings.Contains(strings.ToLower(stderr.String()), "zero") {
		t.Errorf("stderr = %q, want it to mention the division-by-zero error", stderr.String())
	}
	if !strings.Contains(stdout.String(), "6\n") {
		t.Errorf("stdout = %q, want the session to continue and echo 6 for x + 1", stdout.String())
	}
}

// TestREPLDeliverWritesThroughInterpreterOutput confirms deliver()'s
// own output reaches stdout, and that its NULL return value isn't also
// echoed on top of it (deliver is a bare expression statement, but its
// result is nobox).
func TestREPLDeliverWritesThroughInterpreterOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runREPL(strings.NewReader(`deliver("hi")`+"\n"), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "hi") {
		t.Errorf("stdout = %q, want deliver's own output", stdout.String())
	}
	if strings.Contains(stdout.String(), "nobox") {
		t.Errorf("stdout = %q, want deliver()'s NULL result not echoed", stdout.String())
	}
}

func TestREPLBlankLineIsHarmless(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runREPL(strings.NewReader("\n\n1\n"), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if stderr.String() != "" {
		t.Errorf("stderr = %q, want blank lines to be harmless", stderr.String())
	}
	if !strings.Contains(stdout.String(), "1\n") {
		t.Errorf("stdout = %q, want the final bare 1 echoed", stdout.String())
	}
}

func TestREPLEmptyStdinExitsCleanly(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runREPL(strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if stderr.String() != "" {
		t.Errorf("stderr = %q, want empty for a clean EOF", stderr.String())
	}
}

func TestLastStatementIsExpressionHelper(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want bool
	}{
		{"bare expression", "1 + 2", true},
		{"call expression", "deliver(1)", true},
		{"recipe literal", "recipe(x) { serve x }", true},
		{"assignment", "x = 1", false},
		// A named recipe declaration parses as an ExpressionStatement
		// wrapping a FunctionLiteral, same as an anonymous one -- so it
		// auto-echoes too (recipe(n) { ... }), which doubles as
		// confirmation the definition took.
		{"recipe declaration", "recipe foo() { serve 1 }", true},
		{"loop", "knead i in [1] { deliver(i) }", false},
		{"empty program", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program := parseForTest(t, tt.src)
			if got := lastStatementIsExpression(program); got != tt.want {
				t.Errorf("lastStatementIsExpression(%q) = %v, want %v", tt.src, got, tt.want)
			}
		})
	}
}

func TestNeedsMoreInputHelper(t *testing.T) {
	tests := []struct {
		name string
		errs []string
		want bool
	}{
		{"no errors", nil, false},
		{"unclosed brace", []string{`line 2:1: expected '}' to close block, got EOF instead`}, true},
		{"unclosed paren via peek", []string{`line 1:8: expected next token to be RPAREN, got EOF ("") instead`}, true},
		{"trailing operator", []string{`line 1:4: no prefix parse function for EOF found`}, true},
		{"genuine error last", []string{`line 1:1: could not parse "3.5.5" as a float`}, false},
		{"error before an EOF continuation is still reported (not this function's job to suppress)", []string{
			`line 1:1: could not parse "3.5.5" as a float`,
			`line 2:1: expected '}' to close block, got EOF instead`,
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := needsMoreInput(tt.errs); got != tt.want {
				t.Errorf("needsMoreInput(%v) = %v, want %v", tt.errs, got, tt.want)
			}
		})
	}
}
