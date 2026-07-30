package ast

import (
	"testing"

	"github.com/Sintfoap/cRust/internal/token"
)

func TestListLiteral(t *testing.T) {
	tests := []struct {
		name     string
		elements []Expression
		want     string
	}{
		{"empty", nil, "[]"},
		{"single", []Expression{&IntegerLiteral{Value: 1, Token: token.Token{Literal: "1"}}}, "[1]"},
		{
			"multiple",
			[]Expression{
				&IntegerLiteral{Value: 1, Token: token.Token{Literal: "1"}},
				&IntegerLiteral{Value: 2, Token: token.Token{Literal: "2"}},
			},
			"[1, 2]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ll := &ListLiteral{Token: token.Token{Type: token.LBRACKET, Literal: "["}, Elements: tt.elements}
			if got := ll.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
			if got := ll.TokenLiteral(); got != "[" {
				t.Errorf("TokenLiteral() = %q, want %q", got, "[")
			}
		})
	}
	var _ Expression = &ListLiteral{}
}

func TestMapLiteral(t *testing.T) {
	tests := []struct {
		name  string
		pairs []MapPair
		want  string
	}{
		{"empty", nil, "{}"},
		{
			"single pair",
			[]MapPair{{Key: &StringLiteral{Value: "a"}, Value: &IntegerLiteral{Value: 1, Token: token.Token{Literal: "1"}}}},
			`{"a": 1}`,
		},
		{
			"multiple pairs preserve order",
			[]MapPair{
				{Key: &StringLiteral{Value: "a"}, Value: &IntegerLiteral{Value: 1, Token: token.Token{Literal: "1"}}},
				{Key: &StringLiteral{Value: "b"}, Value: &IntegerLiteral{Value: 2, Token: token.Token{Literal: "2"}}},
			},
			`{"a": 1, "b": 2}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ml := &MapLiteral{Token: token.Token{Type: token.LBRACE, Literal: "{"}, Pairs: tt.pairs}
			if got := ml.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
	var _ Expression = &MapLiteral{}
}

func TestSetLiteral(t *testing.T) {
	sl := &SetLiteral{
		Token: token.Token{Type: token.TOPPINGS, Literal: "toppings"},
		Elements: []Expression{
			&IntegerLiteral{Value: 1, Token: token.Token{Literal: "1"}},
			&IntegerLiteral{Value: 2, Token: token.Token{Literal: "2"}},
		},
	}
	want := "toppings{1, 2}"
	if got := sl.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	if got := sl.TokenLiteral(); got != "toppings" {
		t.Errorf("TokenLiteral() = %q, want %q", got, "toppings")
	}

	empty := &SetLiteral{Token: token.Token{Literal: "toppings"}}
	if got := empty.String(); got != "toppings{}" {
		t.Errorf("String() = %q, want %q", got, "toppings{}")
	}
	var _ Expression = sl
}
