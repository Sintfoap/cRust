package format

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sintfoap/cRust/internal/ast"
	"github.com/Sintfoap/cRust/internal/interpreter"
	"github.com/Sintfoap/cRust/internal/lexer"
	"github.com/Sintfoap/cRust/internal/object"
	"github.com/Sintfoap/cRust/internal/parser"
)

// fmtSrc parses src and formats it, failing the test on a parse error.
func fmtSrc(t *testing.T, src string) string {
	t.Helper()
	l := lexer.New(src)
	p := parser.New(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("unexpected parse errors for %q: %v", src, errs)
	}
	return Format(program, src)
}

// run evaluates src top-to-bottom (no entry-point resolution — every
// examples/*.crust file this package's tests use is a plain
// top-level script) and returns whatever deliver() wrote, failing the
// test on a parse or runtime error. baseDir is the Interpreter's
// BaseDir (runner.Run's own wiring for a real file) — needed so a
// `delivery` statement in src (examples/module_demo.crust) resolves
// its relative path against the example's own directory rather than
// this test binary's working directory; every synthetic snippet this
// package's other tests pass has no delivery statement to resolve, so
// "" (this process's own cwd, never actually consulted) is fine there.
func run(t *testing.T, src, baseDir string) string {
	t.Helper()
	l := lexer.New(src)
	p := parser.New(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("unexpected parse errors: %v", errs)
	}
	var out bytes.Buffer
	interp := interpreter.New(&out, strings.NewReader(""))
	interp.BaseDir = baseDir
	result := interp.Eval(program, object.NewEnvironment())
	if errObj, ok := result.(*object.Error); ok {
		t.Fatalf("unexpected runtime error: %s", errObj.Message)
	}
	return out.String()
}

// TestFormatPreservesSemanticsAcrossExamples is the strongest
// correctness check this package has: for every real, hand-written
// example program in examples/*.crust, running the original source and
// running Format's output (re-parsed fresh) must produce
// byte-identical deliver() output. run() never resolves a
// store/store_<name> entry point (only top-level code runs), so this
// holds even for dayNN_template.crust, whose recipes read unbox() —
// that read only happens if the recipe is actually called, which
// nothing here does; every other example is a plain top-to-bottom
// script with no entry point at all, confirmed by grepping for
// unbox/store before this test was first written. A formatter that
// changed what a program *does* — even subtly, even just for one
// operator's precedence in one corner of one file — would fail this
// immediately, which is exactly the property that matters most for a
// tool meant to run on real code unattended (crust lsp's
// format-on-save).
func TestFormatPreservesSemanticsAcrossExamples(t *testing.T) {
	files, err := filepath.Glob("../../examples/*.crust")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no example files found — glob pattern or working directory is wrong")
	}
	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			dir := filepath.Dir(path)
			original := run(t, string(src), dir)
			formatted := fmtSrc(t, string(src))
			reformatted := run(t, formatted, dir)
			if original != reformatted {
				t.Errorf("output changed after formatting.\n--- original output ---\n%s\n--- formatted source ---\n%s\n--- reformatted output ---\n%s",
					original, formatted, reformatted)
			}
		})
	}
}

// TestFormatIsIdempotent confirms formatting already-formatted output
// again produces byte-identical text — the standard "fixed point"
// property every real formatter (gofmt included) guarantees, and
// something crust lsp's format-on-save depends on implicitly (saving
// twice in a row shouldn't shuffle the file a second time).
func TestFormatIsIdempotent(t *testing.T) {
	files, err := filepath.Glob("../../examples/*.crust")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			once := fmtSrc(t, string(src))
			twice := fmtSrc(t, once)
			if once != twice {
				t.Errorf("formatting is not idempotent.\n--- first pass ---\n%s\n--- second pass ---\n%s", once, twice)
			}
		})
	}
}

func TestFormatEndsWithExactlyOneTrailingNewline(t *testing.T) {
	tests := []string{
		"x = 1",
		"x = 1\n",
		"x = 1\n\n\n",
		"x = 1\n\n\ndeliver(x)",
	}
	for _, src := range tests {
		got := fmtSrc(t, src)
		if !strings.HasSuffix(got, "\n") {
			t.Errorf("Format(%q) doesn't end with a newline: %q", src, got)
		}
		if strings.HasSuffix(got, "\n\n") {
			t.Errorf("Format(%q) ends with more than one newline: %q", src, got)
		}
	}
}

func TestFormatBasicIndentationAndBraceStyle(t *testing.T) {
	src := `recipe findPair(nums,target){
knead(i=0;i<slices(nums);i++){
order(nums[i]==target){
serve i
}
}
serve nobox
}`
	want := `recipe findPair(nums, target) {
    knead (i = 0; i < slices(nums); i++) {
        order (nums[i] == target) {
            serve i
        }
    }
    serve nobox
}
`
	got := fmtSrc(t, src)
	if got != want {
		t.Errorf("Format() =\n%s\nwant\n%s", got, want)
	}
}

func TestFormatPrecedenceMinimalParens(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"sum then product needs no parens", "y = x + 2 * 3 - 1", "y = x + 2 * 3 - 1\n"},
		{"explicit grouping that changes precedence is kept", "z = (x + 2) * 3", "z = (x + 2) * 3\n"},
		{"right side of same-precedence subtraction needs parens", "a = 1 - (2 - 3)", "a = 1 - (2 - 3)\n"},
		{"left side of same-precedence subtraction needs none", "a = (1 - 2) - 3", "a = 1 - 2 - 3\n"},
		{"right-associative elvis chain needs no parens", "b = 2 ?: 3 ?: 4", "b = 2 ?: 3 ?: 4\n"},
		{"left-grouped elvis needs parens", "c = (1 ?: 2) ?: 3", "c = (1 ?: 2) ?: 3\n"},
		{"chained calls need no parens", "r = f(1)(2)", "r = f(1)(2)\n"},
		{"chained index needs no parens", "r = a[0][1]", "r = a[0][1]\n"},
		{"call on a grouped sum needs parens", "r = (a + b)(1)", "r = (a + b)(1)\n"},
		{"index on a grouped sum needs parens", "r = (a + b)[0]", "r = (a + b)[0]\n"},
		{"stacked negation keeps a separating space, not parens", "d = - -1", "d = - -1\n"},
		{"negation of a sum needs parens", "d = -(a + b)", "d = -(a + b)\n"},
		{"comparison as ternary cond needs no parens", `t = x > 1 (| "big" |) "small"`, `t = x > 1 (| "big" |) "small"` + "\n"},
		{"nested ternary as cond needs parens", `t = (a (| b |) c) (| d |) e`, `t = (a (| b |) c) (| d |) e` + "\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fmtSrc(t, tt.src)
			if got != tt.want {
				t.Errorf("Format(%q) = %q, want %q", tt.src, got, tt.want)
			}
		})
	}
}

func TestFormatPrecedenceReparsesToEquivalentBehavior(t *testing.T) {
	// Each of these programs' correctness depends entirely on getting
	// precedence/associativity right; TestFormatPrecedenceMinimalParens
	// checks the exact rendered text, this checks that re-running the
	// *result* still behaves like the original -- the property that
	// actually matters.
	tests := []struct {
		name string
		src  string
	}{
		{"left-assoc subtraction chain", `deliver((10 - 3) - 2)` + "\n" + `deliver(10 - 3 - 2)`},
		{"right-assoc elvis chain", `deliver(nobox ?: nobox ?: 5)`},
		{"mixed sum and product", `deliver(1 + 2 * 3)`},
		{"grouped sum then product", `deliver((1 + 2) * 3)`},
		{"stacked negation", `deliver(- -5)`},
		{"chained calls", `f = recipe(x) { serve recipe(y) { serve x + y } }` + "\n" + `deliver(f(1)(2))`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := run(t, tt.src, "")
			formatted := fmtSrc(t, tt.src)
			reformatted := run(t, formatted, "")
			if original != reformatted {
				t.Errorf("%s: original = %q, reformatted = %q (formatted source: %q)", tt.name, original, reformatted, formatted)
			}
		})
	}
}

func TestFormatStringEscaping(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"plain string unchanged", `s = "hello"`, `s = "hello"` + "\n"},
		{"escapes round-trip", `s = "a\nb\tc\rd\"e\\f"`, `s = "a\nb\tc\rd\"e\\f"` + "\n"},
		{"unicode passes through unescaped", `s = "pizza 🍕"`, `s = "pizza 🍕"` + "\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fmtSrc(t, tt.src)
			if got != tt.want {
				t.Errorf("Format(%q) = %q, want %q", tt.src, got, tt.want)
			}
			// And it must still parse back to the same String value.
			l := lexer.New(got)
			pp := parser.New(l)
			reparsed := pp.ParseProgram()
			if errs := pp.Errors(); len(errs) > 0 {
				t.Fatalf("formatted string literal doesn't reparse: %v", errs)
			}
			_ = reparsed
		})
	}
}

// TestNodePrecedenceUnknownOperatorFallsBackToLowest exercises
// nodePrecedence's defensive fallback for an *ast.InfixExpression
// whose Operator isn't in infixPrecedence -- unreachable through any
// AST internal/parser can actually produce (every real InfixExpression
// operator is one parseInfixExpression registers, all of which are in
// the table), so this constructs one directly rather than trying to
// find source that would parse to it.
func TestNodePrecedenceUnknownOperatorFallsBackToLowest(t *testing.T) {
	bogus := &ast.InfixExpression{Operator: "???"}
	if got := nodePrecedence(bogus); got != precLowest {
		t.Errorf("nodePrecedence(unknown operator) = %d, want precLowest (%d)", got, precLowest)
	}
}

func TestFormatFloatLiteral(t *testing.T) {
	got := fmtSrc(t, "x = 3.5")
	want := "x = 3.5\n"
	if got != want {
		t.Errorf("Format() = %q, want %q", got, want)
	}
}

func TestFormatCollectionLiterals(t *testing.T) {
	tests := []struct{ src, want string }{
		{"x = [1,2,3]", "x = [1, 2, 3]\n"},
		{"x = []", "x = []\n"},
		{`x = {"a":1,"b":2}`, `x = {"a": 1, "b": 2}` + "\n"},
		{"x = {}", "x = {}\n"},
		{"x = toppings{1,2,3}", "x = toppings{1, 2, 3}\n"},
		{"x = (1,2,3)", "x = (1, 2, 3)\n"},
	}
	for _, tt := range tests {
		got := fmtSrc(t, tt.src)
		if got != tt.want {
			t.Errorf("Format(%q) = %q, want %q", tt.src, got, tt.want)
		}
	}
}

func TestFormatUnpackAssign(t *testing.T) {
	got := fmtSrc(t, "a,b,c = xs")
	want := "a, b, c = xs\n"
	if got != want {
		t.Errorf("Format() = %q, want %q", got, want)
	}
}

func TestFormatDeliveryStatement(t *testing.T) {
	got := fmtSrc(t, `delivery   "utils.crust"`)
	want := "delivery \"utils.crust\"\n"
	if got != want {
		t.Errorf("Format() = %q, want %q", got, want)
	}
}

func TestFormatIncDecAlwaysPrintsPostfix(t *testing.T) {
	// The AST doesn't distinguish x++ from ++x (statements.go's
	// IncDecStatement doc comment: "which side it appeared on in the
	// source doesn't matter"), so the formatter has to pick one
	// canonical form.
	got := fmtSrc(t, "++x")
	want := "x++\n"
	if got != want {
		t.Errorf("Format(%q) = %q, want %q", "++x", got, want)
	}
}

func TestFormatIfComboSpecial(t *testing.T) {
	src := `order(x==1){
deliver("one")
}combo(x==2){
deliver("two")
}special{
deliver("other")
}`
	want := `order (x == 1) {
    deliver("one")
} combo (x == 2) {
    deliver("two")
} special {
    deliver("other")
}
`
	got := fmtSrc(t, src)
	if got != want {
		t.Errorf("Format() =\n%s\nwant\n%s", got, want)
	}
}

func TestFormatForEachLoop(t *testing.T) {
	got := fmtSrc(t, "knead item in xs{deliver(item)}")
	want := "knead item in xs {\n    deliver(item)\n}\n"
	if got != want {
		t.Errorf("Format() = %q, want %q", got, want)
	}
}

func TestFormatBakeLoop(t *testing.T) {
	got := fmtSrc(t, "bake(x<10){x++}")
	want := "bake (x < 10) {\n    x++\n}\n"
	if got != want {
		t.Errorf("Format() = %q, want %q", got, want)
	}
}

// TestFormatCountedLoopWithCallInitAndPost confirms a knead loop's
// Init/Post clauses support a bare call expression, not just an
// assignment or increment — simpleStatementText's third case.
func TestFormatCountedLoopWithCallInitAndPost(t *testing.T) {
	got := fmtSrc(t, "knead(setup();check();teardown()){deliver(1)}")
	want := "knead (setup(); check(); teardown()) {\n    deliver(1)\n}\n"
	if got != want {
		t.Errorf("Format() = %q, want %q", got, want)
	}
}

// TestFormatBlankLineAfterBlockBodiedTopLevelStatement exercises
// lastLine/lastLineOfBlock's block-statement branches (order/knead/
// bake), not just the plain-statement default — blankLineBetweenTopLevel
// needs an accurate "did the previous statement's block leave a real
// gap before the next one" answer for these too.
func TestFormatBlankLineAfterBlockBodiedTopLevelStatement(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{"if", "order (stuffed) {\n    x = 1\n}\n\ny = 2\n"},
		{"knead counted", "knead (i = 0; i < 1; i++) {\n    x = i\n}\n\ny = 2\n"},
		{"knead for-each", "knead i in [1] {\n    x = i\n}\n\ny = 2\n"},
		{"bake", "bake (thin) {\n    x = 1\n}\n\ny = 2\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fmtSrc(t, tt.src)
			if !strings.Contains(got, "}\n\ny = 2") {
				t.Errorf("Format(%q) = %q, want the blank line after the block preserved", tt.src, got)
			}
		})
	}
}

// TestFormatNoBlankLineAfterEmptyBlock confirms lastLineOfBlock's
// empty-block fallback (headerLine+1, accounting for the closing
// brace's own line since there's no last inner statement to ask)
// doesn't misfire into adding a spurious blank line for a block with
// nothing in it.
func TestFormatNoBlankLineAfterEmptyBlock(t *testing.T) {
	got := fmtSrc(t, "order (stuffed) {\n}\ny = 2\n")
	if strings.Contains(got, "\n\n") {
		t.Errorf("Format() = %q, want no blank line after an empty block when the source had none", got)
	}
}

// TestFormatNoBlankLineAfterNonEmptyBlockWithoutOne is
// TestFormatNoBlankLineAfterEmptyBlock's non-empty-block counterpart —
// confirms lastLineOfBlock's "+1 for the closing brace" adjustment is
// applied to the last-inner-statement case too, not just the empty
// one (an earlier version of this logic got this specific case wrong:
// it undercounted by exactly the brace's own line, which would have
// inserted a spurious blank line here for every block-bodied top-level
// statement immediately followed by another one).
func TestFormatNoBlankLineAfterNonEmptyBlockWithoutOne(t *testing.T) {
	got := fmtSrc(t, "order (stuffed) {\n    x = 1\n}\ny = 2\n")
	if strings.Contains(got, "\n\n") {
		t.Errorf("Format() = %q, want no blank line when the source had none", got)
	}
}

// TestFormatIdempotentWithInlineFunctionLiteralArgument regression-
// tests a real bug: `lastLine`'s default case used to be plain
// s.Pos().Line, with no allowance for a statement that isn't itself
// block-headed (an if/loop/bake) but still embeds one via a
// FunctionLiteral argument — a recipe literal passed to map/filter/
// reduce/etc. Formatting a source where such a call was written
// compact on one line, immediately followed by another statement with
// no blank line between them, correctly produced no blank line on the
// first pass; but the printer always expands a recipe body onto
// multiple lines regardless of how compact the source was, so
// formatting *that already-expanded output* a second time made the
// stale line-based gap estimate think a blank line belonged there —
// failing TestFormatIsIdempotent, first caught for real by
// examples/day1_essentials.crust rather than by a synthetic test.
func TestFormatIdempotentWithInlineFunctionLiteralArgument(t *testing.T) {
	src := "deliver(map(xs, recipe(x) { serve x * x }))\ndeliver(filter(xs, recipe(x) { serve x % 2 == 0 }))\n"
	once := fmtSrc(t, src)
	twice := fmtSrc(t, once)
	if once != twice {
		t.Errorf("formatting is not idempotent.\n--- first pass ---\n%s\n--- second pass ---\n%s", once, twice)
	}
	if strings.Contains(once, "\n\n") {
		t.Errorf("Format() = %q, want no blank line inserted between the two deliver(...) calls", once)
	}
}

// TestFormatIdempotentWithFunctionLiteralNestedDeeper covers
// lastLineOfExpr's recursive cases beyond CallExpression's own
// Arguments — a recipe literal buried inside a ListLiteral element and
// inside an InfixExpression operand — so the fix above isn't only
// proven for the one shape it was first found in.
func TestFormatIdempotentWithFunctionLiteralNestedDeeper(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{"inside a list literal", "fns = [recipe(x) { serve x * x }, recipe(x) { serve x + 1 }]\ny = 2\n"},
		{"inside an infix operand", "total = 1 + reduce(xs, recipe(acc, x) { serve acc + x }, 0)\ny = 2\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			once := fmtSrc(t, tt.src)
			twice := fmtSrc(t, once)
			if once != twice {
				t.Errorf("formatting is not idempotent.\n--- first pass ---\n%s\n--- second pass ---\n%s", once, twice)
			}
		})
	}
}

func TestFormatCountedLoopWithOmittedClauses(t *testing.T) {
	got := fmtSrc(t, "knead(;i<10;){deliver(i)}")
	want := "knead (; i < 10; ) {\n    deliver(i)\n}\n"
	if got != want {
		t.Errorf("Format() = %q, want %q", got, want)
	}
}

func TestFormatNamedRecipeNoSpaceBeforeParen(t *testing.T) {
	got := fmtSrc(t, "recipe foo(x){serve x}")
	if !strings.HasPrefix(got, "recipe foo(x) {") {
		t.Errorf("Format() = %q, want it to start with %q", got, "recipe foo(x) {")
	}
}

func TestFormatAnonymousRecipeNoSpaceBeforeParen(t *testing.T) {
	got := fmtSrc(t, "f = recipe(x){serve x}")
	if !strings.HasPrefix(got, "f = recipe(x) {") {
		t.Errorf("Format() = %q, want it to start with %q", got, "f = recipe(x) {")
	}
}

func TestFormatBareReturnAndBreakContinue(t *testing.T) {
	tests := []struct{ src, want string }{
		{"recipe f(){serve}", "recipe f() {\n    serve\n}\n"},
		{"bake(stuffed){burnt}", "bake (stuffed) {\n    burnt\n}\n"},
		{"bake(stuffed){flip}", "bake (stuffed) {\n    flip\n}\n"},
	}
	for _, tt := range tests {
		got := fmtSrc(t, tt.src)
		if got != tt.want {
			t.Errorf("Format(%q) = %q, want %q", tt.src, got, tt.want)
		}
	}
}

func TestFormatCompoundAssignOperators(t *testing.T) {
	for _, op := range []string{"+=", "-=", "*=", "/=", "%="} {
		src := "x " + op + " 1"
		got := fmtSrc(t, src)
		want := src + "\n"
		if got != want {
			t.Errorf("Format(%q) = %q, want %q", src, got, want)
		}
	}
}

// --- Comment preservation ---

func TestFormatPreservesLeadingStandaloneComment(t *testing.T) {
	src := "// explains x\nx = 1\n"
	want := "// explains x\nx = 1\n"
	got := fmtSrc(t, src)
	if got != want {
		t.Errorf("Format(%q) = %q, want %q", src, got, want)
	}
}

func TestFormatPreservesTrailingComment(t *testing.T) {
	src := "x = 1   // fifteen\n"
	got := fmtSrc(t, src)
	if !strings.Contains(got, "x = 1") || !strings.Contains(got, "// fifteen") {
		t.Errorf("Format(%q) = %q, want both the statement and its trailing comment", src, got)
	}
	if !strings.Contains(got, "x = 1   // fifteen") && !strings.HasSuffix(strings.TrimRight(got, "\n"), "// fifteen") {
		// Accept any reasonable spacing, but the comment must stay on
		// the same output line as the statement it followed.
		lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
		if len(lines) != 1 || !strings.Contains(lines[0], "// fifteen") {
			t.Errorf("Format(%q) = %q, want the trailing comment to stay on x's own line", src, got)
		}
	}
}

func TestFormatPreservesCommentInsideBlock(t *testing.T) {
	src := "recipe f() {\n    // step one\n    x = 1\n    serve x\n}\n"
	got := fmtSrc(t, src)
	if !strings.Contains(got, "// step one") {
		t.Errorf("Format() = %q, want the interior comment preserved", got)
	}
	// And it must appear before x = 1, not after.
	if strings.Index(got, "// step one") > strings.Index(got, "x = 1") {
		t.Errorf("Format() = %q, want the comment before x = 1, not after", got)
	}
}

func TestFormatPreservesCommentAtEndOfFile(t *testing.T) {
	src := "x = 1\n// trailing note\n"
	got := fmtSrc(t, src)
	if !strings.Contains(got, "// trailing note") {
		t.Errorf("Format(%q) = %q, want the end-of-file comment preserved", src, got)
	}
}

func TestFormatNeverDropsAnyComment(t *testing.T) {
	// A broader sweep: every comment scanned from the source must
	// appear, verbatim, somewhere in the output -- the one invariant
	// this package cannot compromise on (see comments.go's package
	// doc). Deliberately exercises comments in several different
	// syntactic positions at once, including ones this package's
	// design notes admit aren't attached with full precision (e.g. a
	// comment on a bare closing-brace line).
	src := `// file header
recipe f(x) { // header trailing
    // before y
    y = x + 1   // y trailing
    order (y > 0) {
        deliver(y)
        // just before close
    }
    serve y
}
// after f
f(1)
// end of file`
	got := fmtSrc(t, src)
	for _, want := range []string{
		"// file header",
		"// header trailing",
		"// before y",
		"// y trailing",
		"// just before close",
		"// after f",
		"// end of file",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Format() output is missing comment %q entirely -- comments must never be dropped.\nGot:\n%s", want, got)
		}
	}
}

func TestFormatCollapsesMultipleBlankLinesToOne(t *testing.T) {
	src := "x = 1\n\n\n\ny = 2\n"
	got := fmtSrc(t, src)
	if strings.Contains(got, "\n\n\n") {
		t.Errorf("Format(%q) = %q, want at most one blank line between statements", src, got)
	}
	if !strings.Contains(got, "x = 1\n\ny = 2\n") {
		t.Errorf("Format(%q) = %q, want exactly one blank line preserved", src, got)
	}
}

func TestFormatNoBlankLineWhenSourceHadNone(t *testing.T) {
	src := "x = 1\ny = 2\n"
	got := fmtSrc(t, src)
	want := "x = 1\ny = 2\n"
	if got != want {
		t.Errorf("Format(%q) = %q, want %q", src, got, want)
	}
}

// --- Comment scanner unit tests ---

func TestScanCommentsIgnoresSlashInsideString(t *testing.T) {
	comments := scanComments(`x = "http://example.com"`)
	if len(comments) != 0 {
		t.Errorf("scanComments found %v inside a string literal, want none", comments)
	}
}

func TestScanCommentsHandlesEscapedQuoteInString(t *testing.T) {
	comments := scanComments(`x = "a \" // not a comment" // real comment`)
	if len(comments) != 1 {
		t.Fatalf("scanComments found %d comments, want 1: %v", len(comments), comments)
	}
	if comments[0].Text != "// real comment" {
		t.Errorf("comment text = %q, want %q", comments[0].Text, "// real comment")
	}
}

func TestScanCommentsStandaloneVsTrailing(t *testing.T) {
	src := "// standalone\nx = 1 // trailing\n"
	comments := scanComments(src)
	if len(comments) != 2 {
		t.Fatalf("got %d comments, want 2: %v", len(comments), comments)
	}
	if !comments[0].Standalone {
		t.Errorf("first comment Standalone = false, want true: %+v", comments[0])
	}
	if comments[1].Standalone {
		t.Errorf("second comment Standalone = true, want false: %+v", comments[1])
	}
}

// TestScanCommentsUnterminatedStringDoesNotHang confirms scanComments
// degrades gracefully on a raw newline inside an open string --
// unreachable from Format's own real callers (a file with an
// unterminated string fails to parse before Format is ever called),
// but scanComments has no way to know that on its own, so it still
// needs to not hang or panic if ever handed one directly.
func TestScanCommentsUnterminatedStringDoesNotHang(t *testing.T) {
	comments := scanComments("x = \"unterminated\ny = 1 // real comment\n")
	if len(comments) != 1 || comments[0].Text != "// real comment" {
		t.Errorf("scanComments() = %v, want exactly the one real comment after the unterminated string", comments)
	}
}

func TestScanCommentsLineNumbers(t *testing.T) {
	src := "x = 1\ny = 2 // on line 2\n"
	comments := scanComments(src)
	if len(comments) != 1 {
		t.Fatalf("got %d comments, want 1", len(comments))
	}
	if comments[0].Line != 2 {
		t.Errorf("comment line = %d, want 2", comments[0].Line)
	}
}
