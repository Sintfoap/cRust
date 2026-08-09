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

// --- Error.Frames (stack traces) ---------------------------------------------

// TestErrorFramesSingleCallHasOneFrame confirms a single level of call
// wrapping still records exactly one frame (Error.FrameLines' own doc
// comment on why that one frame alone doesn't get *printed*, but the
// data itself should still be there for anything else that might want
// it).
func TestErrorFramesSingleCallHasOneFrame(t *testing.T) {
	errObj := wantError(t, testEval(t, `
recipe divide(a, b) { serve a / b }
divide(1, 0)
`), "division by zero")

	if len(errObj.Frames) != 1 {
		t.Fatalf("Frames = %+v, want exactly 1", errObj.Frames)
	}
	if errObj.Frames[0].Name != "divide(...)" {
		t.Errorf("Frames[0].Name = %q, want %q", errObj.Frames[0].Name, "divide(...)")
	}
}

// TestErrorFramesNestedCallsBuildInnermostFirst is this feature's core
// claim: divide -> process -> (top level), each appended as the error
// unwinds back up through applyFunction, in the order they're
// unwound — innermost (the recipe whose body actually failed) first.
func TestErrorFramesNestedCallsBuildInnermostFirst(t *testing.T) {
	errObj := wantError(t, testEval(t, `
recipe divide(a, b) {
    serve a / b
}
recipe process(x) {
    serve divide(x, 0)
}
process(10)
`), "division by zero")

	if len(errObj.Frames) != 2 {
		t.Fatalf("Frames = %+v, want exactly 2", errObj.Frames)
	}
	if got := errObj.Frames[0].Name; got != "divide(...)" {
		t.Errorf("Frames[0].Name = %q, want %q (innermost first)", got, "divide(...)")
	}
	if got := errObj.Frames[1].Name; got != "process(...)" {
		t.Errorf("Frames[1].Name = %q, want %q", got, "process(...)")
	}
	// Frames[0].Line/Col is where divide() was called *from* -- inside
	// process's own body, line 6 (`serve divide(x, 0)`).
	if errObj.Frames[0].Line != 6 {
		t.Errorf("Frames[0].Line = %d, want 6 (where divide() was called from)", errObj.Frames[0].Line)
	}
	// Frames[1].Line/Col is where process() was called from -- the
	// top-level call on line 8.
	if errObj.Frames[1].Line != 8 {
		t.Errorf("Frames[1].Line = %d, want 8 (where process() was called from)", errObj.Frames[1].Line)
	}
}

// TestErrorFramesRecursionOneFramePerLevel confirms recursive calls
// each contribute their own frame rather than collapsing into one --
// this is what falls out of appending a frame every time applyFunction
// unwinds through a *object.Function call, with no special-casing
// needed for recursion at all (it's just Go's own call stack
// unwinding, once per nested applyFunction call, recursive or not).
func TestErrorFramesRecursionOneFramePerLevel(t *testing.T) {
	errObj := wantError(t, testEval(t, `
recipe boom(n) {
    order (n <= 0) {
        serve 1 / 0
    }
    serve boom(n - 1)
}
boom(3)
`), "division by zero")

	if len(errObj.Frames) != 4 {
		t.Fatalf("Frames = %+v, want 4 (boom(3)->boom(2)->boom(1)->boom(0))", errObj.Frames)
	}
	for i, f := range errObj.Frames {
		if f.Name != "boom(...)" {
			t.Errorf("Frames[%d].Name = %q, want %q", i, f.Name, "boom(...)")
		}
	}
}

// TestErrorFramesBuiltinDoesNotAddItsOwnFrame confirms a builtin's
// error gets no frame for the builtin itself (it has no recipe body to
// unwind through) but still picks up a frame once it propagates
// through the enclosing recipe's own applyFunction.
func TestErrorFramesBuiltinDoesNotAddItsOwnFrame(t *testing.T) {
	errObj := wantError(t, testEval(t, `
recipe process() {
    serve sqrt(-1)
}
process()
`), "")

	if len(errObj.Frames) != 1 {
		t.Fatalf("Frames = %+v, want exactly 1 (process, not sqrt)", errObj.Frames)
	}
	if errObj.Frames[0].Name != "process(...)" {
		t.Errorf("Frames[0].Name = %q, want %q", errObj.Frames[0].Name, "process(...)")
	}
}

// TestErrorFramesCallExportedUsesGenericLabel confirms Call's own
// eagerly-set "call(...)" label is what names the frame when there's
// no source-level call expression for frameName to read from (Call's
// own doc comment on why this label is deliberately generic).
func TestErrorFramesCallExportedUsesGenericLabel(t *testing.T) {
	interp := New(io.Discard, strings.NewReader(""))
	env := object.NewEnvironment()
	testEvalWith(t, interp, env, `apply = recipe(x) { serve x / 0 }`)
	fn, _ := env.Get("apply")

	errObj := wantError(t, interp.Call(fn, []object.Object{object.NewInteger(1)}), "division by zero")
	if len(errObj.Frames) != 1 || errObj.Frames[0].Name != "call(...)" {
		t.Errorf("Frames = %+v, want a single call(...) frame", errObj.Frames)
	}
}

// TestErrorFramesCallNamedUsesRealName is CallExportedUsesGenericLabel's
// counterpart for CallNamed — the frame should carry the real name
// passed in, the same one the debugger's KPI/stepper attributes the
// run to (CallNamed's own doc comment).
func TestErrorFramesCallNamedUsesRealName(t *testing.T) {
	interp := New(io.Discard, strings.NewReader(""))
	env := object.NewEnvironment()
	testEvalWith(t, interp, env, `apply = recipe(x) { serve x / 0 }`)
	fn, _ := env.Get("apply")

	errObj := wantError(t, interp.CallNamed(fn, []object.Object{object.NewInteger(1)}, "store_part1"), "division by zero")
	if len(errObj.Frames) != 1 || errObj.Frames[0].Name != "store_part1(...)" {
		t.Errorf("Frames = %+v, want a single store_part1(...) frame", errObj.Frames)
	}
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
