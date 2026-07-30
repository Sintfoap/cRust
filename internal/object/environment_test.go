package object

import "testing"

func TestEnvironmentGetSetLocal(t *testing.T) {
	env := NewEnvironment()
	env.Set("x", NewInteger(1))

	got, ok := env.Get("x")
	if !ok {
		t.Fatal("expected x to be found")
	}
	if got.(*Integer).Value != 1 {
		t.Errorf("x = %v, want 1", got)
	}

	_, ok = env.Get("missing")
	if ok {
		t.Error("expected missing to not be found")
	}
}

func TestEnvironmentGetWalksOuter(t *testing.T) {
	outer := NewEnvironment()
	outer.Set("x", NewInteger(1))
	inner := NewEnclosedEnvironment(outer)

	got, ok := inner.Get("x")
	if !ok || got.(*Integer).Value != 1 {
		t.Errorf("inner.Get(x) = %v, %v; want 1, true", got, ok)
	}
}

// TestEnvironmentSetMutatesExistingOuterBinding is the core of
// SPEC.md §3's scoping rule: assigning to a name that already exists
// in an enclosing scope updates it there, rather than shadowing it
// with a new local binding. This is what makes accumulator patterns
// (total = total + x inside a knead loop) and closures mutating a
// captured variable work with no declare keyword.
func TestEnvironmentSetMutatesExistingOuterBinding(t *testing.T) {
	outer := NewEnvironment()
	outer.Set("total", NewInteger(0))
	inner := NewEnclosedEnvironment(outer)

	inner.Set("total", NewInteger(5))

	// The binding must have been updated in outer, not shadowed in inner.
	outerVal, _ := outer.Get("total")
	if outerVal.(*Integer).Value != 5 {
		t.Errorf("outer's total = %v, want 5 (Set should have mutated it in place)", outerVal)
	}

	// inner.store itself should still be empty — nothing new was
	// created locally.
	if _, ok := inner.store["total"]; ok {
		t.Error("Set created a new local binding instead of mutating the existing outer one")
	}
}

// TestEnvironmentSetCreatesLocalWhenNoOuterBindingExists is the other
// half of the same rule: if the name isn't bound anywhere in the
// chain, Set creates it in the *current* (innermost) scope.
func TestEnvironmentSetCreatesLocalWhenNoOuterBindingExists(t *testing.T) {
	outer := NewEnvironment()
	inner := NewEnclosedEnvironment(outer)

	inner.Set("fresh", NewInteger(42))

	if _, ok := outer.Get("fresh"); ok {
		t.Error("fresh should not have leaked into outer")
	}
	got, ok := inner.Get("fresh")
	if !ok || got.(*Integer).Value != 42 {
		t.Errorf("inner.Get(fresh) = %v, %v; want 42, true", got, ok)
	}
}

// TestEnvironmentClosureMutation mirrors SPEC.md §3's example almost
// exactly: a function updating a variable captured from its defining
// scope, with no global/nonlocal keyword.
func TestEnvironmentClosureMutation(t *testing.T) {
	global := NewEnvironment()
	global.Set("tally", NewInteger(0))

	// Simulates a recipe call: a new enclosed environment over the
	// scope the closure captured.
	call := NewEnclosedEnvironment(global)
	current, _ := call.Get("tally")
	call.Set("tally", NewInteger(current.(*Integer).Value+1))

	got, _ := global.Get("tally")
	if got.(*Integer).Value != 1 {
		t.Errorf("tally = %v, want 1", got)
	}
}
