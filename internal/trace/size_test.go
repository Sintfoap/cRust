package trace

import (
	"testing"

	"github.com/Sintfoap/cRust/internal/object"
)

func TestSizeOf(t *testing.T) {
	tests := []struct {
		name string
		v    object.Object
		want int
		ok   bool
	}{
		{"string", &object.String{Value: "hello"}, 5, true},
		{"string counts runes not bytes", &object.String{Value: "héllo"}, 5, true},
		{"list", object.NewList([]object.Object{object.NewInteger(1), object.NewInteger(2)}), 2, true},
		{"empty list", object.NewList(nil), 0, true},
		{"tuple", object.NewTuple([]object.Object{object.NewInteger(1)}), 1, true},
		{"integer has no size", object.NewInteger(42), 0, false},
		{"float has no size", &object.Float{Value: 1.5}, 0, false},
		{"boolean has no size", object.TRUE, 0, false},
		{"null has no size", object.NULL, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size, ok := SizeOf(tt.v)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if ok && size != tt.want {
				t.Errorf("size = %d, want %d", size, tt.want)
			}
		})
	}
}

func TestSizeOfSet(t *testing.T) {
	set := object.NewSet()
	set.Add(object.NewInteger(1))
	set.Add(object.NewInteger(2))
	size, ok := SizeOf(set)
	if !ok || size != 2 {
		t.Errorf("SizeOf(set) = %d, %v, want 2, true", size, ok)
	}
}

func TestSizeOfMap(t *testing.T) {
	m := object.NewMap()
	m.Set(&object.String{Value: "a"}, object.NewInteger(1))
	size, ok := SizeOf(m)
	if !ok || size != 1 {
		t.Errorf("SizeOf(map) = %d, %v, want 1, true", size, ok)
	}
}
