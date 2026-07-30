package object

import "testing"

func TestFloatInspect(t *testing.T) {
	tests := []struct {
		value float64
		want  string
	}{
		{3.14, "3.14"},
		{-0.5, "-0.5"},
		{0, "0"},
	}
	for _, tt := range tests {
		if got := (&Float{Value: tt.value}).Inspect(); got != tt.want {
			t.Errorf("Inspect(%v) = %q, want %q", tt.value, got, tt.want)
		}
	}
}

func TestFloatHashKey(t *testing.T) {
	if (&Float{Value: 1.5}).HashKey() != (&Float{Value: 1.5}).HashKey() {
		t.Error("equal Floats must produce equal HashKeys")
	}
	if (&Float{Value: 1.5}).HashKey() == (&Float{Value: 2.5}).HashKey() {
		t.Error("different Floats must not produce equal HashKeys")
	}
}
