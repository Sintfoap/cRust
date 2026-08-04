package interpreter

import (
	"testing"

	"github.com/Sintfoap/cRust/internal/object"
)

// --- Assignment -----------------------------------------------------------

func TestPlainAssignment(t *testing.T) {
	wantInteger(t, testEval(t, "x = 5\nx"), 5)
}

func TestCompoundAssignment(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"x = 5\nx += 3\nx", 8},
		{"x = 5\nx -= 3\nx", 2},
		{"x = 5\nx *= 3\nx", 15},
		{"x = 5\nx %= 3\nx", 2},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			wantInteger(t, testEval(t, tt.input), tt.want)
		})
	}
}

func TestCompoundDivideAssignWidensToFloat(t *testing.T) {
	wantFloat(t, testEval(t, "x = 5\nx /= 2\nx"), 2.5)
}

func TestCompoundAssignmentOnUndefinedVariableIsError(t *testing.T) {
	wantError(t, testEval(t, "x += 1"), "undefined variable")
}

func TestAssignmentScopingRule(t *testing.T) {
	// SPEC.md §3: assignment walks outward and mutates an existing
	// binding rather than always creating a new local one.
	input := `
total = 0
recipe sum(nums) {
    knead i in nums {
        total = total + i
    }
    serve total
}
sum([1, 2, 3])
total
`
	wantInteger(t, testEval(t, input), 6)
}

func TestClosureMutatesCapturedVariable(t *testing.T) {
	input := `
tally = 0
recipe bump() {
    tally = tally + 1
}
bump()
bump()
bump()
tally
`
	wantInteger(t, testEval(t, input), 3)
}

// TestParameterShadowsSameNamedOuterVariable is a regression test for a
// real scoping bug: a recipe's parameter binding used to go through
// Environment.Set, which walks outer scopes looking for an existing
// binding to mutate (exactly what TestClosureMutatesCapturedVariable
// above relies on for a *free* variable). Applied to a *parameter*
// instead, that meant calling a function whose parameter happened to
// share a name with an existing outer (e.g. global) variable would
// silently overwrite that outer variable on every reassignment inside
// the function body — found while writing a grid-simulation example
// where a recipe parameter and a top-level variable were both named
// `g`. Parameters must always bind fresh in the call's own local
// scope (object.Environment.Declare now), never alias an outer
// variable just because the names collide.
func TestParameterShadowsSameNamedOuterVariable(t *testing.T) {
	input := `
recipe touch(g) {
    g = 999
}
g = 1
touch(g)
g
`
	wantInteger(t, testEval(t, input), 1)
}

func TestIndexAssignment(t *testing.T) {
	wantInteger(t, testEval(t, "xs = [1, 2, 3]\nxs[1] = 99\nxs[1]"), 99)
}

func TestIndexAssignmentOutOfRange(t *testing.T) {
	wantError(t, testEval(t, "xs = [1, 2, 3]\nxs[5] = 1"), "index out of range")
}

func TestIndexCompoundAssignment(t *testing.T) {
	wantInteger(t, testEval(t, "xs = [1, 2, 3]\nxs[0] += 10\nxs[0]"), 11)
}

func TestIndexAssignmentEvaluatesBaseAndIndexOnce(t *testing.T) {
	// If evalIndexAssign re-evaluated target.Left/target.Index for a
	// compound op's read-then-write, this counter would double-count.
	input := `
calls = 0
xs = [10, 20, 30]
recipe idx() {
    calls = calls + 1
    serve 1
}
xs[idx()] += 5
calls
`
	wantInteger(t, testEval(t, input), 1)
}

func TestMapIndexAssignmentInvalidKeyType(t *testing.T) {
	wantError(t, testEval(t, "m = {}\nm[[1]] = 1"), "map keys must be Hashable")
}

func TestInvalidAssignmentTargets(t *testing.T) {
	wantError(t, testEval(t, "s = toppings{1}\ns[0] = 1"), "not indexable")
}

// --- Unpacking assignment ---------------------------------------------------

func TestUnpackAssignment(t *testing.T) {
	input := `
entries = [1, 2, 3, 4]
first, rest = entries
rest
`
	result := testEval(t, input)
	list := result.(*object.List)
	if len(list.Elements) != 3 {
		t.Fatalf("got %d elements, want 3", len(list.Elements))
	}
	wantInteger(t, list.Elements[0], 2)
}

func TestUnpackAssignmentLastTargetAlwaysAList(t *testing.T) {
	input := `
a, b = [1, 2]
b
`
	result := testEval(t, input)
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("got %T, want *object.List (last target always a List)", result)
	}
	if len(list.Elements) != 1 {
		t.Fatalf("got %d elements, want 1", len(list.Elements))
	}
	wantInteger(t, list.Elements[0], 2)
}

func TestUnpackAssignmentEmptyRest(t *testing.T) {
	input := `
a, b = [1]
b
`
	result := testEval(t, input)
	list := result.(*object.List)
	if len(list.Elements) != 0 {
		t.Fatalf("got %d elements, want 0", len(list.Elements))
	}
}

func TestUnpackAssignmentNotEnoughElements(t *testing.T) {
	wantError(t, testEval(t, "a, b, c = [1]"), "not enough values")
}

func TestUnpackAssignmentRequiresList(t *testing.T) {
	wantError(t, testEval(t, "a, b = toppings{1, 2}"), "cannot unpack")
	wantError(t, testEval(t, "a, b = 5"), "cannot unpack")
}

func TestTupleUnpackBothTargetsAreBareValues(t *testing.T) {
	input := `
a, b = (1, 2)
b
`
	// Unlike List-unpack, the last target is a bare value here, not a
	// one-element List.
	wantInteger(t, testEval(t, input), 2)
}

func TestTupleUnpackThreeWay(t *testing.T) {
	wantInteger(t, testEval(t, "a, b, c = (1, 2, 3)\na + b + c"), 6)
}

func TestTupleUnpackSwapIdiom(t *testing.T) {
	input := `
x = 10
y = 20
x, y = (y, x)
x
`
	wantInteger(t, testEval(t, input), 20)
	input2 := `
x = 10
y = 20
x, y = (y, x)
y
`
	wantInteger(t, testEval(t, input2), 10)
}

func TestTupleUnpackArityMismatch(t *testing.T) {
	wantError(t, testEval(t, "a, b = (1, 2, 3)"), "need exactly 2")
	wantError(t, testEval(t, "a, b, c = (1, 2)"), "need exactly 3")
}

func TestTupleUnpackErrorInValuePropagates(t *testing.T) {
	wantError(t, testEval(t, "a, b = (1 / 0, 2)"), "division by zero")
}

// TestTupleUnpackSingleParenFallsBackToListUnpack confirms
// `a, b = (xs)` (no comma) is unaffected by tuple-sugar and keeps the
// ordinary List-unpack semantics — the last target is still a List.
func TestTupleUnpackSingleParenFallsBackToListUnpack(t *testing.T) {
	input := `
xs = [1, 2]
a, b = (xs)
b
`
	result := testEval(t, input)
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("got %T, want *object.List", result)
	}
	wantInteger(t, list.Elements[0], 2)
}

// --- Inc/Dec ----------------------------------------------------------------

func TestIncDecIdentifier(t *testing.T) {
	wantInteger(t, testEval(t, "x = 5\nx++\nx"), 6)
	wantInteger(t, testEval(t, "x = 5\n++x\nx"), 6)
	wantInteger(t, testEval(t, "x = 5\nx--\nx"), 4)
}

func TestIncDecFloat(t *testing.T) {
	wantFloat(t, testEval(t, "x = 5.5\nx++\nx"), 6.5)
}

func TestIncDecIndex(t *testing.T) {
	wantInteger(t, testEval(t, "xs = [1, 2, 3]\nxs[0]++\nxs[0]"), 2)
}

func TestIncDecTypeError(t *testing.T) {
	wantError(t, testEval(t, `x = "foo"
x++`), "expected Integer or Float")
}

func TestIncDecUndefinedVariable(t *testing.T) {
	wantError(t, testEval(t, "x++"), "undefined variable")
}

// --- order/combo/special -----------------------------------------------------

func TestIfStatementConsequence(t *testing.T) {
	input := `recipe f() { order (stuffed) { serve 1 } serve 2 } f()`
	wantInteger(t, testEval(t, input), 1)
}

func TestIfStatementCombo(t *testing.T) {
	input := `
recipe grade(n) {
    order (n >= 90) {
        serve "A"
    } combo (n >= 80) {
        serve "B"
    } combo (n >= 70) {
        serve "C"
    } special {
        serve "F"
    }
}
grade(85)
`
	wantString(t, testEval(t, input), "B")
}

func TestIfStatementFallsThroughToSpecial(t *testing.T) {
	input := `
recipe grade(n) {
    order (n >= 90) {
        serve "A"
    } special {
        serve "F"
    }
}
grade(10)
`
	wantString(t, testEval(t, input), "F")
}

func TestIfStatementNoBranchMatchesIsNull(t *testing.T) {
	input := `recipe f() { order (thin) { serve 1 } } f()`
	wantNull(t, testEval(t, input))
}

// --- knead counted loop --------------------------------------------------------

func TestCountedLoop(t *testing.T) {
	input := `
total = 0
knead (i = 0; i < 5; i++) {
    total = total + i
}
total
`
	wantInteger(t, testEval(t, input), 10)
}

func TestCountedLoopBurntStopsImmediatelyWithoutRunningPost(t *testing.T) {
	input := `
postRuns = 0
i = 0
knead (; i < 10; postRuns = postRuns + 1) {
    order (i == 3) {
        burnt
    }
    i = i + 1
}
postRuns
`
	// burnt fires when i==3 (after 3 increments via body, so postRuns
	// should reflect exactly 3 completed iterations before the break —
	// Post must NOT run on the iteration that breaks.
	wantInteger(t, testEval(t, input), 3)
}

func TestCountedLoopFlipStillRunsPost(t *testing.T) {
	input := `
sum = 0
knead (i = 0; i < 5; i++) {
    order (i == 2) {
        flip
    }
    sum = sum + i
}
sum
`
	// 0 + 1 + 3 + 4 = 8 (2 skipped by flip, but the loop still completes
	// all 5 iterations since Post still runs after flip).
	wantInteger(t, testEval(t, input), 8)
}

func TestCountedLoopAllClausesOptional(t *testing.T) {
	input := `
i = 0
knead (;;) {
    order (i >= 3) {
        burnt
    }
    i = i + 1
}
i
`
	wantInteger(t, testEval(t, input), 3)
}

// --- knead for-each loop --------------------------------------------------------

func TestForEachOverList(t *testing.T) {
	input := `
total = 0
knead x in [1, 2, 3] {
    total = total + x
}
total
`
	wantInteger(t, testEval(t, input), 6)
}

func TestForEachOverSet(t *testing.T) {
	input := `
count = 0
knead x in toppings{1, 2, 3} {
    count = count + 1
}
count
`
	wantInteger(t, testEval(t, input), 3)
}

func TestForEachOverMapIteratesKeys(t *testing.T) {
	input := `
count = 0
m = {"a": 1, "b": 2}
knead k in m {
    count = count + 1
}
count
`
	wantInteger(t, testEval(t, input), 2)
}

func TestForEachBurntAndFlip(t *testing.T) {
	input := `
sum = 0
knead x in [1, 2, 3, 4, 5] {
    order (x == 2) {
        flip
    }
    order (x == 4) {
        burnt
    }
    sum = sum + x
}
sum
`
	// 1 contributes, 2 is skipped by flip, 3 contributes, burnt fires on
	// 4 before it can contribute, 5 is never reached: sum = 1 + 3 = 4.
	wantInteger(t, testEval(t, input), 4)
}

func TestForEachOverNonIterableIsError(t *testing.T) {
	wantError(t, testEval(t, "knead x in 5 { deliver(x) }"), "cannot iterate")
}

// --- bake -----------------------------------------------------------------------

func TestBakeLoop(t *testing.T) {
	input := `
i = 0
sum = 0
bake (i < 5) {
    sum = sum + i
    i = i + 1
}
sum
`
	wantInteger(t, testEval(t, input), 10)
}

func TestBakeBurnt(t *testing.T) {
	input := `
i = 0
bake (stuffed) {
    order (i == 3) {
        burnt
    }
    i = i + 1
}
i
`
	wantInteger(t, testEval(t, input), 3)
}

// --- Nested loops: burnt/flip only affect the innermost loop --------------------

func TestBurntOnlyBreaksInnermostLoop(t *testing.T) {
	input := `
outerRuns = 0
knead (i = 0; i < 3; i++) {
    outerRuns = outerRuns + 1
    knead (j = 0; j < 10; j++) {
        order (j == 2) {
            burnt
        }
    }
}
outerRuns
`
	wantInteger(t, testEval(t, input), 3)
}

// --- Functions and closures -----------------------------------------------------

func TestNamedRecipeDeclarationAndCall(t *testing.T) {
	input := `
recipe add(a, b) {
    serve a + b
}
add(2, 3)
`
	wantInteger(t, testEval(t, input), 5)
}

func TestAnonymousRecipeAsValue(t *testing.T) {
	input := `
double = recipe(x) { serve x * 2 }
double(21)
`
	wantInteger(t, testEval(t, input), 42)
}

func TestRecipeWithoutServeReturnsNobox(t *testing.T) {
	input := `
recipe f() {
    x = 1
}
f()
`
	wantNull(t, testEval(t, input))
}

func TestBareServeReturnsNobox(t *testing.T) {
	input := `
recipe f() {
    serve
}
f()
`
	wantNull(t, testEval(t, input))
}

func TestRecursion(t *testing.T) {
	input := `
recipe fact(n) {
    order (n <= 1) {
        serve 1
    }
    serve n * fact(n - 1)
}
fact(5)
`
	wantInteger(t, testEval(t, input), 120)
}

func TestClosureCapturesDefiningScope(t *testing.T) {
	input := `
recipe makeCounter() {
    count = 0
    recipe increment() {
        count = count + 1
        serve count
    }
    serve increment
}
counter = makeCounter()
counter()
counter()
counter()
`
	wantInteger(t, testEval(t, input), 3)
}

func TestReturnPropagatesThroughNestedBlocksAndLoops(t *testing.T) {
	input := `
recipe findFirst(nums, target) {
    knead x in nums {
        order (x == target) {
            serve x
        }
    }
    serve nobox
}
findFirst([1, 2, 3, 4], 3)
`
	wantInteger(t, testEval(t, input), 3)
}

func TestWrongNumberOfArguments(t *testing.T) {
	input := `
recipe add(a, b) { serve a + b }
add(1)
`
	wantError(t, testEval(t, input), "wrong number of arguments")
}

func TestCallingNonFunctionIsError(t *testing.T) {
	wantError(t, testEval(t, "x = 5\nx()"), "not a recipe")
}

func TestCallingUndefinedIdentifierIsError(t *testing.T) {
	wantError(t, testEval(t, "undefinedFn()"), "identifier not found")
}
