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

func TestGridEqualityByContent(t *testing.T) {
	wantBoolean(t, testEval(t, `grid("ab\ncd") == grid("ab\ncd")`), true)
	wantBoolean(t, testEval(t, `grid("ab") == grid("cd")`), false)
}

func TestGridEqualityConsidersOffset(t *testing.T) {
	// Two grids with the same visible content but a different origin
	// (one shifted by an earlier setAt into negative coordinates) are
	// not the same value -- the same coordinate would mean a different
	// cell on each.
	input := `
a = grid("x")
b = grid("x")
setAt(b, (-1, 0), "y")
a == b
`
	wantBoolean(t, testEval(t, input), false)
}

func TestGridEqualityDifferentDimensions(t *testing.T) {
	wantBoolean(t, testEval(t, `grid("a") == grid("ab")`), false)
	wantBoolean(t, testEval(t, `grid("a") == grid("a\nb")`), false)
}

func TestGridInequality(t *testing.T) {
	wantBoolean(t, testEval(t, `grid("a") != grid("b")`), true)
	wantBoolean(t, testEval(t, `grid("a") != grid("a")`), false)
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

// --- Slices -------------------------------------------------------------------

func wantIntegerList(t *testing.T, got object.Object, want []int64) {
	t.Helper()
	list, ok := got.(*object.List)
	if !ok {
		t.Fatalf("got %T (%v), want *object.List", got, got)
	}
	if len(list.Elements) != len(want) {
		t.Fatalf("got %d elements, want %d (%v)", len(list.Elements), len(want), want)
	}
	for i, w := range want {
		wantInteger(t, list.Elements[i], w)
	}
}

func TestSliceListInclusiveForward(t *testing.T) {
	wantIntegerList(t, testEval(t, "xs = [10, 20, 30, 40, 50]; xs[0..2]"), []int64{10, 20, 30})
}

func TestSliceListExclusiveForward(t *testing.T) {
	wantIntegerList(t, testEval(t, "xs = [10, 20, 30, 40, 50]; xs[0.<2]"), []int64{10, 20})
}

func TestSliceListNegativeBoundsReverse(t *testing.T) {
	wantIntegerList(t, testEval(t, "xs = [10, 20, 30, 40, 50]; xs[-1..0]"), []int64{50, 40, 30, 20, 10})
}

func TestSliceListNegativeBoundsForward(t *testing.T) {
	wantIntegerList(t, testEval(t, "xs = [10, 20, 30, 40, 50]; xs[-2..-1]"), []int64{40, 50})
}

func TestSliceListSingleElementInclusive(t *testing.T) {
	wantIntegerList(t, testEval(t, "xs = [10, 20, 30]; xs[1..1]"), []int64{20})
}

func TestSliceListSingleElementExclusiveIsEmpty(t *testing.T) {
	wantIntegerList(t, testEval(t, "xs = [10, 20, 30]; xs[1.<1]"), []int64{})
}

func TestSliceListBackwardExclusive(t *testing.T) {
	wantIntegerList(t, testEval(t, "xs = [10, 20, 30, 40, 50]; xs[3.<0]"), []int64{40, 30, 20})
}

func TestSliceDoesNotMutateOriginal(t *testing.T) {
	result := testEval(t, `
xs = [10, 20, 30, 40, 50]
ys = xs[0..1]
push(ys, 99)
xs
`)
	wantIntegerList(t, result, []int64{10, 20, 30, 40, 50})
}

func TestSliceOutOfRangeIsError(t *testing.T) {
	wantError(t, testEval(t, "xs = [1, 2, 3]; xs[0..10]"), "slice index out of range")
	wantError(t, testEval(t, "xs = [1, 2, 3]; xs[-10..0]"), "slice index out of range")
}

func TestSliceRequiresIntegerBounds(t *testing.T) {
	wantError(t, testEval(t, `xs = [1, 2, 3]; xs["a"..2]`), "slice bounds must be Integers")
	wantError(t, testEval(t, `xs = [1, 2, 3]; xs[0.."a"]`), "slice bounds must be Integers")
}

func TestSliceTuple(t *testing.T) {
	result := testEval(t, "t = (1, 2, 3, 4, 5); t[1..3]")
	tuple, ok := result.(*object.Tuple)
	if !ok {
		t.Fatalf("got %T, want *object.Tuple", result)
	}
	if len(tuple.Elements) != 3 {
		t.Fatalf("got %d elements, want 3", len(tuple.Elements))
	}
	wantInteger(t, tuple.Elements[0], 2)
	wantInteger(t, tuple.Elements[2], 4)
}

func TestSliceString(t *testing.T) {
	wantString(t, testEval(t, `s = "hello world"; s[0..4]`), "hello")
	wantString(t, testEval(t, `s = "hello world"; s[0.<4]`), "hell")
	wantString(t, testEval(t, `s = "hello"; s[-1..0]`), "olleh")
}

// TestSliceExclusiveEndAtContainerLengthIsValid guards against a real
// off-by-one regression: exclusive slicing's end bound is a boundary
// marker, never a dereferenced position (only end-1 ever gets read
// walking forward), so end == the container's own length is a
// legitimate "read up through the very end" bound — the same way
// Python's s[3:5] on a 5-character string is fine — not an
// out-of-range error. Found while writing an AoC solution's
// last-N-characters slice (`s[slices(s)-2 .< slices(s)]`), which used
// to fail outright.
func TestSliceExclusiveEndAtContainerLengthIsValid(t *testing.T) {
	wantString(t, testEval(t, `s = "170cm"; s[3.<5]`), "cm")
	wantIntegerList(t, testEval(t, "xs = [10, 20, 30]; xs[1.<3]"), []int64{20, 30})
}

// TestRangeEndConsumesTrailingSameLevelOperatorWithoutParens guards
// against a real parser regression: a range's End must swallow a whole
// term, including any of its own trailing "+"/"-" chain, the same way
// term = factor { ("+"|"-") factor } already does on its own — writing
// `xs[0.<a-2]` without parens around `a-2` used to leave the "-2" for
// an outer loop that doesn't exist at the right level, misattaching it
// to the whole slice result instead of just the range's end
// (`xs[0.<a] - 2`, which then fails outright: List/String minus
// Integer isn't defined). Every pre-existing slice test happened to
// always parenthesize a compound end bound (see
// TestSliceExpressionBoundsAreEvaluated's `xs[a..(a+2)]`), which is
// exactly how this stayed undetected.
func TestRangeEndConsumesTrailingSameLevelOperatorWithoutParens(t *testing.T) {
	wantIntegerList(t, testEval(t, "xs = [10, 20, 30, 40, 50]; a = 5; xs[0.<a-2]"), []int64{10, 20, 30})
}

func TestSliceUnsupportedTypeIsError(t *testing.T) {
	wantError(t, testEval(t, "s = toppings{1, 2}; s[0..1]"), "does not support slicing")
}

func TestSliceExpressionBoundsAreEvaluated(t *testing.T) {
	wantIntegerList(t, testEval(t, "xs = [10, 20, 30, 40, 50]; a = 1; xs[a..(a+2)]"), []int64{20, 30, 40})
}

func TestSlicePropagatesStartError(t *testing.T) {
	wantError(t, testEval(t, "xs = [1, 2, 3]; xs[(1 / 0)..2]"), "division by zero")
}

func TestSlicePropagatesEndError(t *testing.T) {
	wantError(t, testEval(t, "xs = [1, 2, 3]; xs[0..(1 / 0)]"), "division by zero")
}

func TestPlainIndexPropagatesIndexError(t *testing.T) {
	wantError(t, testEval(t, "xs = [1, 2, 3]; xs[1 / 0]"), "division by zero")
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
