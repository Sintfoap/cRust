package ast

import (
	"testing"

	"github.com/Sintfoap/cRust/internal/token"
)

func TestCountedLoopAllClausesPresent(t *testing.T) {
	cl := &CountedLoop{
		Token: token.Token{Type: token.KNEAD, Literal: "knead"},
		Init: &AssignStatement{
			Target: &Identifier{Value: "i"}, Operator: "=", Value: &IntegerLiteral{Value: 0, Token: token.Token{Literal: "0"}},
		},
		Cond: &InfixExpression{Left: &Identifier{Value: "i"}, Operator: "<", Right: &IntegerLiteral{Value: 10, Token: token.Token{Literal: "10"}}},
		Post: &IncDecStatement{Target: &Identifier{Value: "i"}, Operator: "++"},
		Body: &BlockStatement{Token: token.Token{Literal: "{"}},
	}
	want := "knead (i = 0; (i < 10); i++) {\n}"
	if got := cl.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	if got := cl.TokenLiteral(); got != "knead" {
		t.Errorf("TokenLiteral() = %q, want %q", got, "knead")
	}
	var _ Statement = cl
}

func TestCountedLoopAllClausesEmpty(t *testing.T) {
	cl := &CountedLoop{
		Token: token.Token{Literal: "knead"},
		Body:  &BlockStatement{Token: token.Token{Literal: "{"}},
	}
	want := "knead (; ; ) {\n}"
	if got := cl.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestForEachLoop(t *testing.T) {
	fel := &ForEachLoop{
		Token:      token.Token{Type: token.KNEAD, Literal: "knead"},
		Identifier: &Identifier{Value: "item"},
		Collection: &Identifier{Value: "xs"},
		Body:       &BlockStatement{Token: token.Token{Literal: "{"}},
	}
	want := "knead item in xs {\n}"
	if got := fel.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	if got := fel.TokenLiteral(); got != "knead" {
		t.Errorf("TokenLiteral() = %q, want %q", got, "knead")
	}
	var _ Statement = fel
}

func TestBakeStatement(t *testing.T) {
	bs := &BakeStatement{
		Token:     token.Token{Type: token.BAKE, Literal: "bake"},
		Condition: &Identifier{Value: "cond"},
		Body:      &BlockStatement{Token: token.Token{Literal: "{"}},
	}
	want := "bake (cond) {\n}"
	if got := bs.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	if got := bs.TokenLiteral(); got != "bake" {
		t.Errorf("TokenLiteral() = %q, want %q", got, "bake")
	}
	var _ Statement = bs
}
