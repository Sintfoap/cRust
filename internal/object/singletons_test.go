package object

import "testing"

func TestBooleanSingletons(t *testing.T) {
	if NativeBoolToBooleanObject(true) != TRUE {
		t.Error("NativeBoolToBooleanObject(true) should return the TRUE singleton")
	}
	if NativeBoolToBooleanObject(false) != FALSE {
		t.Error("NativeBoolToBooleanObject(false) should return the FALSE singleton")
	}
	if NativeBoolToBooleanObject(true) != NativeBoolToBooleanObject(true) {
		t.Error("repeated calls for the same value must return the same pointer")
	}
	if TRUE == FALSE {
		t.Fatal("TRUE and FALSE must not be the same object")
	}
}

func TestBooleanInspect(t *testing.T) {
	if TRUE.Inspect() != "stuffed" {
		t.Errorf("TRUE.Inspect() = %q, want %q", TRUE.Inspect(), "stuffed")
	}
	if FALSE.Inspect() != "thin" {
		t.Errorf("FALSE.Inspect() = %q, want %q", FALSE.Inspect(), "thin")
	}
}

func TestBooleanHashKey(t *testing.T) {
	if TRUE.HashKey() != TRUE.HashKey() {
		t.Error("TRUE must hash consistently")
	}
	if TRUE.HashKey() == FALSE.HashKey() {
		t.Error("TRUE and FALSE must not hash equal")
	}
}

func TestNullSingleton(t *testing.T) {
	if NULL.Inspect() != "nobox" {
		t.Errorf("NULL.Inspect() = %q, want %q", NULL.Inspect(), "nobox")
	}
	if NULL.Type() != NULL_OBJ {
		t.Errorf("NULL.Type() = %v, want %v", NULL.Type(), NULL_OBJ)
	}
}
