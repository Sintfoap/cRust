package ast

import "github.com/Sintfoap/cRust/internal/token"

// TernaryExpression is `cond (| then |) else` (SPEC.md §5.3). It gets
// its own node — three children, not two — rather than reusing
// InfixExpression, since the middle `(|`/`|)` delimiters bound a whole
// second expression, not a single right-hand operand.
type TernaryExpression struct {
	Token token.Token // '(|'
	Cond  Expression
	Then  Expression
	Else  Expression
}

func (te *TernaryExpression) expressionNode()      {}
func (te *TernaryExpression) TokenLiteral() string { return te.Token.Literal }
func (te *TernaryExpression) String() string {
	return "(" + te.Cond.String() + " (| " + te.Then.String() + " |) " + te.Else.String() + ")"
}

// ElvisExpression is `a ?: b` (SPEC.md §5.4) — a if it isn't nobox,
// else b.
type ElvisExpression struct {
	Token token.Token // '?:'
	Left  Expression
	Right Expression
}

func (ee *ElvisExpression) expressionNode()      {}
func (ee *ElvisExpression) TokenLiteral() string { return ee.Token.Literal }
func (ee *ElvisExpression) String() string {
	return "(" + ee.Left.String() + " ?: " + ee.Right.String() + ")"
}
