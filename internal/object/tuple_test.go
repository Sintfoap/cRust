package object

import "testing"

func TestTupleType(t *testing.T) {
	tup := NewTuple([]Object{NewInteger(1), NewInteger(2)})
	if tup.Type() != TUPLE_OBJ {
		t.Errorf("Type() = %v, want %v", tup.Type(), TUPLE_OBJ)
	}
}

func TestTupleInspect(t *testing.T) {
	tup := NewTuple([]Object{NewInteger(1), &String{Value: "hi"}})
	want := `(1, hi)`
	if got := tup.Inspect(); got != want {
		t.Errorf("Inspect() = %q, want %q", got, want)
	}
}

func TestEmptyTupleInspect(t *testing.T) {
	tup := NewTuple(nil)
	if got := tup.Inspect(); got != "()" {
		t.Errorf("Inspect() = %q, want %q", got, "()")
	}
}

func TestTupleIsAReferenceType(t *testing.T) {
	original := NewTuple([]Object{NewInteger(1), NewInteger(2)})
	alias := original
	if alias != original {
		t.Fatal("assigning a *Tuple should share the same pointer")
	}
}

func TestTupleHashKeyEqualForEqualContents(t *testing.T) {
	a := NewTuple([]Object{NewInteger(1), &String{Value: "x"}})
	b := NewTuple([]Object{NewInteger(1), &String{Value: "x"}})
	if a.HashKey() != b.HashKey() {
		t.Errorf("HashKey() differs for equal-content Tuples: %v vs %v", a.HashKey(), b.HashKey())
	}
}

func TestTupleHashKeyDiffersForDifferentContents(t *testing.T) {
	a := NewTuple([]Object{NewInteger(1), NewInteger(2)})
	b := NewTuple([]Object{NewInteger(2), NewInteger(1)}) // same elements, different order
	if a.HashKey() == b.HashKey() {
		t.Error("HashKey() should differ when element order differs")
	}
}

func TestTupleHashKeyDiffersByArity(t *testing.T) {
	a := NewTuple([]Object{NewInteger(1)})
	b := NewTuple([]Object{NewInteger(1), NewInteger(1)})
	if a.HashKey() == b.HashKey() {
		t.Error("HashKey() should differ for different-length Tuples")
	}
}

func TestTupleHashKeyDistinguishesTypeFromValue(t *testing.T) {
	// A Tuple containing TRUE (Boolean HashKey.Value = 1) shouldn't
	// hash the same as a Tuple containing Integer(1), even though
	// Boolean and Integer HashKeys can share a raw Value — HashKey
	// itself carries Type precisely to prevent this kind of collision,
	// and Tuple's combination has to preserve that guarantee.
	a := NewTuple([]Object{TRUE})
	b := NewTuple([]Object{NewInteger(1)})
	if a.HashKey() == b.HashKey() {
		t.Error("HashKey() conflated a Boolean element with an Integer element")
	}
}

func TestTupleImplementsHashable(t *testing.T) {
	var _ Hashable = NewTuple(nil)
}
