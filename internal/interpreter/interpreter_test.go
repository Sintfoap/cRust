package interpreter

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Sintfoap/cRust/internal/lexer"
	"github.com/Sintfoap/cRust/internal/object"
	"github.com/Sintfoap/cRust/internal/parser"
)

// testEval lexes, parses, and evaluates input against a fresh
// Interpreter/Environment, discarding any `deliver` output. Fails the
// test immediately on a parse error, since that's never what a test in
// this package means to exercise.
func testEval(t *testing.T, input string) object.Object {
	t.Helper()
	_, result := testEvalCapture(t, input)
	return result
}

func testEvalCapture(t *testing.T, input string) (string, object.Object) {
	t.Helper()
	_, out, result := testEvalCaptureWithStdin(t, input, "")
	return out, result
}

// testEvalCaptureWithStdin is testEvalCapture with a caller-supplied
// stdin, for tests exercising `unbox()`'s no-argument (stdin-reading)
// form.
func testEvalCaptureWithStdin(t *testing.T, input, stdin string) (*Interpreter, string, object.Object) {
	t.Helper()
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parser errors for %q: %v", input, errs)
	}

	var buf bytes.Buffer
	interp := New(&buf, strings.NewReader(stdin))
	env := object.NewEnvironment()
	result := interp.Eval(program, env)
	return interp, buf.String(), result
}

func wantInteger(t *testing.T, got object.Object, want int64) {
	t.Helper()
	i, ok := got.(*object.Integer)
	if !ok {
		t.Fatalf("got %T (%s), want *object.Integer", got, got.Inspect())
	}
	if i.Value != want {
		t.Errorf("Value = %d, want %d", i.Value, want)
	}
}

func wantFloat(t *testing.T, got object.Object, want float64) {
	t.Helper()
	f, ok := got.(*object.Float)
	if !ok {
		t.Fatalf("got %T (%s), want *object.Float", got, got.Inspect())
	}
	if f.Value != want {
		t.Errorf("Value = %v, want %v", f.Value, want)
	}
}

func wantString(t *testing.T, got object.Object, want string) {
	t.Helper()
	s, ok := got.(*object.String)
	if !ok {
		t.Fatalf("got %T (%s), want *object.String", got, got.Inspect())
	}
	if s.Value != want {
		t.Errorf("Value = %q, want %q", s.Value, want)
	}
}

func wantBoolean(t *testing.T, got object.Object, want bool) {
	t.Helper()
	b, ok := got.(*object.Boolean)
	if !ok {
		t.Fatalf("got %T (%s), want *object.Boolean", got, got.Inspect())
	}
	if b.Value != want {
		t.Errorf("Value = %v, want %v", b.Value, want)
	}
}

func wantNull(t *testing.T, got object.Object) {
	t.Helper()
	if got != object.NULL {
		t.Errorf("got %T (%s), want object.NULL", got, got.Inspect())
	}
}

func wantError(t *testing.T, got object.Object, wantSubstr string) *object.Error {
	t.Helper()
	e, ok := got.(*object.Error)
	if !ok {
		t.Fatalf("got %T (%s), want *object.Error", got, got.Inspect())
	}
	if wantSubstr != "" && !strings.Contains(e.Message, wantSubstr) {
		t.Errorf("Message = %q, want it to contain %q", e.Message, wantSubstr)
	}
	return e
}

// --- Literals ---------------------------------------------------------------

func TestEvalIntegerLiteral(t *testing.T) {
	wantInteger(t, testEval(t, "5"), 5)
}

func TestEvalFloatLiteral(t *testing.T) {
	wantFloat(t, testEval(t, "3.14"), 3.14)
}

func TestEvalStringLiteral(t *testing.T) {
	wantString(t, testEval(t, `"hello"`), "hello")
}

func TestEvalBooleanLiterals(t *testing.T) {
	wantBoolean(t, testEval(t, "stuffed"), true)
	wantBoolean(t, testEval(t, "thin"), false)
}

func TestEvalNilLiteral(t *testing.T) {
	wantNull(t, testEval(t, "nobox"))
}

// --- Arithmetic ---------------------------------------------------------------

func TestEvalIntegerArithmetic(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"5 + 5", 10},
		{"5 - 5", 0},
		{"5 * 5", 25},
		{"5 % 2", 1},
		{"-5 + 10", 5},
		{"2 * (3 + 4)", 14},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			wantInteger(t, testEval(t, tt.input), tt.want)
		})
	}
}

func TestDivisionAlwaysProducesFloat(t *testing.T) {
	wantFloat(t, testEval(t, "10 / 2"), 5.0)
	wantFloat(t, testEval(t, "7 / 2"), 3.5)
}

func TestMixedIntFloatWidensToFloat(t *testing.T) {
	wantFloat(t, testEval(t, "1 + 1.5"), 2.5)
	wantFloat(t, testEval(t, "1.5 - 1"), 0.5)
	wantFloat(t, testEval(t, "2 * 1.5"), 3.0)
}

func TestModuloRequiresTwoIntegers(t *testing.T) {
	wantError(t, testEval(t, "5.0 % 2"), "requires two Integers")
	wantError(t, testEval(t, "5 % 2.0"), "requires two Integers")
}

func TestDivisionByZero(t *testing.T) {
	wantError(t, testEval(t, "1 / 0"), "division by zero")
	wantError(t, testEval(t, "1 % 0"), "division by zero")
	wantError(t, testEval(t, "1.0 / 0"), "division by zero")
}

func TestStringConcatenation(t *testing.T) {
	wantString(t, testEval(t, `"foo" + "bar"`), "foobar")
}

func TestStringPlusNumberIsTypeError(t *testing.T) {
	wantError(t, testEval(t, `"foo" + 1`), "type error")
	wantError(t, testEval(t, `1 + "foo"`), "type error")
}

func TestListConcatenation(t *testing.T) {
	result := testEval(t, "[1, 2] + [3, 4]")
	list := result.(*object.List)
	if len(list.Elements) != 4 {
		t.Fatalf("got %d elements, want 4", len(list.Elements))
	}
	wantInteger(t, list.Elements[0], 1)
	wantInteger(t, list.Elements[3], 4)
}

func TestListConcatenationDoesNotMutateOperands(t *testing.T) {
	input := `
a = [1, 2]
b = [3, 4]
c = a + b
a
`
	result := testEval(t, input)
	list := result.(*object.List)
	if len(list.Elements) != 2 {
		t.Errorf("original list a was mutated by +: len = %d, want 2", len(list.Elements))
	}
}

func TestListPlusOperatorAssign(t *testing.T) {
	wantInteger(t, testEval(t, "a = [1]\na += [2]\na[1]"), 2)
}

func TestListPlusNonListIsTypeError(t *testing.T) {
	wantError(t, testEval(t, "[1] + 5"), "type error")
	wantError(t, testEval(t, "5 + [1]"), "type error")
}

func TestTupleConcatenation(t *testing.T) {
	result := testEval(t, "(1, 2) + (3, 4)")
	tup := result.(*object.Tuple)
	if len(tup.Elements) != 4 {
		t.Fatalf("got %d elements, want 4", len(tup.Elements))
	}
	wantInteger(t, tup.Elements[0], 1)
	wantInteger(t, tup.Elements[3], 4)
}

func TestTuplePlusNonTupleIsTypeError(t *testing.T) {
	wantError(t, testEval(t, "(1, 2) + 5"), "type error")
	wantError(t, testEval(t, "(1, 2) + [1, 2]"), "type error")
}

func TestUnsupportedArithmeticOperands(t *testing.T) {
	wantError(t, testEval(t, "stuffed - thin"), "")
	wantError(t, testEval(t, "[1] + 5"), "")
	wantError(t, testEval(t, "(1, 2) + [1, 2]"), "")
}

// --- Comparison and equality ---------------------------------------------------

func TestNumericComparison(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"1 < 2", true},
		{"2 < 1", false},
		{"1 <= 1", true},
		{"1 >= 2", false},
		{"1 == 1", true},
		{"1 != 1", false},
		{"1 == 1.0", true},
		{"1.5 < 2", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			wantBoolean(t, testEval(t, tt.input), tt.want)
		})
	}
}

func TestStringComparison(t *testing.T) {
	wantBoolean(t, testEval(t, `"a" < "b"`), true)
	wantBoolean(t, testEval(t, `"apple" == "apple"`), true)
	wantBoolean(t, testEval(t, `"apple" != "orange"`), true)
}

func TestCrossTypeEqualityIsAlwaysFalseNeverAnError(t *testing.T) {
	wantBoolean(t, testEval(t, `1 == "1"`), false)
	wantBoolean(t, testEval(t, `1 != "1"`), true)
	wantBoolean(t, testEval(t, "stuffed == 1"), false)
	wantBoolean(t, testEval(t, "nobox == thin"), false)
}

func TestMismatchedOrderingIsAnError(t *testing.T) {
	wantError(t, testEval(t, `1 < "1"`), "cannot compare")
	wantError(t, testEval(t, "stuffed < thin"), "cannot compare")
}

func TestListEqualityByContent(t *testing.T) {
	wantBoolean(t, testEval(t, "[1, 2, 3] == [1, 2, 3]"), true)
	wantBoolean(t, testEval(t, "[1, 2] == [1, 2, 3]"), false)
	wantBoolean(t, testEval(t, "[1, [2, 3]] == [1, [2, 3]]"), true)
}

func TestSetEqualityIgnoresOrder(t *testing.T) {
	wantBoolean(t, testEval(t, "toppings{1, 2} == toppings{2, 1}"), true)
}

func TestMapEqualityByContent(t *testing.T) {
	wantBoolean(t, testEval(t, `a = {"a": 1, "b": 2}
b = {"b": 2, "a": 1}
a == b`), true)
	wantBoolean(t, testEval(t, `a = {"a": 1}
b = {"a": 2}
a == b`), false)
}

// --- Prefix operators -----------------------------------------------------------

func TestUnaryMinus(t *testing.T) {
	wantInteger(t, testEval(t, "-5"), -5)
	wantFloat(t, testEval(t, "-5.5"), -5.5)
}

func TestUnaryMinusTypeError(t *testing.T) {
	wantError(t, testEval(t, `-"foo"`), "")
}

func TestHold(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"hold stuffed", false},
		{"hold thin", true},
		{"hold nobox", true},
		{"hold 0", false},
		{"hold 1", false},
		{`hold ""`, false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			wantBoolean(t, testEval(t, tt.input), tt.want)
		})
	}
}

// --- Truthiness (SPEC.md §6): only thin/nobox are falsy -------------------------

func TestFalsyValuesInOrder(t *testing.T) {
	// order's block only runs on a truthy condition; use serve to
	// observe whether the falsy branch (special) or truthy branch ran.
	tests := []struct {
		input string
		want  string
	}{
		{"order (thin) { serve \"yes\" } special { serve \"no\" }", "no"},
		{"order (nobox) { serve \"yes\" } special { serve \"no\" }", "no"},
		{"order (0) { serve \"yes\" } special { serve \"no\" }", "yes"},
		{"order (0.0) { serve \"yes\" } special { serve \"no\" }", "yes"},
		{`order ("") { serve "yes" } special { serve "no" }`, "yes"},
		{"order ([]) { serve \"yes\" } special { serve \"no\" }", "yes"},
		{"order (toppings{}) { serve \"yes\" } special { serve \"no\" }", "yes"},
		{"order (stuffed) { serve \"yes\" } special { serve \"no\" }", "yes"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := testEval(t, "recipe f() { "+tt.input+" } f()")
			wantString(t, result, tt.want)
		})
	}
}

// --- with/or short-circuiting ----------------------------------------------------

func TestWithOrProduceStrictBooleans(t *testing.T) {
	wantBoolean(t, testEval(t, "stuffed with stuffed"), true)
	wantBoolean(t, testEval(t, "stuffed with thin"), false)
	wantBoolean(t, testEval(t, "thin or stuffed"), true)
	wantBoolean(t, testEval(t, "thin or thin"), false)
	// Truthy-but-not-Boolean operands still coerce to a strict Boolean.
	wantBoolean(t, testEval(t, "1 with 2"), true)
}

func TestWithShortCircuitsOnFalsyLeft(t *testing.T) {
	out, result := testEvalCapture(t, `thin with deliver("should not run")`)
	wantBoolean(t, result, false)
	if out != "" {
		t.Errorf("right side of `with` was evaluated despite short-circuiting: output = %q", out)
	}
}

func TestOrShortCircuitsOnTruthyLeft(t *testing.T) {
	out, result := testEvalCapture(t, `stuffed or deliver("should not run")`)
	wantBoolean(t, result, true)
	if out != "" {
		t.Errorf("right side of `or` was evaluated despite short-circuiting: output = %q", out)
	}
}

// --- Ternary and Elvis --------------------------------------------------------

func TestTernary(t *testing.T) {
	wantString(t, testEval(t, `stuffed (| "yes" |) "no"`), "yes")
	wantString(t, testEval(t, `thin (| "yes" |) "no"`), "no")
}

func TestTernaryOnlyEvaluatesTakenBranch(t *testing.T) {
	out, result := testEvalCapture(t, `stuffed (| "yes" |) deliver("should not run")`)
	wantString(t, result, "yes")
	if out != "" {
		t.Errorf("untaken branch was evaluated: output = %q", out)
	}
}

func TestChainedTernary(t *testing.T) {
	input := `
score = 82
(
    score >= 90 (| "A" |)
    score >= 80 (| "B" |)
    score >= 70 (| "C" |)
    "F"
)
`
	wantString(t, testEval(t, input), "B")
}

func TestElvis(t *testing.T) {
	wantInteger(t, testEval(t, "nobox ?: 5"), 5)
	wantInteger(t, testEval(t, "3 ?: 5"), 3)
}

func TestElvisTestsForNoboxSpecificallyNotFalsiness(t *testing.T) {
	wantBoolean(t, testEval(t, "thin ?: 99"), false)
	wantInteger(t, testEval(t, "0 ?: 99"), 0)
}

func TestElvisShortCircuits(t *testing.T) {
	out, result := testEvalCapture(t, `5 ?: deliver("should not run")`)
	wantInteger(t, result, 5)
	if out != "" {
		t.Errorf("right side of ?: was evaluated despite left not being nobox: output = %q", out)
	}
}

// --- Ranges -----------------------------------------------------------------

func TestRangeInclusive(t *testing.T) {
	result := testEval(t, "1..5")
	list := result.(*object.List)
	if len(list.Elements) != 5 {
		t.Fatalf("got %d elements, want 5", len(list.Elements))
	}
	wantInteger(t, list.Elements[0], 1)
	wantInteger(t, list.Elements[4], 5)
}

func TestRangeExclusive(t *testing.T) {
	result := testEval(t, "1.<5")
	list := result.(*object.List)
	if len(list.Elements) != 4 {
		t.Fatalf("got %d elements, want 4", len(list.Elements))
	}
	wantInteger(t, list.Elements[3], 4)
}

func TestRangeStartGreaterThanEndIsEmpty(t *testing.T) {
	result := testEval(t, "5..1")
	list := result.(*object.List)
	if len(list.Elements) != 0 {
		t.Fatalf("got %d elements, want 0", len(list.Elements))
	}
}

func TestExclusiveRangeOfEqualBoundsIsEmpty(t *testing.T) {
	result := testEval(t, "1.<1")
	list := result.(*object.List)
	if len(list.Elements) != 0 {
		t.Fatalf("got %d elements, want 0", len(list.Elements))
	}
}

func TestRangeRequiresIntegerBounds(t *testing.T) {
	wantError(t, testEval(t, "1.0..5"), "range bounds must be Integers")
	wantError(t, testEval(t, "1..5.0"), "range bounds must be Integers")
}

// --- Error short-circuiting ----------------------------------------------------

func TestErrorHaltsFurtherEvaluation(t *testing.T) {
	out, result := testEvalCapture(t, `
x = 1 / 0
deliver("should not run")
`)
	wantError(t, result, "division by zero")
	if out != "" {
		t.Errorf("statement after the error still ran: output = %q", out)
	}
}

func TestErrorInSubexpressionPropagates(t *testing.T) {
	wantError(t, testEval(t, "1 + (1 / 0)"), "division by zero")
}

// --- Input and type conversion builtins -----------------------------------

func TestUnboxReadsInjectedStdin(t *testing.T) {
	_, _, result := testEvalCaptureWithStdin(t, `unbox()`, "puzzle input\n")
	wantString(t, result, "puzzle input\n")
}

func TestUnboxLinesPipeline(t *testing.T) {
	_, buf, _ := testEvalCaptureWithStdin(t, `
deliver(lines(unbox()))
`, "1\n2\n3\n")
	if buf != "[1, 2, 3]\n" {
		t.Errorf("output = %q, want the three lines as a List", buf)
	}
}

func TestTypeConversionRoundTrip(t *testing.T) {
	wantInteger(t, testEval(t, `int(str(42))`), 42)
	wantString(t, testEval(t, `str(int("7") + int(float("3.9")))`), "10")
}
