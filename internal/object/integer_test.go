package object

import "testing"

func TestNewIntegerCachesSmallValues(t *testing.T) {
	a := NewInteger(5)
	b := NewInteger(5)
	if a != b {
		t.Errorf("NewInteger(5) returned different pointers: %p != %p — small values should be cached", a, b)
	}

	neg := NewInteger(-100)
	neg2 := NewInteger(-100)
	if neg != neg2 {
		t.Errorf("NewInteger(-100) returned different pointers — negative small values should be cached too")
	}

	lo := NewInteger(intCacheMin)
	hi := NewInteger(intCacheMax)
	if lo.Value != intCacheMin || hi.Value != intCacheMax {
		t.Errorf("cache boundary values wrong: got %d, %d", lo.Value, hi.Value)
	}
}

func TestNewIntegerOutsideCacheStillCorrect(t *testing.T) {
	big := NewInteger(int64(intCacheMax) + 1000)
	if big.Value != int64(intCacheMax)+1000 {
		t.Errorf("large value wrong: got %d", big.Value)
	}

	// Values outside the cache aren't required to share a pointer, but
	// two independently allocated Integers must still compare equal by
	// value.
	a := NewInteger(int64(intCacheMax) + 1)
	b := NewInteger(int64(intCacheMax) + 1)
	if a.Value != b.Value {
		t.Errorf("two out-of-cache Integers for the same value disagree: %d != %d", a.Value, b.Value)
	}
}

func TestIntegerHashKey(t *testing.T) {
	if NewInteger(5).HashKey() != NewInteger(5).HashKey() {
		t.Error("equal Integers must produce equal HashKeys")
	}
	if NewInteger(5).HashKey() == NewInteger(6).HashKey() {
		t.Error("different Integers must not produce equal HashKeys")
	}
}

func TestIntegerInspect(t *testing.T) {
	tests := []struct {
		value int64
		want  string
	}{
		{0, "0"},
		{42, "42"},
		{-7, "-7"},
	}
	for _, tt := range tests {
		if got := NewInteger(tt.value).Inspect(); got != tt.want {
			t.Errorf("Inspect(%d) = %q, want %q", tt.value, got, tt.want)
		}
	}
}
