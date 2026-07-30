package interpreter

import (
	"testing"

	"github.com/Sintfoap/cRust/internal/object"
)

// --- List literals and indexing --------------------------------------------

func TestListLiteral(t *testing.T) {
	result := testEval(t, "[1, 2, 3]")
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("got %T, want *object.List", result)
	}
	if len(list.Elements) != 3 {
		t.Fatalf("got %d elements, want 3", len(list.Elements))
	}
	wantInteger(t, list.Elements[0], 1)
}

func TestListIndex(t *testing.T) {
	wantInteger(t, testEval(t, "[10, 20, 30][1]"), 20)
}

func TestListIndexOutOfRange(t *testing.T) {
	wantError(t, testEval(t, "[1, 2][5]"), "index out of range")
}

func TestListIndexNegativeIsError(t *testing.T) {
	// SPEC.md's own examples compute the last index manually
	// (slices(list)-1) rather than using -1, which implies negative
	// indexing isn't supported.
	wantError(t, testEval(t, "[1, 2][-1]"), "index out of range")
}

func TestListIndexNonInteger(t *testing.T) {
	wantError(t, testEval(t, `[1, 2]["x"]`), "must be an Integer")
}

// --- Map literals and indexing --------------------------------------------

func TestMapLiteral(t *testing.T) {
	input := `m = {"a": 1, "b": 2}
m["a"]`
	wantInteger(t, testEval(t, input), 1)
}

func TestMapIntegerKey(t *testing.T) {
	input := `m = {1: "one", 2: "two"}
m[1]`
	wantString(t, testEval(t, input), "one")
}

func TestMapLiteralRejectsInvalidKeyType(t *testing.T) {
	wantError(t, testEval(t, "m = {[1]: 2}"), "map keys must be a String or Integer")
}

func TestMapMissingKeyReadsAsNobox(t *testing.T) {
	// SPEC.md's memoization idiom (cache[n] = cache[n] ?: compute(n))
	// only works if a missing key evaluates to nobox, not an error.
	input := `m = {"a": 1}
m["missing"]`
	wantNull(t, testEval(t, input))
}

func TestMemoizationIdiom(t *testing.T) {
	input := `
calls = 0
cache = {}
recipe computeSlow(n) {
    calls = calls + 1
    serve n * n
}
recipe cached(n) {
    cache[n] = cache[n] ?: computeSlow(n)
    serve cache[n]
}
cached(5)
cached(5)
cached(5)
calls
`
	// computeSlow should only run once despite three calls to cached(5).
	wantInteger(t, testEval(t, input), 1)
}

// --- Set literals -----------------------------------------------------------

func TestSetLiteral(t *testing.T) {
	result := testEval(t, "toppings{1, 2, 2, 3}")
	set, ok := result.(*object.Set)
	if !ok {
		t.Fatalf("got %T, want *object.Set", result)
	}
	if set.Len() != 3 {
		t.Errorf("Len() = %d, want 3 (duplicates dropped)", set.Len())
	}
}

func TestSetIsNotIndexable(t *testing.T) {
	wantError(t, testEval(t, "toppings{1, 2}[0]"), "not indexable")
}

func TestSetUnhashableElementIsError(t *testing.T) {
	wantError(t, testEval(t, "toppings{[1, 2]}"), "unhashable")
}

// --- String indexing --------------------------------------------------------

func TestStringIndex(t *testing.T) {
	wantString(t, testEval(t, `"hello"[1]`), "e")
}

func TestStringIndexOutOfRange(t *testing.T) {
	wantError(t, testEval(t, `"hi"[10]`), "index out of range")
}

func TestStringIsImmutable(t *testing.T) {
	wantError(t, testEval(t, `s = "hi"
s[0] = "x"`), "immutable")
}

// --- Builtins, exercised through cRust source (not the Go API directly) --------

func TestDeliverBuiltin(t *testing.T) {
	out, _ := testEvalCapture(t, `deliver("hello", 5)`)
	if out != "hello 5\n" {
		t.Errorf("output = %q, want %q", out, "hello 5\n")
	}
}

func TestSlicesBuiltin(t *testing.T) {
	wantInteger(t, testEval(t, `slices("hello")`), 5)
	wantInteger(t, testEval(t, `slices([1, 2, 3])`), 3)
	wantInteger(t, testEval(t, `slices(toppings{1, 2})`), 2)
}

func TestSauceBuiltin(t *testing.T) {
	wantInteger(t, testEval(t, "sauce(nobox, 5)"), 5)
	wantInteger(t, testEval(t, "sauce(3, 5)"), 3)
}

func TestCharsBuiltin(t *testing.T) {
	result := testEval(t, `chars("ab")`)
	list := result.(*object.List)
	if len(list.Elements) != 2 {
		t.Fatalf("got %d elements, want 2", len(list.Elements))
	}
	wantString(t, list.Elements[0], "a")
}

func TestIdivBuiltin(t *testing.T) {
	wantInteger(t, testEval(t, "idiv(7, 2)"), 3)
}

func TestSetBuiltinsRoundTrip(t *testing.T) {
	input := `
s = gather([1, 2, 2, 3])
sprinkle(s, 4)
scrape(s, 1)
deliver(topped(s, 4))
deliver(topped(s, 1))
slices(s)
`
	out, result := testEvalCapture(t, input)
	if out != "stuffed\nthin\n" {
		t.Errorf("output = %q, want %q", out, "stuffed\nthin\n")
	}
	wantInteger(t, result, 3)
}

func TestCombineSharedStripBuiltins(t *testing.T) {
	input := `
a = toppings{1, 2, 3}
b = toppings{2, 3, 4}
slices(combine(a, b))
`
	wantInteger(t, testEval(t, input), 4)

	input = `
a = toppings{1, 2, 3}
b = toppings{2, 3, 4}
slices(shared(a, b))
`
	wantInteger(t, testEval(t, input), 2)

	input = `
a = toppings{1, 2, 3}
b = toppings{2, 3, 4}
slices(strip(a, b))
`
	wantInteger(t, testEval(t, input), 1)
}

func TestBuiltinErrorGetsCallSitePosition(t *testing.T) {
	errObj := wantError(t, testEval(t, "idiv(1, 0)"), "division by zero")
	if errObj.Line == 0 {
		t.Error("Line = 0, want the call site's line to be attached")
	}
}

func TestUserCodeCanShadowABuiltin(t *testing.T) {
	// Builtins are predeclared identifiers, not keywords (SPEC.md §4) —
	// user code is free to redefine them.
	input := `
deliver = recipe(x) { serve x * 2 }
deliver(21)
`
	wantInteger(t, testEval(t, input), 42)
}
