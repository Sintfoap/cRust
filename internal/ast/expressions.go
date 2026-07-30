package ast

import (
	"strings"

	"github.com/Sintfoap/cRust/internal/token"
)

// PrefixExpression is a unary operator applied to an operand:
// `-x` or `hold x` (SPEC.md §5, unary).
type PrefixExpression struct {
	Token    token.Token // the operator token
	Operator string
	Right    Expression
}

func (pe *PrefixExpression) expressionNode()      {}
func (pe *PrefixExpression) TokenLiteral() string { return pe.Token.Literal }
func (pe *PrefixExpression) String() string {
	return "(" + pe.Operator + " " + pe.Right.String() + ")"
}

// InfixExpression covers every binary operator that's just "left OP
// right" with no special structure of its own: arithmetic, comparison,
// and the keyword logical operators (with/or). Ranges, ternary, and
// Elvis get their own node types instead, since each needs more than
// one right-hand operand slot.
type InfixExpression struct {
	Token    token.Token // the operator token
	Left     Expression
	Operator string
	Right    Expression
}

func (ie *InfixExpression) expressionNode()      {}
func (ie *InfixExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *InfixExpression) String() string {
	return "(" + ie.Left.String() + " " + ie.Operator + " " + ie.Right.String() + ")"
}

// RangeExpression is `start..end` or `start.<end` (SPEC.md §5.1).
type RangeExpression struct {
	Token     token.Token // '..' or '.<'
	Start     Expression
	End       Expression
	Inclusive bool // true for .., false for .<
}

func (re *RangeExpression) expressionNode()      {}
func (re *RangeExpression) TokenLiteral() string { return re.Token.Literal }
func (re *RangeExpression) String() string {
	op := ".."
	if !re.Inclusive {
		op = ".<"
	}
	return "(" + re.Start.String() + op + re.End.String() + ")"
}

// CallExpression is `function(arg, arg, ...)` (SPEC.md §8, call).
// Function is usually an *Identifier but can be any Expression that
// evaluates to a callable, e.g. an immediately-invoked FunctionLiteral.
type CallExpression struct {
	Token     token.Token // '('
	Function  Expression
	Arguments []Expression
}

func (ce *CallExpression) expressionNode()      {}
func (ce *CallExpression) TokenLiteral() string { return ce.Token.Literal }
func (ce *CallExpression) String() string {
	args := make([]string, len(ce.Arguments))
	for i, a := range ce.Arguments {
		args[i] = a.String()
	}
	return ce.Function.String() + "(" + strings.Join(args, ", ") + ")"
}

// IndexExpression is `left[index]` (SPEC.md §8, index) — List by
// position, Map by key, String by position; not valid on Set, but
// that's a runtime restriction (SPEC.md §2.2), not a parse-time one.
type IndexExpression struct {
	Token token.Token // '['
	Left  Expression
	Index Expression
}

func (ie *IndexExpression) expressionNode()      {}
func (ie *IndexExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *IndexExpression) String() string {
	return "(" + ie.Left.String() + "[" + ie.Index.String() + "])"
}
