package object

import (
	"strings"
	"testing"
)

func TestSetAddHasRemove(t *testing.T) {
	s := NewSet()

	if s.Has(NewInteger(1)) {
		t.Fatal("empty set should not contain 1")
	}

	if ok := s.Add(NewInteger(1)); !ok {
		t.Fatal("Add(1) should succeed")
	}
	if !s.Has(NewInteger(1)) {
		t.Error("set should contain 1 after Add(1)")
	}
	if s.Len() != 1 {
		t.Errorf("Len() = %d, want 1", s.Len())
	}

	// Adding an equal-by-value item again must not create a duplicate —
	// that's the entire point of a Set (SPEC.md §2.2).
	s.Add(NewInteger(1))
	if s.Len() != 1 {
		t.Errorf("adding a duplicate value should not grow the set: Len() = %d", s.Len())
	}

	s.Remove(NewInteger(1))
	if s.Has(NewInteger(1)) {
		t.Error("set should not contain 1 after Remove(1)")
	}
	if s.Len() != 0 {
		t.Errorf("Len() = %d, want 0", s.Len())
	}
}

func TestSetRemoveAbsentIsNoOp(t *testing.T) {
	s := NewSet()
	s.Add(NewInteger(1))
	s.Remove(NewInteger(99)) // not present
	if s.Len() != 1 {
		t.Errorf("removing an absent item changed the set: Len() = %d, want 1", s.Len())
	}
}

func TestSetNonHashableItemFails(t *testing.T) {
	s := NewSet()
	if ok := s.Add(NewList(nil)); ok {
		t.Error("Add with a non-Hashable item (List) should fail")
	}
	if s.Has(NewList(nil)) {
		t.Error("a failed Add must not leave the set thinking it contains the item")
	}
}

func TestSetRemoveNonHashableIsNoOp(t *testing.T) {
	s := NewSet()
	s.Add(NewInteger(1))
	s.Remove(NewList(nil)) // not Hashable at all
	if s.Len() != 1 {
		t.Errorf("removing a non-Hashable item should be a no-op, not panic or change the set: Len() = %d", s.Len())
	}
}

func TestSetInspect(t *testing.T) {
	s := NewSet()
	s.Add(NewInteger(1))

	got := s.Inspect()
	want := `toppings{1}`
	if got != want {
		t.Errorf("Inspect() = %q, want %q", got, want)
	}
}

func TestSetInspectMultipleElements(t *testing.T) {
	s := NewSet()
	s.Add(NewInteger(1))
	s.Add(NewInteger(2))

	got := s.Inspect()
	for _, want := range []string{"1", "2", ", "} {
		if !strings.Contains(got, want) {
			t.Errorf("Inspect() = %q, missing %q", got, want)
		}
	}
}

func TestEmptySetInspect(t *testing.T) {
	if got := NewSet().Inspect(); got != "toppings{}" {
		t.Errorf("Inspect() = %q, want %q", got, "toppings{}")
	}
}

func TestSetIsAReferenceType(t *testing.T) {
	original := NewSet()
	alias := original
	alias.Add(NewInteger(1))

	if !original.Has(NewInteger(1)) {
		t.Error("mutating through alias didn't affect original")
	}
}

func TestSetEqualityIgnoresOrderByConstruction(t *testing.T) {
	a := NewSet()
	a.Add(NewInteger(1))
	a.Add(NewInteger(2))

	b := NewSet()
	b.Add(NewInteger(2))
	b.Add(NewInteger(1))

	// Sets are unordered by construction (backed by a Go map keyed on
	// HashKey), so there's no "insertion order" to disagree on in the
	// first place — this just confirms both end up with the same
	// membership regardless of add order (SPEC.md §2.2).
	if a.Len() != b.Len() {
		t.Fatalf("same elements added in different orders produced different sizes: %d vs %d", a.Len(), b.Len())
	}
	if !a.Has(NewInteger(1)) || !a.Has(NewInteger(2)) || !b.Has(NewInteger(1)) || !b.Has(NewInteger(2)) {
		t.Error("both sets should contain both elements regardless of add order")
	}
}
