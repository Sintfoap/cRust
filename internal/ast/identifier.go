package ast

import "github.com/Sintfoap/cRust/internal/token"

// Identifier is a bare name — a variable reference, an lvalue, a
// recipe/parameter name.
type Identifier struct {
	Token token.Token // the IDENT token
	Value string
}

func (i *Identifier) expressionNode()      {}
func (i *Identifier) TokenLiteral() string { return i.Token.Literal }
func (i *Identifier) String() string       { return i.Value }
