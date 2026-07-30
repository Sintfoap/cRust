package object

import "testing"

func TestError(t *testing.T) {
	err := &Error{Message: "division by zero", Line: 4, Col: 9}

	if err.Type() != ERROR_OBJ {
		t.Errorf("Type() = %v, want %v", err.Type(), ERROR_OBJ)
	}
	if want := "Error: division by zero"; err.Inspect() != want {
		t.Errorf("Inspect() = %q, want %q", err.Inspect(), want)
	}
}
