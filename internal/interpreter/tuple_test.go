package interpreter

import (
	"testing"

	"github.com/Sintfoap/cRust/internal/object"
)

// --- Tuple literals ----------------------------------------------------

func TestTupleLiteral(t *testing.T) {
	result := testEval(t, "(1, 2, 3)")
	tup, ok := result.(*object.Tuple)
	if !ok {
		t.Fatalf("got %T, want *object.Tuple", result)
	}
	if len(tup.Elements) != 3 {
		t.Fatalf("got %d elements, want 3", len(tup.Elements))
	}
	wantInteger(t, tup.Elements[0], 1)
}

func TestTupleLiteralUnhashableElementIsError(t *testing.T) {
	errObj := wantError(t, testEval(t, "(1, [1, 2])"), "unhashable")
	if errObj == nil {
		t.Fatal("expected an error")
	}
}

func TestTupleLiteralOfHashableTypesIsFine(t *testing.T) {
	// Integer, Float, String, Boolean, and a nested Tuple should all
	// be valid Tuple elements.
	testEval(t, `(1, 2.5, "x", stuffed, (1, 2))`)
}

// --- Indexing ------------------------------------------------------------

func TestTupleIndex(t *testing.T) {
	wantInteger(t, testEval(t, "(10, 20, 30)[1]"), 20)
}

func TestTupleIndexOutOfRange(t *testing.T) {
	wantError(t, testEval(t, "(1, 2)[5]"), "index out of range")
}

func TestTupleIndexNonInteger(t *testing.T) {
	wantError(t, testEval(t, `(1, 2)["x"]`), "must be an Integer")
}

func TestTupleIndexAssignmentIsError(t *testing.T) {
	wantError(t, testEval(t, "t = (1, 2)\nt[0] = 9"), "immutable")
}

// --- Equality --------------------------------------------------------------

func TestTupleEquality(t *testing.T) {
	wantBoolean(t, testEval(t, "(1, 2) == (1, 2)"), true)
	wantBoolean(t, testEval(t, "(1, 2) == (1, 3)"), false)
	wantBoolean(t, testEval(t, "(1, 2) == (1, 2, 3)"), false)
	wantBoolean(t, testEval(t, "(1, 2) != (2, 1)"), true)
}

func TestTupleNotEqualToList(t *testing.T) {
	// Different types compare unequal, same as every other cross-type
	// comparison (SPEC.md §6) -- a Tuple and a List with identical
	// contents aren't the same value.
	wantBoolean(t, testEval(t, "(1, 2) == [1, 2]"), false)
}

// --- Iteration -------------------------------------------------------------

func TestKneadOverTuple(t *testing.T) {
	input := `
total = 0
knead x in (1, 2, 3) {
    total += x
}
total
`
	wantInteger(t, testEval(t, input), 6)
}

// --- slices() ----------------------------------------------------------

func TestSlicesOfTuple(t *testing.T) {
	wantInteger(t, testEval(t, "slices((1, 2, 3))"), 3)
}

// --- Set/Map hashability ----------------------------------------------

func TestTupleAsSetElement(t *testing.T) {
	input := `
seen = toppings{}
sprinkle(seen, (1, 2))
topped(seen, (1, 2))
`
	wantBoolean(t, testEval(t, input), true)
}

func TestTupleSetLiteralWithTuples(t *testing.T) {
	wantInteger(t, testEval(t, "slices(toppings{(1, 2), (1, 2), (3, 4)})"), 2)
}

// --- Unpacking through arbitrary expressions ----------------------------

func TestTupleUnpackThroughTernary(t *testing.T) {
	// The motivating case: a ternary choosing between two Tuples,
	// unpacked directly -- doesn't need to be a literal at the
	// unpack-assignment's own RHS.
	input := `
c = "a"
lastchr = "b"
result, lastchr = c != lastchr (| (c, c) |) (c, lastchr)
result
`
	wantString(t, testEval(t, input), "a")
}

func TestTupleUnpackThroughFunctionCall(t *testing.T) {
	input := `
recipe minMax(a, b) {
    serve a < b (| (a, b) |) (b, a)
}
lo, hi = minMax(5, 2)
lo
`
	wantInteger(t, testEval(t, input), 2)
}

func TestTupleUnpackRLEWithTernary(t *testing.T) {
	// The original motivating example from conversation: a ternary
	// picking between two Tuples, unpacked with both targets ending up
	// bare values -- this is exactly what plain List literals in the
	// same position cannot do (the last target would always be wrapped
	// in a List).
	input := `
result = ""
lastchr = nobox
chars_ = ["a", "a", "b", "b", "b", "c"]
knead c in chars_ {
    result, lastchr = c != lastchr (| (result + c, c) |) (result, lastchr)
}
result
`
	wantString(t, testEval(t, input), "abc")
}
