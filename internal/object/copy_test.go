package object

import "testing"

func TestDeepCopyScalarsPassThroughUnchanged(t *testing.T) {
	i := NewInteger(5)
	if DeepCopy(i) != Object(i) {
		t.Error("DeepCopy(Integer) should return the same object, nothing to copy")
	}
	s := &String{Value: "x"}
	if DeepCopy(s) != Object(s) {
		t.Error("DeepCopy(String) should return the same object, nothing to copy")
	}
	if DeepCopy(NULL) != Object(NULL) {
		t.Error("DeepCopy(NULL) should return the same object")
	}
	if DeepCopy(TRUE) != Object(TRUE) {
		t.Error("DeepCopy(TRUE) should return the same object")
	}
}

func TestDeepCopyListIsIndependent(t *testing.T) {
	orig := NewList([]Object{NewInteger(1), NewInteger(2)})
	cp := DeepCopy(orig).(*List)
	if cp == orig {
		t.Fatal("DeepCopy(List) returned the same pointer, want a new List")
	}
	cp.Elements = append(cp.Elements, NewInteger(3))
	if len(orig.Elements) != 2 {
		t.Errorf("mutating the copy changed the original: len(orig.Elements) = %d, want 2", len(orig.Elements))
	}
}

func TestDeepCopyNestedListIsIndependent(t *testing.T) {
	inner := NewList([]Object{NewInteger(1)})
	orig := NewList([]Object{inner})
	cp := DeepCopy(orig).(*List)
	innerCp := cp.Elements[0].(*List)
	if innerCp == inner {
		t.Fatal("DeepCopy did not duplicate the nested List, want a distinct copy")
	}
	innerCp.Elements = append(innerCp.Elements, NewInteger(2))
	if len(inner.Elements) != 1 {
		t.Errorf("mutating the nested copy changed the original: len(inner.Elements) = %d, want 1", len(inner.Elements))
	}
}

func TestDeepCopyMapValueIsIndependent(t *testing.T) {
	inner := NewList([]Object{NewInteger(1)})
	orig := NewMap()
	orig.Set(&String{Value: "k"}, inner)
	cp := DeepCopy(orig).(*Map)
	if cp == orig {
		t.Fatal("DeepCopy(Map) returned the same pointer, want a new Map")
	}
	got, _ := cp.Get(&String{Value: "k"})
	innerCp := got.(*List)
	innerCp.Elements = append(innerCp.Elements, NewInteger(2))
	if len(inner.Elements) != 1 {
		t.Errorf("mutating the copy's value changed the original: len(inner.Elements) = %d, want 1", len(inner.Elements))
	}
}

func TestDeepCopyGridCellIsIndependent(t *testing.T) {
	inner := NewList([]Object{NewInteger(1)})
	orig := &Grid{Rows: [][]Object{{inner}}, RowOffset: 2, ColOffset: 3}
	cp := DeepCopy(orig).(*Grid)
	if cp == orig {
		t.Fatal("DeepCopy(Grid) returned the same pointer, want a new Grid")
	}
	if cp.RowOffset != 2 || cp.ColOffset != 3 {
		t.Errorf("DeepCopy(Grid) offsets = (%d, %d), want (2, 3)", cp.RowOffset, cp.ColOffset)
	}
	innerCp := cp.Rows[0][0].(*List)
	innerCp.Elements = append(innerCp.Elements, NewInteger(2))
	if len(inner.Elements) != 1 {
		t.Errorf("mutating the copy's cell changed the original: len(inner.Elements) = %d, want 1", len(inner.Elements))
	}
}

func TestDeepCopySetIsNewObjectWithSameMembers(t *testing.T) {
	orig := NewSet()
	orig.Add(NewInteger(1))
	orig.Add(NewInteger(2))
	cp := DeepCopy(orig).(*Set)
	if cp == orig {
		t.Fatal("DeepCopy(Set) returned the same pointer, want a new Set")
	}
	if !Equal(orig, cp) {
		t.Error("DeepCopy(Set) should have the same members as the original")
	}
}

func TestDeepCopyTupleReturnsSameObject(t *testing.T) {
	// Tuple elements must be Hashable, which rules out any mutable
	// container living inside one -- so a Tuple is already as
	// independent as a copy could make it, and DeepCopy leaves it as-is.
	orig := NewTuple([]Object{NewInteger(1), NewInteger(2)})
	if DeepCopy(orig) != Object(orig) {
		t.Error("DeepCopy(Tuple) should return the same object, nothing to copy")
	}
}

func TestDeepCopyCyclicListDoesNotHang(t *testing.T) {
	xs := NewList([]Object{NewInteger(1)})
	xs.Elements = append(xs.Elements, xs)

	cp := DeepCopy(xs).(*List)
	if cp == xs {
		t.Fatal("DeepCopy(cyclic List) returned the same pointer, want a new List")
	}
	if len(cp.Elements) != 2 {
		t.Fatalf("len(cp.Elements) = %d, want 2", len(cp.Elements))
	}
	if cp.Elements[1].(*List) != cp {
		t.Error("self-reference in the copy should point back at the copy itself, not the original")
	}
}

func TestDeepCopyPreservesSharedSubstructure(t *testing.T) {
	shared := NewList([]Object{NewInteger(1)})
	orig := NewList([]Object{shared, shared})

	cp := DeepCopy(orig).(*List)
	a := cp.Elements[0].(*List)
	b := cp.Elements[1].(*List)
	if a != b {
		t.Error("two references to the same original List should copy to the same new List, want shared substructure preserved")
	}
}
