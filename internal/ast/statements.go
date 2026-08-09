package ast

import (
	"fmt"
	"strings"

	"github.com/Sintfoap/cRust/internal/token"
)

// BlockStatement is `{ statement... }` (SPEC.md §8, block) — the body
// of a recipe, order/combo/special, or a loop. Note that a block
// existing doesn't imply a new scope: SPEC.md §3 only gives `recipe`
// calls their own Environment, so evaluating a BlockStatement for
// order/knead/bake must reuse the enclosing scope, not create one.
type BlockStatement struct {
	Token      token.Token // '{'
	Statements []Statement
}

func (bs *BlockStatement) statementNode()       {}
func (bs *BlockStatement) TokenLiteral() string { return bs.Token.Literal }
func (bs *BlockStatement) Pos() token.Token     { return bs.Token }
func (bs *BlockStatement) String() string {
	var out strings.Builder
	out.WriteString("{\n")
	for _, s := range bs.Statements {
		out.WriteString(s.String())
		out.WriteByte('\n')
	}
	out.WriteString("}")
	return out.String()
}

// ExpressionStatement wraps a bare expression used on its own as a
// statement — almost always a call, e.g. `deliver(x)` on its own line.
type ExpressionStatement struct {
	Token      token.Token // the expression's first token
	Expression Expression
}

func (es *ExpressionStatement) statementNode()       {}
func (es *ExpressionStatement) TokenLiteral() string { return es.Token.Literal }
func (es *ExpressionStatement) Pos() token.Token     { return es.Token }
func (es *ExpressionStatement) String() string {
	if es.Expression == nil {
		return ""
	}
	return es.Expression.String()
}

// AssignStatement is `target OP value` for a single lvalue (SPEC.md
// §3, §8 assignStmt) — Target is an *Identifier or *IndexExpression,
// Operator is one of "=", "+=", "-=", "*=", "/=", "%=".
type AssignStatement struct {
	Token    token.Token // the operator token
	Target   Expression
	Operator string
	Value    Expression
}

func (as *AssignStatement) statementNode()       {}
func (as *AssignStatement) TokenLiteral() string { return as.Token.Literal }
func (as *AssignStatement) Pos() token.Token     { return as.Token }
func (as *AssignStatement) String() string {
	return as.Target.String() + " " + as.Operator + " " + as.Value.String()
}

// UnpackAssignStatement is `id, id, ... = value` (SPEC.md §3.1) —
// always plain `=`, always bare identifier targets (never index
// expressions). Value can evaluate to either a List (the classic rule:
// first N-1 targets take one element each, the last always takes a
// List of everything left over) or a Tuple (SPEC.md §2: exact arity
// required, every target — including the last — takes its own bare
// value) — internal/interpreter's evalUnpackAssignStatement dispatches
// on Value's *runtime* type, not anything visible here at parse time,
// since Value can be any expression (a ternary choosing between two
// Tuples, a function call returning one, etc.), not just a literal.
type UnpackAssignStatement struct {
	Token   token.Token // the first identifier's token
	Targets []*Identifier
	Value   Expression
}

func (uas *UnpackAssignStatement) statementNode()       {}
func (uas *UnpackAssignStatement) TokenLiteral() string { return uas.Token.Literal }
func (uas *UnpackAssignStatement) Pos() token.Token     { return uas.Token }
func (uas *UnpackAssignStatement) String() string {
	names := make([]string, len(uas.Targets))
	for i, t := range uas.Targets {
		names[i] = t.String()
	}
	return strings.Join(names, ", ") + " = " + uas.Value.String()
}

// IncDecStatement is `target++`/`++target`/`target--`/`--target`
// (SPEC.md §5.2) — Operator is "++" or "--"; which side it appeared on
// in the source doesn't matter, since both mean the same thing.
type IncDecStatement struct {
	Token    token.Token // '++' or '--'
	Target   Expression
	Operator string
}

func (ids *IncDecStatement) statementNode()       {}
func (ids *IncDecStatement) TokenLiteral() string { return ids.Token.Literal }
func (ids *IncDecStatement) Pos() token.Token     { return ids.Token }
func (ids *IncDecStatement) String() string {
	return ids.Target.String() + ids.Operator
}

// ReturnStatement is `serve [value]` (SPEC.md §4) — ReturnValue is nil
// for a bare `serve`.
type ReturnStatement struct {
	Token       token.Token // 'serve'
	ReturnValue Expression  // nil if bare
}

func (rs *ReturnStatement) statementNode()       {}
func (rs *ReturnStatement) TokenLiteral() string { return rs.Token.Literal }
func (rs *ReturnStatement) Pos() token.Token     { return rs.Token }
func (rs *ReturnStatement) String() string {
	if rs.ReturnValue == nil {
		return "serve"
	}
	return "serve " + rs.ReturnValue.String()
}

// BurntStatement is `burnt` (SPEC.md §4) — break.
type BurntStatement struct {
	Token token.Token
}

func (bs *BurntStatement) statementNode()       {}
func (bs *BurntStatement) TokenLiteral() string { return bs.Token.Literal }
func (bs *BurntStatement) String() string       { return "burnt" }
func (bs *BurntStatement) Pos() token.Token     { return bs.Token }

// FlipStatement is `flip` (SPEC.md §4) — continue.
type FlipStatement struct {
	Token token.Token
}

func (fs *FlipStatement) statementNode()       {}
func (fs *FlipStatement) TokenLiteral() string { return fs.Token.Literal }
func (fs *FlipStatement) String() string       { return "flip" }
func (fs *FlipStatement) Pos() token.Token     { return fs.Token }

// DeliveryStatement is `delivery "path/to/file.crust"` (SPEC.md §10) —
// cRust's module system: brings another file's top-level bindings
// (recipes, variables) into the importing file's own scope, resolved
// and evaluated at internal/interpreter's discretion (once per
// distinct file, even if delivered from more than one place — not
// visible here). Path is always a plain string literal, never a
// general expression — the same "no computed imports" choice Go's own
// `import` makes — so there's no separate Expression node for it, just
// the already-unescaped literal text token.STRING's own lexing already
// produces.
type DeliveryStatement struct {
	Token token.Token // 'delivery'
	Path  string
}

func (ds *DeliveryStatement) statementNode()       {}
func (ds *DeliveryStatement) TokenLiteral() string { return ds.Token.Literal }
func (ds *DeliveryStatement) Pos() token.Token     { return ds.Token }
func (ds *DeliveryStatement) String() string       { return fmt.Sprintf("delivery %q", ds.Path) }
