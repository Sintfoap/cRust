package object

import "testing"

func TestIsTruthyNullIsFalse(t *testing.T) {
	if IsTruthy(NULL) {
		t.Error("IsTruthy(NULL) = true, want false")
	}
}

func TestIsTruthyFalseIsFalse(t *testing.T) {
	if IsTruthy(FALSE) {
		t.Error("IsTruthy(FALSE) = true, want false")
	}
}

func TestIsTruthyTrueIsTrue(t *testing.T) {
	if !IsTruthy(TRUE) {
		t.Error("IsTruthy(TRUE) = false, want true")
	}
}

func TestIsTruthyZeroIsTrue(t *testing.T) {
	if !IsTruthy(NewInteger(0)) {
		t.Error("IsTruthy(0) = false, want true (only thin/nobox are falsy)")
	}
}

func TestIsTruthyEmptyStringIsTrue(t *testing.T) {
	if !IsTruthy(&String{Value: ""}) {
		t.Error("IsTruthy(\"\") = false, want true")
	}
}

func TestIsTruthyEmptyListIsTrue(t *testing.T) {
	if !IsTruthy(NewList(nil)) {
		t.Error("IsTruthy([]) = false, want true")
	}
}

func TestIsTruthyEmptySetIsTrue(t *testing.T) {
	if !IsTruthy(NewSet()) {
		t.Error("IsTruthy(toppings{}) = false, want true")
	}
}
