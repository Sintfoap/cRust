package object

import "testing"

// TestHashKeyDistinctAcrossTypes guards against two different types
// whose HashKey.Value happens to collide (e.g. Integer(1) and
// Boolean(true), which both naturally hash to Value: 1) being treated
// as the same Map/Set key. HashKey.Type is what prevents that.
func TestHashKeyDistinctAcrossTypes(t *testing.T) {
	one := NewInteger(1).HashKey()
	trueKey := TRUE.HashKey()

	if one.Value != trueKey.Value {
		t.Fatalf("test assumption broken: Integer(1) and TRUE were expected to share a Value (%d vs %d)", one.Value, trueKey.Value)
	}
	if one == trueKey {
		t.Error("Integer(1) and TRUE must not produce equal HashKeys despite sharing a Value")
	}
}

func TestHashablePrimitivesImplementHashable(t *testing.T) {
	var objs = []Object{
		NewInteger(1),
		&Float{Value: 1.5},
		&String{Value: "x"},
		TRUE,
	}
	for _, o := range objs {
		if _, ok := o.(Hashable); !ok {
			t.Errorf("%T does not implement Hashable", o)
		}
	}
}

func TestListsAndMapsAreNotHashable(t *testing.T) {
	var objs = []Object{
		NewList(nil),
		NewMap(),
		NewSet(),
	}
	for _, o := range objs {
		if _, ok := o.(Hashable); ok {
			t.Errorf("%T should not implement Hashable (it's mutable)", o)
		}
	}
}
