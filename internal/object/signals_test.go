package object

import "testing"

func TestReturnValue(t *testing.T) {
	rv := &ReturnValue{Value: NewInteger(5)}
	if rv.Type() != RETURN_VALUE_OBJ {
		t.Errorf("Type() = %v, want %v", rv.Type(), RETURN_VALUE_OBJ)
	}
	if rv.Inspect() != "5" {
		t.Errorf("Inspect() = %q, want %q (delegates to wrapped Value)", rv.Inspect(), "5")
	}
}

func TestBreakSignal(t *testing.T) {
	if BREAK.Type() != BREAK_OBJ {
		t.Errorf("Type() = %v, want %v", BREAK.Type(), BREAK_OBJ)
	}
	if BREAK.Inspect() != "burnt" {
		t.Errorf("Inspect() = %q, want %q", BREAK.Inspect(), "burnt")
	}
}

func TestContinueSignal(t *testing.T) {
	if CONTINUE.Type() != CONTINUE_OBJ {
		t.Errorf("Type() = %v, want %v", CONTINUE.Type(), CONTINUE_OBJ)
	}
	if CONTINUE.Inspect() != "flip" {
		t.Errorf("Inspect() = %q, want %q", CONTINUE.Inspect(), "flip")
	}
}

func TestBreakAndContinueAreDistinct(t *testing.T) {
	var a Object = BREAK
	var b Object = CONTINUE
	if a == b {
		t.Error("BREAK and CONTINUE must not compare equal as Objects")
	}
	if a.Type() == b.Type() {
		t.Error("BREAK and CONTINUE must have distinct ObjectTypes")
	}
}
