package object

import "testing"

func TestBuiltin(t *testing.T) {
	called := false
	b := &Builtin{Fn: func(args ...Object) Object {
		called = true
		return NULL
	}}

	if b.Type() != BUILTIN_OBJ {
		t.Errorf("Type() = %v, want %v", b.Type(), BUILTIN_OBJ)
	}
	if b.Inspect() != "builtin function" {
		t.Errorf("Inspect() = %q, want %q", b.Inspect(), "builtin function")
	}

	result := b.Fn(NewInteger(1), NewInteger(2))
	if !called {
		t.Error("Fn was not invoked")
	}
	if result != NULL {
		t.Errorf("Fn(...) = %v, want NULL", result)
	}
}
