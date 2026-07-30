package ast

import (
	"fmt"

	"github.com/Sintfoap/cRust/internal/token"
)

// IntegerLiteral is an INT token (SPEC.md §2).
type IntegerLiteral struct {
	Token token.Token
	Value int64
}

func (il *IntegerLiteral) expressionNode()      {}
func (il *IntegerLiteral) TokenLiteral() string { return il.Token.Literal }
func (il *IntegerLiteral) String() string       { return il.Token.Literal }

// FloatLiteral is a FLOAT token (SPEC.md §2).
type FloatLiteral struct {
	Token token.Token
	Value float64
}

func (fl *FloatLiteral) expressionNode()      {}
func (fl *FloatLiteral) TokenLiteral() string { return fl.Token.Literal }
func (fl *FloatLiteral) String() string       { return fl.Token.Literal }

// StringLiteral is a STRING token, already escape-processed by the
// lexer (Value holds the decoded content, not the raw source text —
// see SPEC.md §2.1).
type StringLiteral struct {
	Token token.Token
	Value string
}

func (sl *StringLiteral) expressionNode()      {}
func (sl *StringLiteral) TokenLiteral() string { return sl.Token.Literal }
func (sl *StringLiteral) String() string       { return fmt.Sprintf("%q", sl.Value) }

// BooleanLiteral is `stuffed` or `thin` (SPEC.md §4) — cRust has no
// bare true/false.
type BooleanLiteral struct {
	Token token.Token // STUFFED or THIN
	Value bool
}

func (bl *BooleanLiteral) expressionNode()      {}
func (bl *BooleanLiteral) TokenLiteral() string { return bl.Token.Literal }
func (bl *BooleanLiteral) String() string       { return bl.Token.Literal }

// NilLiteral is `nobox` (SPEC.md §4).
type NilLiteral struct {
	Token token.Token // NOBOX
}

func (nl *NilLiteral) expressionNode()      {}
func (nl *NilLiteral) TokenLiteral() string { return nl.Token.Literal }
func (nl *NilLiteral) String() string       { return "nobox" }
