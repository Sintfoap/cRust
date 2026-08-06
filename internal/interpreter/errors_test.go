package interpreter

import (
	"io"
	"strings"
	"testing"

	"github.com/Sintfoap/cRust/internal/lexer"
	"github.com/Sintfoap/cRust/internal/object"
	"github.com/Sintfoap/cRust/internal/parser"
)

// TestCallExported exercises Interpreter.Call directly — the entry
// point cmd/crust's --store resolution uses (SPEC.md §9), which has no
// source-level CallExpression to go through Eval.
func TestCallExported(t *testing.T) {
	interp := New(io.Discard, strings.NewReader(""))
	env := object.NewEnvironment()

	fn := testEvalWith(t, interp, env, `apply = recipe(x) { serve x + 1 }`)
	_ = fn
	got, ok := env.Get("apply")
	if !ok {
		t.Fatal("apply was not bound in env")
	}

	result := interp.Call(got, []object.Object{object.NewInteger(41)})
	wantInteger(t, result, 42)
}

func TestCallExportedOnNonFunction(t *testing.T) {
	interp := New(io.Discard, strings.NewReader(""))
	result := interp.Call(object.NewInteger(5), nil)
	wantError(t, result, "not a recipe")
}

// TestCallNamedExported exercises Interpreter.CallNamed the same way
// TestCallExported does for Call — it's the variant cmd/crust's --store
// entry-point resolution uses (SPEC.md §9) so the debugger's KPI/
// stepper can attribute the run to the real recipe name instead of
// Call's generic "call(...)" frame label.
func TestCallNamedExported(t *testing.T) {
	interp := New(io.Discard, strings.NewReader(""))
	env := object.NewEnvironment()

	testEvalWith(t, interp, env, `apply = recipe(x) { serve x + 1 }`)
	got, ok := env.Get("apply")
	if !ok {
		t.Fatal("apply was not bound in env")
	}

	result := interp.CallNamed(got, []object.Object{object.NewInteger(41)}, "store_part1")
	wantInteger(t, result, 42)
}

func TestCallNamedExportedOnNonFunction(t *testing.T) {
	interp := New(io.Discard, strings.NewReader(""))
	result := interp.CallNamed(object.NewInteger(5), nil, "store")
	wantError(t, result, "not a recipe")
}

func testEvalWith(t *testing.T, interp *Interpreter, env *object.Environment, input string) object.Object {
	t.Helper()
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parser errors for %q: %v", input, errs)
	}
	return interp.Eval(program, env)
}

// --- Error propagation through every remaining collection/condition site -------

func TestErrorInListLiteralElement(t *testing.T) {
	wantError(t, testEval(t, "[1, 1/0, 3]"), "division by zero")
}

func TestErrorInMapLiteralKey(t *testing.T) {
	wantError(t, testEval(t, "m = {(1/0): 1}"), "division by zero")
}

func TestErrorInMapLiteralValue(t *testing.T) {
	wantError(t, testEval(t, `m = {"a": 1/0}`), "division by zero")
}

func TestErrorInSetLiteralElement(t *testing.T) {
	wantError(t, testEval(t, "toppings{1/0}"), "division by zero")
}

func TestErrorInCallArguments(t *testing.T) {
	input := `
recipe f(a, b) { serve a }
f(1, 1/0)
`
	wantError(t, testEval(t, input), "division by zero")
}

func TestErrorAsCallTarget(t *testing.T) {
	wantError(t, testEval(t, "(1/0)()"), "division by zero")
}

func TestErrorInIfCondition(t *testing.T) {
	wantError(t, testEval(t, "order (1/0) { deliver(1) }"), "division by zero")
}

func TestErrorInComboCondition(t *testing.T) {
	input := `order (thin) { deliver(1) } combo (1/0) { deliver(2) }`
	wantError(t, testEval(t, input), "division by zero")
}

func TestErrorInCountedLoopClauses(t *testing.T) {
	wantError(t, testEval(t, "knead (x = 1/0; stuffed;) { burnt }"), "division by zero")
	wantError(t, testEval(t, "knead (; 1/0;) { burnt }"), "division by zero")
	// burnt on the very first pass would skip Post entirely (see
	// TestCountedLoopBurntStopsImmediatelyWithoutRunningPost), so this
	// needs the loop to actually complete an iteration without
	// breaking before Post's error can be reached.
	wantError(t, testEval(t, "i = 0\nknead (; i < 2; x = 1/0) { i = i + 1 }"), "division by zero")
}

func TestErrorInForEachCollection(t *testing.T) {
	wantError(t, testEval(t, "knead x in (1/0) { deliver(x) }"), "division by zero")
}

func TestErrorInBakeCondition(t *testing.T) {
	wantError(t, testEval(t, "bake (1/0) { burnt }"), "division by zero")
}

func TestErrorInIndexAssignBase(t *testing.T) {
	wantError(t, testEval(t, "(1/0)[0] = 1"), "division by zero")
}

func TestErrorInIndexAssignIndex(t *testing.T) {
	wantError(t, testEval(t, "xs = [1, 2]\nxs[1/0] = 1"), "division by zero")
}

func TestErrorInIndexAssignValue(t *testing.T) {
	wantError(t, testEval(t, "xs = [1, 2]\nxs[0] = 1/0"), "division by zero")
}

func TestWriteToNonIndexableType(t *testing.T) {
	wantError(t, testEval(t, "x = 5\nx[0] = 1"), "not indexable")
}

func TestIndexIncDecOnNonIndexableType(t *testing.T) {
	wantError(t, testEval(t, "x = 5\nx[0]++"), "not indexable")
}

// --- object.Equal edge cases (via the == operator) ------------------------------

func TestFunctionEqualityIsByIdentity(t *testing.T) {
	input := `
a = recipe(x) { serve x }
b = recipe(x) { serve x }
a == b
`
	wantBoolean(t, testEval(t, input), false)

	input = `
a = recipe(x) { serve x }
b = a
a == b
`
	wantBoolean(t, testEval(t, input), true)
}

func TestSetEqualityDifferentSizes(t *testing.T) {
	wantBoolean(t, testEval(t, "toppings{1, 2} == toppings{1, 2, 3}"), false)
}

func TestListEqualityDifferentTypesAtSamePosition(t *testing.T) {
	wantBoolean(t, testEval(t, `[1] == ["1"]`), false)
}

func TestIsErrorOnNil(t *testing.T) {
	if isError(nil) {
		t.Error("isError(nil) = true, want false")
	}
}

func TestErrorInReturnValue(t *testing.T) {
	input := `
recipe f() {
    serve 1 / 0
}
f()
`
	wantError(t, testEval(t, input), "division by zero")
}

func TestErrorInTernaryCondition(t *testing.T) {
	wantError(t, testEval(t, `(1/0) (| "a" |) "b"`), "division by zero")
}

func TestErrorInElvisLeft(t *testing.T) {
	wantError(t, testEval(t, "(1/0) ?: 5"), "division by zero")
}

func TestErrorInLogicalLeft(t *testing.T) {
	wantError(t, testEval(t, "(1/0) with stuffed"), "division by zero")
	wantError(t, testEval(t, "(1/0) or thin"), "division by zero")
}

func TestErrorInLogicalRight(t *testing.T) {
	wantError(t, testEval(t, "stuffed with (1/0)"), "division by zero")
	wantError(t, testEval(t, "thin or (1/0)"), "division by zero")
}

func TestErrorInUnpackValue(t *testing.T) {
	wantError(t, testEval(t, "a, b = 1/0"), "division by zero")
}
