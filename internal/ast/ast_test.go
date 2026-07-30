package ast

import (
	"testing"

	"github.com/Sintfoap/cRust/internal/token"
)

func TestProgramTokenLiteralEmpty(t *testing.T) {
	p := &Program{}
	if got := p.TokenLiteral(); got != "" {
		t.Errorf("TokenLiteral() = %q, want \"\"", got)
	}
}

func TestProgramTokenLiteralDelegatesToFirstStatement(t *testing.T) {
	p := &Program{
		Statements: []Statement{
			&ExpressionStatement{
				Token:      token.Token{Type: token.IDENT, Literal: "x"},
				Expression: &Identifier{Token: token.Token{Type: token.IDENT, Literal: "x"}, Value: "x"},
			},
		},
	}
	if got := p.TokenLiteral(); got != "x" {
		t.Errorf("TokenLiteral() = %q, want %q", got, "x")
	}
}

func TestProgramString(t *testing.T) {
	p := &Program{
		Statements: []Statement{
			&ExpressionStatement{Expression: &Identifier{Value: "a"}},
			&ExpressionStatement{Expression: &Identifier{Value: "b"}},
		},
	}
	want := "a\nb\n"
	if got := p.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestProgramStringEmpty(t *testing.T) {
	p := &Program{}
	if got := p.String(); got != "" {
		t.Errorf("String() = %q, want \"\"", got)
	}
}

// TestMarkerMethodsAndTokenLiteral exercises every concrete node's
// expressionNode()/statementNode() marker method directly and its
// TokenLiteral(). The markers are no-op bodies that exist purely so the
// Go compiler enforces Expression/Statement at every construction site
// (a *ListLiteral can't accidentally satisfy Statement, etc.) — nothing
// to assert beyond "doesn't panic, and TokenLiteral() reads the right
// field" for whichever ones the String()-focused tests elsewhere in
// this package don't already happen to call.
func TestMarkerMethodsAndTokenLiteral(t *testing.T) {
	tok := token.Token{Literal: "tok"}
	block := &BlockStatement{Token: tok}

	exprs := []Expression{
		&Identifier{Token: tok},
		&IntegerLiteral{Token: tok},
		&FloatLiteral{Token: tok},
		&StringLiteral{Token: tok},
		&BooleanLiteral{Token: tok},
		&NilLiteral{Token: tok},
		&ListLiteral{Token: tok},
		&MapLiteral{Token: tok},
		&SetLiteral{Token: tok},
		&FunctionLiteral{Token: tok, Body: block},
		&PrefixExpression{Token: tok, Right: &Identifier{Value: "x"}},
		&InfixExpression{Token: tok, Left: &Identifier{Value: "x"}, Right: &Identifier{Value: "y"}},
		&RangeExpression{Token: tok, Start: &IntegerLiteral{Token: token.Token{Literal: "0"}}, End: &IntegerLiteral{Token: token.Token{Literal: "1"}}},
		&CallExpression{Token: tok, Function: &Identifier{Value: "f"}},
		&IndexExpression{Token: tok, Left: &Identifier{Value: "x"}, Index: &IntegerLiteral{Token: token.Token{Literal: "0"}}},
		&TernaryExpression{Token: tok, Cond: &Identifier{Value: "c"}, Then: &Identifier{Value: "t"}, Else: &Identifier{Value: "e"}},
		&ElvisExpression{Token: tok, Left: &Identifier{Value: "a"}, Right: &Identifier{Value: "b"}},
	}
	for _, e := range exprs {
		e.expressionNode()
		if got := e.TokenLiteral(); got != "tok" {
			t.Errorf("%T.TokenLiteral() = %q, want %q", e, got, "tok")
		}
	}

	stmts := []Statement{
		block,
		&ExpressionStatement{Token: tok},
		&AssignStatement{Token: tok, Target: &Identifier{Value: "x"}, Value: &IntegerLiteral{Token: token.Token{Literal: "0"}}},
		&UnpackAssignStatement{Token: tok, Targets: []*Identifier{{Value: "a"}}, Value: &Identifier{Value: "xs"}},
		&IncDecStatement{Token: tok, Target: &Identifier{Value: "x"}},
		&ReturnStatement{Token: tok},
		&BurntStatement{Token: tok},
		&FlipStatement{Token: tok},
		&IfStatement{Token: tok, Condition: &Identifier{Value: "c"}, Consequence: block},
		&CountedLoop{Token: tok, Body: block},
		&ForEachLoop{Token: tok, Identifier: &Identifier{Value: "i"}, Collection: &Identifier{Value: "xs"}, Body: block},
		&BakeStatement{Token: tok, Condition: &Identifier{Value: "c"}, Body: block},
	}
	for _, s := range stmts {
		s.statementNode()
		if got := s.TokenLiteral(); got != "tok" {
			t.Errorf("%T.TokenLiteral() = %q, want %q", s, got, "tok")
		}
	}
}
