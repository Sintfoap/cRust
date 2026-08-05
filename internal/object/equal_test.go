package object

import "testing"

func TestEqualIntegers(t *testing.T) {
	if !Equal(NewInteger(1), NewInteger(1)) {
		t.Error("Equal(1, 1) = false, want true")
	}
	if Equal(NewInteger(1), NewInteger(2)) {
		t.Error("Equal(1, 2) = true, want false")
	}
}

func TestEqualIntegerAndFloat(t *testing.T) {
	if !Equal(NewInteger(1), &Float{Value: 1.0}) {
		t.Error("Equal(1, 1.0) = false, want true (Int/Float are one number category)")
	}
}

func TestEqualFloats(t *testing.T) {
	if !Equal(&Float{Value: 1.5}, &Float{Value: 1.5}) {
		t.Error("Equal(1.5, 1.5) = false, want true")
	}
	if Equal(&Float{Value: 1.5}, &Float{Value: 2.5}) {
		t.Error("Equal(1.5, 2.5) = true, want false")
	}
}

func TestEqualStrings(t *testing.T) {
	if !Equal(&String{Value: "a"}, &String{Value: "a"}) {
		t.Error("Equal(\"a\", \"a\") = false, want true")
	}
	if Equal(&String{Value: "a"}, &String{Value: "b"}) {
		t.Error("Equal(\"a\", \"b\") = true, want false")
	}
}

func TestEqualBooleans(t *testing.T) {
	if !Equal(TRUE, TRUE) {
		t.Error("Equal(TRUE, TRUE) = false, want true")
	}
	if Equal(TRUE, FALSE) {
		t.Error("Equal(TRUE, FALSE) = true, want false")
	}
}

func TestEqualNulls(t *testing.T) {
	if !Equal(NULL, NULL) {
		t.Error("Equal(NULL, NULL) = false, want true")
	}
}

func TestEqualCrossTypeAlwaysFalse(t *testing.T) {
	if Equal(NewInteger(1), &String{Value: "1"}) {
		t.Error("Equal(1, \"1\") = true, want false (cross-category)")
	}
}

func TestEqualLists(t *testing.T) {
	a := NewList([]Object{NewInteger(1), NewInteger(2)})
	b := NewList([]Object{NewInteger(1), NewInteger(2)})
	c := NewList([]Object{NewInteger(1), NewInteger(3)})
	if !Equal(a, b) {
		t.Error("Equal(a, b) = false, want true (same contents)")
	}
	if Equal(a, c) {
		t.Error("Equal(a, c) = true, want false (different contents)")
	}
}

func TestEqualListsDifferentLength(t *testing.T) {
	a := NewList([]Object{NewInteger(1)})
	b := NewList([]Object{NewInteger(1), NewInteger(2)})
	if Equal(a, b) {
		t.Error("Equal(a, b) = true, want false (different lengths)")
	}
}

func TestEqualTuples(t *testing.T) {
	a := NewTuple([]Object{NewInteger(1), NewInteger(2)})
	b := NewTuple([]Object{NewInteger(1), NewInteger(2)})
	if !Equal(a, b) {
		t.Error("Equal(a, b) = false, want true")
	}
}

func TestEqualMaps(t *testing.T) {
	a := NewMap()
	a.Set(&String{Value: "k"}, NewInteger(1))
	b := NewMap()
	b.Set(&String{Value: "k"}, NewInteger(1))
	if !Equal(a, b) {
		t.Error("Equal(a, b) = false, want true (same key/value)")
	}
}

func TestEqualMapsDifferentSize(t *testing.T) {
	a := NewMap()
	a.Set(&String{Value: "k"}, NewInteger(1))
	b := NewMap()
	if Equal(a, b) {
		t.Error("Equal(a, b) = true, want false (different sizes)")
	}
}

func TestEqualMapsDifferentValue(t *testing.T) {
	a := NewMap()
	a.Set(&String{Value: "k"}, NewInteger(1))
	b := NewMap()
	b.Set(&String{Value: "k"}, NewInteger(2))
	if Equal(a, b) {
		t.Error("Equal(a, b) = true, want false (different values)")
	}
}

func TestEqualSets(t *testing.T) {
	a := NewSet()
	a.Add(NewInteger(1))
	a.Add(NewInteger(2))
	b := NewSet()
	b.Add(NewInteger(2))
	b.Add(NewInteger(1))
	if !Equal(a, b) {
		t.Error("Equal(a, b) = false, want true (same members, any order)")
	}
}

func TestEqualSetsDifferentSize(t *testing.T) {
	a := NewSet()
	a.Add(NewInteger(1))
	b := NewSet()
	if Equal(a, b) {
		t.Error("Equal(a, b) = true, want false (different sizes)")
	}
}

func TestEqualSetsSameSizeDifferentMembers(t *testing.T) {
	a := NewSet()
	a.Add(NewInteger(1))
	b := NewSet()
	b.Add(NewInteger(2))
	if Equal(a, b) {
		t.Error("Equal(a, b) = true, want false (same size, different members)")
	}
}

func TestEqualGrids(t *testing.T) {
	a := &Grid{Rows: [][]Object{{&String{Value: "x"}}}}
	b := &Grid{Rows: [][]Object{{&String{Value: "x"}}}}
	if !Equal(a, b) {
		t.Error("Equal(a, b) = false, want true (same offset and cells)")
	}
}

func TestEqualGridsDifferentOffset(t *testing.T) {
	a := &Grid{Rows: [][]Object{{&String{Value: "x"}}}, RowOffset: 0}
	b := &Grid{Rows: [][]Object{{&String{Value: "x"}}}, RowOffset: 1}
	if Equal(a, b) {
		t.Error("Equal(a, b) = true, want false (different offset means different logical layout)")
	}
}

func TestEqualGridsDifferentRowCount(t *testing.T) {
	a := &Grid{Rows: [][]Object{{&String{Value: "x"}}}}
	b := &Grid{Rows: [][]Object{{&String{Value: "x"}}, {&String{Value: "y"}}}}
	if Equal(a, b) {
		t.Error("Equal(a, b) = true, want false (different row counts)")
	}
}

func TestEqualGridsDifferentColOffset(t *testing.T) {
	a := &Grid{Rows: [][]Object{{&String{Value: "x"}}}, ColOffset: 0}
	b := &Grid{Rows: [][]Object{{&String{Value: "x"}}}, ColOffset: 1}
	if Equal(a, b) {
		t.Error("Equal(a, b) = true, want false (different column offset)")
	}
}

func TestEqualGridsSameRowCountDifferentContent(t *testing.T) {
	a := &Grid{Rows: [][]Object{{&String{Value: "x"}}}}
	b := &Grid{Rows: [][]Object{{&String{Value: "y"}}}}
	if Equal(a, b) {
		t.Error("Equal(a, b) = true, want false (same shape, different cell content)")
	}
}

func TestEqualUnhandledTypeFallsThroughToFalse(t *testing.T) {
	// Error isn't given its own case in Equal's switch (equality
	// between two runtime errors isn't a meaningful question a cRust
	// program can even ask, since an Error short-circuits evaluation
	// before == would run) -- exercises the default: return false arm.
	a := &Error{Message: "boom"}
	b := &Error{Message: "boom"}
	if Equal(a, b) {
		t.Error("Equal(a, b) = true, want false (unhandled type defaults to not-equal)")
	}
}

func TestEqualFunctionsByIdentity(t *testing.T) {
	f := &Function{}
	g := &Function{}
	if !Equal(f, f) {
		t.Error("Equal(f, f) = false, want true (same identity)")
	}
	if Equal(f, g) {
		t.Error("Equal(f, g) = true, want false (different identity, both empty)")
	}
}
