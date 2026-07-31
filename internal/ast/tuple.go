package ast

import (
	"strings"

	"github.com/Sintfoap/cRust/internal/token"
)

// TupleLiteral is `(a, b, ...)` (SPEC.md §2) — a fixed-size,
// immutable, heterogeneous value, distinct from List specifically so
// it can be hashed (used as a Map key or Set element — SPEC.md §7's
// Set builtins, most usefully for grid-coordinate dedup). Written with
// the same '(' token ordinary grouping uses; the parser tells the two
// apart by whether a ',' follows the first inner expression — see
// parseGroupedExpression in internal/parser/expressions.go.
type TupleLiteral struct {
	Token    token.Token // '('
	Elements []Expression
}

func (tl *TupleLiteral) expressionNode()      {}
func (tl *TupleLiteral) TokenLiteral() string { return tl.Token.Literal }
func (tl *TupleLiteral) String() string {
	parts := make([]string, len(tl.Elements))
	for i, e := range tl.Elements {
		parts[i] = e.String()
	}
	return "(" + strings.Join(parts, ", ") + ")"
}
