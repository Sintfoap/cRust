package object

import "testing"

func TestListIsAReferenceType(t *testing.T) {
	original := NewList([]Object{NewInteger(1), NewInteger(2)})

	// Simulate what "assigning" a List to another variable looks like
	// at the Go level: copying the *List pointer, not the struct.
	alias := original
	alias.Elements = append(alias.Elements, NewInteger(3))

	if len(original.Elements) != 3 {
		t.Errorf("mutating through alias didn't affect original: len = %d, want 3", len(original.Elements))
	}
}

func TestListInspect(t *testing.T) {
	l := NewList([]Object{NewInteger(1), NewInteger(2), &String{Value: "hi"}})
	want := `[1, 2, hi]`
	if got := l.Inspect(); got != want {
		t.Errorf("Inspect() = %q, want %q", got, want)
	}
}

func TestEmptyListInspect(t *testing.T) {
	l := NewList(nil)
	if got := l.Inspect(); got != "[]" {
		t.Errorf("Inspect() = %q, want %q", got, "[]")
	}
}
