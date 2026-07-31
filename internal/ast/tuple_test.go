package ast

import (
	"testing"

	"github.com/Sintfoap/cRust/internal/token"
)

func TestTupleLiteral(t *testing.T) {
	tests := []struct {
		name     string
		elements []Expression
		want     string
	}{
		{"two elements", []Expression{
			&IntegerLiteral{Value: 1, Token: token.Token{Literal: "1"}},
			&IntegerLiteral{Value: 2, Token: token.Token{Literal: "2"}},
		}, "(1, 2)"},
		{"three elements", []Expression{
			&IntegerLiteral{Value: 1, Token: token.Token{Literal: "1"}},
			&IntegerLiteral{Value: 2, Token: token.Token{Literal: "2"}},
			&IntegerLiteral{Value: 3, Token: token.Token{Literal: "3"}},
		}, "(1, 2, 3)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tl := &TupleLiteral{Token: token.Token{Type: token.LPAREN, Literal: "("}, Elements: tt.elements}
			if got := tl.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
			if got := tl.TokenLiteral(); got != "(" {
				t.Errorf("TokenLiteral() = %q, want %q", got, "(")
			}
		})
	}
	var _ Expression = &TupleLiteral{}
}
