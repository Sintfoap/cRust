package ast

import (
	"strings"

	"github.com/Sintfoap/cRust/internal/token"
)

// ComboClause is one `combo (cond) { block }` link in an IfStatement's
// else-if chain.
type ComboClause struct {
	Condition Expression
	Body      *BlockStatement
}

// IfStatement is `order (cond) { ... } combo (cond) { ... }... [special { ... }]`
// (SPEC.md §8, orderStmt). This is deliberately a Statement, not an
// Expression — SPEC.md §5.3 draws that line specifically to justify
// the ternary existing as its own thing: use IfStatement when a branch
// needs to run more than one statement, use TernaryExpression to pick
// a single value.
type IfStatement struct {
	Token       token.Token // 'order'
	Condition   Expression
	Consequence *BlockStatement
	Combos      []ComboClause
	Alternative *BlockStatement // nil if there's no `special`
}

func (is *IfStatement) statementNode()       {}
func (is *IfStatement) TokenLiteral() string { return is.Token.Literal }
func (is *IfStatement) Pos() token.Token     { return is.Token }
func (is *IfStatement) String() string {
	var out strings.Builder
	out.WriteString("order (")
	out.WriteString(is.Condition.String())
	out.WriteString(") ")
	out.WriteString(is.Consequence.String())
	for _, c := range is.Combos {
		out.WriteString(" combo (")
		out.WriteString(c.Condition.String())
		out.WriteString(") ")
		out.WriteString(c.Body.String())
	}
	if is.Alternative != nil {
		out.WriteString(" special ")
		out.WriteString(is.Alternative.String())
	}
	return out.String()
}
