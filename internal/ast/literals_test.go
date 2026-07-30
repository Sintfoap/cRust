package ast

import (
	"testing"

	"github.com/Sintfoap/cRust/internal/token"
)

func TestIntegerLiteral(t *testing.T) {
	il := &IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "42"}, Value: 42}

	if got := il.TokenLiteral(); got != "42" {
		t.Errorf("TokenLiteral() = %q, want %q", got, "42")
	}
	if got := il.String(); got != "42" {
		t.Errorf("String() = %q, want %q", got, "42")
	}
	var _ Expression = il
}

func TestFloatLiteral(t *testing.T) {
	fl := &FloatLiteral{Token: token.Token{Type: token.FLOAT, Literal: "3.14"}, Value: 3.14}

	if got := fl.TokenLiteral(); got != "3.14" {
		t.Errorf("TokenLiteral() = %q, want %q", got, "3.14")
	}
	if got := fl.String(); got != "3.14" {
		t.Errorf("String() = %q, want %q", got, "3.14")
	}
	var _ Expression = fl
}

func TestStringLiteral(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"plain", "hello", `"hello"`},
		{"empty", "", `""`},
		{"embedded quote", `say "hi"`, `"say \"hi\""`},
		{"embedded newline", "a\nb", `"a\nb"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sl := &StringLiteral{Token: token.Token{Type: token.STRING, Literal: tt.value}, Value: tt.value}
			if got := sl.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
			if got := sl.TokenLiteral(); got != tt.value {
				t.Errorf("TokenLiteral() = %q, want %q", got, tt.value)
			}
		})
	}
	var _ Expression = &StringLiteral{}
}

func TestBooleanLiteral(t *testing.T) {
	tests := []struct {
		tok  token.Token
		val  bool
		want string
	}{
		{token.Token{Type: token.STUFFED, Literal: "stuffed"}, true, "stuffed"},
		{token.Token{Type: token.THIN, Literal: "thin"}, false, "thin"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			bl := &BooleanLiteral{Token: tt.tok, Value: tt.val}
			if got := bl.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
			if got := bl.TokenLiteral(); got != tt.want {
				t.Errorf("TokenLiteral() = %q, want %q", got, tt.want)
			}
		})
	}
	var _ Expression = &BooleanLiteral{}
}

func TestNilLiteral(t *testing.T) {
	nl := &NilLiteral{Token: token.Token{Type: token.NOBOX, Literal: "nobox"}}

	if got := nl.String(); got != "nobox" {
		t.Errorf("String() = %q, want %q", got, "nobox")
	}
	if got := nl.TokenLiteral(); got != "nobox" {
		t.Errorf("TokenLiteral() = %q, want %q", got, "nobox")
	}
	var _ Expression = nl
}
