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

// TestEnvironmentDeclareShadowsExistingOuterBinding is Declare's whole
// reason to exist: unlike Set (TestEnvironmentSetMutatesExistingOuterBinding
// above), it must NOT walk out and mutate an outer binding that shares
// the name — it always creates/overwrites a binding in the *current*
// Environment only. This is what keeps a recipe call's parameters
// isolated even when a parameter happens to share a name with a global.
func TestEnvironmentDeclareShadowsExistingOuterBinding(t *testing.T) {
	outer := NewEnvironment()
	outer.Set("g", NewInteger(1))
	inner := NewEnclosedEnvironment(outer)

	inner.Declare("g", NewInteger(999))

	outerVal, _ := outer.Get("g")
	if outerVal.(*Integer).Value != 1 {
		t.Errorf("outer's g = %v, want 1 (Declare must not leak into an enclosing scope)", outerVal)
	}
	innerVal, _ := inner.Get("g")
	if innerVal.(*Integer).Value != 999 {
		t.Errorf("inner's g = %v, want 999", innerVal)
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

func TestEnvironmentSnapshotIncludesLocalBindings(t *testing.T) {
	env := NewEnvironment()
	env.Set("x", NewInteger(1))
	env.Set("y", NewInteger(2))

	snap := env.Snapshot()
	if len(snap) != 2 {
		t.Fatalf("snapshot has %d entries, want 2: %v", len(snap), snap)
	}
	if snap["x"].(*Integer).Value != 1 || snap["y"].(*Integer).Value != 2 {
		t.Errorf("snapshot = %v, want x=1 y=2", snap)
	}
}

func TestEnvironmentSnapshotWalksOuterScopes(t *testing.T) {
	outer := NewEnvironment()
	outer.Set("global", NewInteger(1))
	inner := NewEnclosedEnvironment(outer)
	inner.Declare("local", NewInteger(2))

	snap := inner.Snapshot()
	if snap["global"].(*Integer).Value != 1 {
		t.Errorf("snapshot missing outer binding: %v", snap)
	}
	if snap["local"].(*Integer).Value != 2 {
		t.Errorf("snapshot missing local binding: %v", snap)
	}
}

func TestEnvironmentSnapshotInnerShadowsOuter(t *testing.T) {
	outer := NewEnvironment()
	outer.Set("x", NewInteger(1))
	inner := NewEnclosedEnvironment(outer)
	inner.Declare("x", NewInteger(99))

	snap := inner.Snapshot()
	if snap["x"].(*Integer).Value != 99 {
		t.Errorf("snapshot[x] = %v, want the inner (shadowing) value 99", snap["x"])
	}
}

// TestEnvironmentSnapshotIsPointInTime is Snapshot's whole reason to
// exist: a caller holding an old snapshot must not see a *rebinding*
// (Set replacing the map entry, as opposed to in-place mutation of a
// still-shared value) that happens after the snapshot was taken.
func TestEnvironmentSnapshotIsPointInTime(t *testing.T) {
	env := NewEnvironment()
	env.Set("x", NewInteger(1))

	snap := env.Snapshot()
	env.Set("x", NewInteger(2))

	if snap["x"].(*Integer).Value != 1 {
		t.Errorf("snap[x] = %v, want 1 (the value at snapshot time, unaffected by the later rebinding)", snap["x"])
	}
	live, _ := env.Get("x")
	if live.(*Integer).Value != 2 {
		t.Errorf("env.Get(x) = %v, want 2 (the rebinding did happen, just not through the old snapshot)", live)
	}
}

func TestEnvironmentSnapshotEmptyEnvironment(t *testing.T) {
	snap := NewEnvironment().Snapshot()
	if len(snap) != 0 {
		t.Errorf("snapshot of an empty environment = %v, want empty", snap)
	}
}
