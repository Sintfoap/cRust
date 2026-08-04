package ast

import (
	"testing"

	"github.com/Sintfoap/cRust/internal/token"
)

func TestBlockStatement(t *testing.T) {
	tests := []struct {
		name  string
		stmts []Statement
		want  string
	}{
		{"empty", nil, "{\n}"},
		{
			"single",
			[]Statement{&ExpressionStatement{Expression: &Identifier{Value: "x"}}},
			"{\nx\n}",
		},
		{
			"multiple",
			[]Statement{
				&ExpressionStatement{Expression: &Identifier{Value: "x"}},
				&ExpressionStatement{Expression: &Identifier{Value: "y"}},
			},
			"{\nx\ny\n}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bs := &BlockStatement{Token: token.Token{Literal: "{"}, Statements: tt.stmts}
			if got := bs.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
	var _ Statement = &BlockStatement{}
}

func TestExpressionStatement(t *testing.T) {
	es := &ExpressionStatement{Token: token.Token{Literal: "x"}, Expression: &Identifier{Value: "x"}}
	if got := es.String(); got != "x" {
		t.Errorf("String() = %q, want %q", got, "x")
	}

	// Guards against the typed-nil-in-interface footgun: a nil
	// Expression must not panic String(), since ExpressionStatement can
	// be partially constructed by a parser function that bailed early.
	empty := &ExpressionStatement{}
	if got := empty.String(); got != "" {
		t.Errorf("String() with nil Expression = %q, want \"\"", got)
	}
	var _ Statement = es
}

func TestAssignStatement(t *testing.T) {
	as := &AssignStatement{
		Token:    token.Token{Literal: "="},
		Target:   &Identifier{Value: "x"},
		Operator: "=",
		Value:    &IntegerLiteral{Value: 5, Token: token.Token{Literal: "5"}},
	}
	want := "x = 5"
	if got := as.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	var _ Statement = as
}

func TestUnpackAssignStatement(t *testing.T) {
	uas := &UnpackAssignStatement{
		Token:   token.Token{Literal: "a"},
		Targets: []*Identifier{{Value: "a"}, {Value: "b"}, {Value: "c"}},
		Value:   &Identifier{Value: "xs"},
	}
	want := "a, b, c = xs"
	if got := uas.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	var _ Statement = uas
}

func TestIncDecStatement(t *testing.T) {
	tests := []struct {
		op   string
		want string
	}{
		{"++", "x++"},
		{"--", "x--"},
	}

	for _, tt := range tests {
		t.Run(tt.op, func(t *testing.T) {
			ids := &IncDecStatement{Token: token.Token{Literal: tt.op}, Target: &Identifier{Value: "x"}, Operator: tt.op}
			if got := ids.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
	var _ Statement = &IncDecStatement{}
}

func TestReturnStatement(t *testing.T) {
	rs := &ReturnStatement{Token: token.Token{Literal: "serve"}, ReturnValue: &IntegerLiteral{Value: 5, Token: token.Token{Literal: "5"}}}
	if got := rs.String(); got != "serve 5" {
		t.Errorf("String() = %q, want %q", got, "serve 5")
	}

	bare := &ReturnStatement{Token: token.Token{Literal: "serve"}}
	if got := bare.String(); got != "serve" {
		t.Errorf("String() with nil ReturnValue = %q, want %q", got, "serve")
	}
	var _ Statement = rs
}

func TestBurntStatement(t *testing.T) {
	bs := &BurntStatement{Token: token.Token{Literal: "burnt"}}
	if got := bs.String(); got != "burnt" {
		t.Errorf("String() = %q, want %q", got, "burnt")
	}
	if got := bs.TokenLiteral(); got != "burnt" {
		t.Errorf("TokenLiteral() = %q, want %q", got, "burnt")
	}
	var _ Statement = bs
}

// TestStatementPosReturnsOwnToken spot-checks that Pos() (added to the
// Statement interface so the debugger recorder can attach a line
// number to any statement kind without a type switch) returns the
// statement's own token, not a zero value or another field.
func TestStatementPosReturnsOwnToken(t *testing.T) {
	tok := token.Token{Line: 7, Col: 3}
	tests := []struct {
		name string
		stmt Statement
	}{
		{"BlockStatement", &BlockStatement{Token: tok}},
		{"ExpressionStatement", &ExpressionStatement{Token: tok}},
		{"AssignStatement", &AssignStatement{Token: tok}},
		{"UnpackAssignStatement", &UnpackAssignStatement{Token: tok}},
		{"IncDecStatement", &IncDecStatement{Token: tok}},
		{"ReturnStatement", &ReturnStatement{Token: tok}},
		{"BurntStatement", &BurntStatement{Token: tok}},
		{"FlipStatement", &FlipStatement{Token: tok}},
		{"CountedLoop", &CountedLoop{Token: tok}},
		{"ForEachLoop", &ForEachLoop{Token: tok}},
		{"BakeStatement", &BakeStatement{Token: tok}},
		{"IfStatement", &IfStatement{Token: tok}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.stmt.Pos(); got != tok {
				t.Errorf("Pos() = %+v, want %+v", got, tok)
			}
		})
	}
}

func TestFlipStatement(t *testing.T) {
	fs := &FlipStatement{Token: token.Token{Literal: "flip"}}
	if got := fs.String(); got != "flip" {
		t.Errorf("String() = %q, want %q", got, "flip")
	}
	if got := fs.TokenLiteral(); got != "flip" {
		t.Errorf("TokenLiteral() = %q, want %q", got, "flip")
	}
	var _ Statement = fs
}
