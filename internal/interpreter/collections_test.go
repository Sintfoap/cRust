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

func TestPushAppendsInPlace(t *testing.T) {
	input := `
a = [1, 2]
push(a, 3)
a
`
	result := testEval(t, input)
	list := result.(*object.List)
	if len(list.Elements) != 3 {
		t.Fatalf("got %d elements, want 3", len(list.Elements))
	}
	wantInteger(t, list.Elements[2], 3)
}

func TestPushMutatesSharedReference(t *testing.T) {
	input := `
recipe addOne(list) {
    push(list, 1)
}
a = []
addOne(a)
a
`
	result := testEval(t, input)
	list := result.(*object.List)
	if len(list.Elements) != 1 {
		t.Errorf("push through a function call didn't mutate the caller's list: len = %d, want 1", len(list.Elements))
	}
}

// --- map() builtin (higher-order, not the Map type) -----------------------

func TestMapBuiltinWithLambda(t *testing.T) {
	input := `
squared = map([1, 2, 3], recipe(x) { serve x * x })
squared
`
	result := testEval(t, input)
	list := result.(*object.List)
	want := []int64{1, 4, 9}
	if len(list.Elements) != len(want) {
		t.Fatalf("got %d elements, want %d", len(list.Elements), len(want))
	}
	for i, w := range want {
		wantInteger(t, list.Elements[i], w)
	}
}

func TestMapBuiltinWithNamedFunction(t *testing.T) {
	input := `
recipe double(x) {
    serve x * 2
}
map([1, 2, 3], double)
`
	result := testEval(t, input)
	list := result.(*object.List)
	wantInteger(t, list.Elements[0], 2)
	wantInteger(t, list.Elements[2], 6)
}

func TestMapBuiltinWithBuiltinFunction(t *testing.T) {
	// A builtin (str) passed as the transform, not just a user recipe.
	input := `map([1, 2, 3], str)`
	result := testEval(t, input)
	list := result.(*object.List)
	wantString(t, list.Elements[0], "1")
}

func TestMapBuiltinComposesMultipleSteps(t *testing.T) {
	// The user's motivating pattern: knead over ints(split(line)) for
	// every line, via a lambda that chains split then ints -- map
	// itself only takes one function, composition happens by writing
	// a lambda that calls both.
	input := `
lines_ = ["1 2 3", "4 5"]
rows = map(lines_, recipe(line) { serve ints(split(line)) })
total = 0
knead row in rows {
    knead n in row {
        total += n
    }
}
total
`
	wantInteger(t, testEval(t, input), 15)
}

func TestMapBuiltinClosesOverEnclosingScope(t *testing.T) {
	input := `
factor = 10
scaled = map([1, 2, 3], recipe(x) { serve x * factor })
scaled
`
	result := testEval(t, input)
	list := result.(*object.List)
	wantInteger(t, list.Elements[2], 30)
}

func TestMapBuiltinErrorInFunctionPropagates(t *testing.T) {
	wantError(t, testEval(t, `map([1, 0], recipe(x) { serve 1 / x })`), "division by zero")
}

func TestMapBuiltinOnTuple(t *testing.T) {
	result := testEval(t, `map((1, 2, 3), recipe(x) { serve x + 1 })`)
	list := result.(*object.List)
	wantInteger(t, list.Elements[0], 2)
	wantInteger(t, list.Elements[2], 4)
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
