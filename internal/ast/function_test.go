package ast

import (
	"testing"

	"github.com/Sintfoap/cRust/internal/token"
)

func TestFunctionLiteralNamed(t *testing.T) {
	fl := &FunctionLiteral{
		Token: token.Token{Type: token.RECIPE, Literal: "recipe"},
		Name:  &Identifier{Value: "add"},
		Parameters: []*Identifier{
			{Value: "x"},
			{Value: "y"},
		},
		Body: &BlockStatement{Token: token.Token{Literal: "{"}},
	}

	want := "recipe add(x, y) {\n}"
	if got := fl.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	if got := fl.TokenLiteral(); got != "recipe" {
		t.Errorf("TokenLiteral() = %q, want %q", got, "recipe")
	}
	var _ Expression = fl
}

func TestFunctionLiteralAnonymous(t *testing.T) {
	fl := &FunctionLiteral{
		Token:      token.Token{Literal: "recipe"},
		Name:       nil,
		Parameters: nil,
		Body:       &BlockStatement{Token: token.Token{Literal: "{"}},
	}

	want := "recipe () {\n}"
	if got := fl.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
