package ast

import (
	"testing"

	"github.com/Sintfoap/cRust/internal/token"
)

func TestTernaryExpression(t *testing.T) {
	te := &TernaryExpression{
		Token: token.Token{Type: token.TERN_THEN, Literal: "(|"},
		Cond:  &Identifier{Value: "cond"},
		Then:  &Identifier{Value: "a"},
		Else:  &Identifier{Value: "b"},
	}
	want := "(cond (| a |) b)"
	if got := te.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	if got := te.TokenLiteral(); got != "(|" {
		t.Errorf("TokenLiteral() = %q, want %q", got, "(|")
	}
	var _ Expression = te
}

func TestElvisExpression(t *testing.T) {
	ee := &ElvisExpression{
		Token: token.Token{Type: token.ELVIS, Literal: "?:"},
		Left:  &Identifier{Value: "a"},
		Right: &Identifier{Value: "b"},
	}
	want := "(a ?: b)"
	if got := ee.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	if got := ee.TokenLiteral(); got != "?:" {
		t.Errorf("TokenLiteral() = %q, want %q", got, "?:")
	}
	var _ Expression = ee
}
