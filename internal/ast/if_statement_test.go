package ast

import (
	"testing"

	"github.com/Sintfoap/cRust/internal/token"
)

func TestIfStatementBare(t *testing.T) {
	is := &IfStatement{
		Token:       token.Token{Type: token.ORDER, Literal: "order"},
		Condition:   &Identifier{Value: "cond"},
		Consequence: &BlockStatement{Token: token.Token{Literal: "{"}},
	}
	want := "order (cond) {\n}"
	if got := is.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	if got := is.TokenLiteral(); got != "order" {
		t.Errorf("TokenLiteral() = %q, want %q", got, "order")
	}
	var _ Statement = is
}

func TestIfStatementWithCombosAndAlternative(t *testing.T) {
	is := &IfStatement{
		Token:       token.Token{Literal: "order"},
		Condition:   &Identifier{Value: "a"},
		Consequence: &BlockStatement{Token: token.Token{Literal: "{"}},
		Combos: []ComboClause{
			{Condition: &Identifier{Value: "b"}, Body: &BlockStatement{Token: token.Token{Literal: "{"}}},
		},
		Alternative: &BlockStatement{Token: token.Token{Literal: "{"}},
	}
	want := "order (a) {\n} combo (b) {\n} special {\n}"
	if got := is.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestIfStatementNoAlternative(t *testing.T) {
	is := &IfStatement{
		Token:       token.Token{Literal: "order"},
		Condition:   &Identifier{Value: "a"},
		Consequence: &BlockStatement{Token: token.Token{Literal: "{"}},
	}
	if is.Alternative != nil {
		t.Fatal("Alternative should be nil by default")
	}
	want := "order (a) {\n}"
	if got := is.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
