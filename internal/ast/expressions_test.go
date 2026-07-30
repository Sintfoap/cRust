package ast

import (
	"testing"

	"github.com/Sintfoap/cRust/internal/token"
)

func TestPrefixExpression(t *testing.T) {
	tests := []struct {
		op   string
		want string
	}{
		{"-", "(- x)"},
		{"hold", "(hold x)"},
	}

	for _, tt := range tests {
		t.Run(tt.op, func(t *testing.T) {
			pe := &PrefixExpression{
				Token:    token.Token{Literal: tt.op},
				Operator: tt.op,
				Right:    &Identifier{Value: "x"},
			}
			if got := pe.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
			if got := pe.TokenLiteral(); got != tt.op {
				t.Errorf("TokenLiteral() = %q, want %q", got, tt.op)
			}
		})
	}
	var _ Expression = &PrefixExpression{}
}

func TestInfixExpression(t *testing.T) {
	ie := &InfixExpression{
		Token:    token.Token{Literal: "+"},
		Left:     &Identifier{Value: "a"},
		Operator: "+",
		Right:    &Identifier{Value: "b"},
	}
	want := "(a + b)"
	if got := ie.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	var _ Expression = ie
}

func TestRangeExpression(t *testing.T) {
	tests := []struct {
		name      string
		inclusive bool
		want      string
	}{
		{"inclusive", true, "(1..5)"},
		{"exclusive", false, "(1.<5)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			re := &RangeExpression{
				Token:     token.Token{Literal: ".."},
				Start:     &IntegerLiteral{Value: 1, Token: token.Token{Literal: "1"}},
				End:       &IntegerLiteral{Value: 5, Token: token.Token{Literal: "5"}},
				Inclusive: tt.inclusive,
			}
			if got := re.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
	var _ Expression = &RangeExpression{}
}

func TestCallExpression(t *testing.T) {
	tests := []struct {
		name string
		args []Expression
		want string
	}{
		{"no args", nil, "f()"},
		{"one arg", []Expression{&Identifier{Value: "x"}}, "f(x)"},
		{"multiple args", []Expression{&Identifier{Value: "x"}, &Identifier{Value: "y"}}, "f(x, y)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ce := &CallExpression{
				Token:     token.Token{Literal: "("},
				Function:  &Identifier{Value: "f"},
				Arguments: tt.args,
			}
			if got := ce.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
	var _ Expression = &CallExpression{}
}

func TestIndexExpression(t *testing.T) {
	ie := &IndexExpression{
		Token: token.Token{Literal: "["},
		Left:  &Identifier{Value: "arr"},
		Index: &IntegerLiteral{Value: 0, Token: token.Token{Literal: "0"}},
	}
	want := "(arr[0])"
	if got := ie.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	var _ Expression = ie
}
