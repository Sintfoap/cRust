package format

import (
	"strconv"

	"github.com/Sintfoap/cRust/internal/ast"
)

// writeExpr writes e's canonical text to p.buf, wrapping it in parens
// exactly when nodePrecedence(e) is lower than minPrec — the caller's
// way of saying "anything binding less tightly than this would change
// meaning if printed bare here." Every call site in this package
// passes the exact threshold the language's own precedence-climbing
// parser would require for e to land in that exact position
// unparenthesized; see format.go's package doc and the precedence
// table above nodePrecedence for the reasoning. Where that reasoning
// left a genuine choice between "provably minimal" and "provably safe
// even in an edge case not worth the extra proof," this always takes
// safe — a stray extra paren around a doubly-negated prefix
// expression is a cosmetic footnote no AoC solution will ever trip on;
// changed output semantics on a reformat would not be.
func (p *printer) writeExpr(e ast.Expression, minPrec int) {
	if nodePrecedence(e) < minPrec {
		p.buf.WriteByte('(')
		p.writeExprBare(e)
		p.buf.WriteByte(')')
		return
	}
	p.writeExprBare(e)
}

func (p *printer) writeExprBare(e ast.Expression) {
	switch e := e.(type) {
	case *ast.Identifier:
		p.buf.WriteString(e.Value)
	case *ast.IntegerLiteral:
		p.buf.WriteString(strconv.FormatInt(e.Value, 10))
	case *ast.FloatLiteral:
		p.buf.WriteString(floatLiteral(e.Value))
	case *ast.StringLiteral:
		p.buf.WriteString(stringLiteral(e.Value))
	case *ast.BooleanLiteral:
		if e.Value {
			p.buf.WriteString("stuffed")
		} else {
			p.buf.WriteString("thin")
		}
	case *ast.NilLiteral:
		p.buf.WriteString("nobox")
	case *ast.PrefixExpression:
		p.buf.WriteString(e.Operator)
		switch {
		case e.Operator == "hold":
			p.buf.WriteByte(' ')
		case e.Operator == "-" && startsWithMinus(e.Right):
			// Without this, stacked negation ("- -x") would print as
			// "--x" — not just ugly but a different, invalid token:
			// internal/lexer lexes a bare "--" as the decrement
			// operator (IncDecStatement), not two MINUS tokens, so the
			// result wouldn't even re-lex the same way, let alone
			// re-parse to the same tree. Nothing else in this
			// language's grammar can produce two adjacent operator
			// characters that collide with a different token this way
			// (every infix operator already prints with spaces on both
			// sides, and "hold" always gets its own space above), so
			// this one targeted check covers it.
			p.buf.WriteByte(' ')
		}
		// precPrefix, not precPrefix+1: unlike an infix operator's
		// right operand (parsed via a bounded parseExpression(X) call
		// that only incorporates strictly-higher-precedence
		// continuations), a prefix operator's operand comes from an
		// unconditional recursive prefix() call with no precedence
		// bound of its own — nested prefix operators ("- -x") chain
		// through that path regardless, so a same-precedence child
		// here (another PrefixExpression) reparses correctly without
		// a paren, unlike the infix case this package is otherwise
		// conservative about.
		p.writeExpr(e.Right, precPrefix)
	case *ast.InfixExpression:
		prec := infixPrecedence[e.Operator]
		p.writeExpr(e.Left, prec)
		p.buf.WriteByte(' ')
		p.buf.WriteString(e.Operator)
		p.buf.WriteByte(' ')
		p.writeExpr(e.Right, prec+1)
	case *ast.RangeExpression:
		p.writeExpr(e.Start, precRange)
		if e.Inclusive {
			p.buf.WriteString("..")
		} else {
			p.buf.WriteString(".<")
		}
		p.writeExpr(e.End, precProduct)
	case *ast.TernaryExpression:
		p.writeExpr(e.Cond, precTernary+1)
		p.buf.WriteString(" (| ")
		p.writeExpr(e.Then, precLowest)
		p.buf.WriteString(" |) ")
		p.writeExpr(e.Else, precTernary+1)
	case *ast.ElvisExpression:
		p.writeExpr(e.Left, precElvis+1)
		p.buf.WriteString(" ?: ")
		p.writeExpr(e.Right, precElvis)
	case *ast.CallExpression:
		p.writeExpr(e.Function, precCall)
		p.buf.WriteByte('(')
		for i, a := range e.Arguments {
			if i > 0 {
				p.buf.WriteString(", ")
			}
			p.writeExpr(a, precLowest)
		}
		p.buf.WriteByte(')')
	case *ast.IndexExpression:
		p.writeExpr(e.Left, precIndex)
		p.buf.WriteByte('[')
		p.writeExpr(e.Index, precLowest)
		p.buf.WriteByte(']')
	case *ast.ListLiteral:
		p.buf.WriteByte('[')
		for i, el := range e.Elements {
			if i > 0 {
				p.buf.WriteString(", ")
			}
			p.writeExpr(el, precLowest)
		}
		p.buf.WriteByte(']')
	case *ast.TupleLiteral:
		// No 1-element special case: internal/parser's own grammar
		// (parseGroupedExpression) can only ever build a TupleLiteral
		// with 2+ Elements in the first place — a single comma
		// immediately followed by ')' has nothing valid to parse as a
		// second element, so a 1-element Tuple literal has no source
		// syntax to round-trip through at all, and never reaches here.
		p.buf.WriteByte('(')
		for i, el := range e.Elements {
			if i > 0 {
				p.buf.WriteString(", ")
			}
			p.writeExpr(el, precLowest)
		}
		p.buf.WriteByte(')')
	case *ast.SetLiteral:
		p.buf.WriteString("toppings{")
		for i, el := range e.Elements {
			if i > 0 {
				p.buf.WriteString(", ")
			}
			p.writeExpr(el, precLowest)
		}
		p.buf.WriteByte('}')
	case *ast.MapLiteral:
		p.buf.WriteByte('{')
		for i, pair := range e.Pairs {
			if i > 0 {
				p.buf.WriteString(", ")
			}
			p.writeExpr(pair.Key, precLowest)
			p.buf.WriteString(": ")
			p.writeExpr(pair.Value, precLowest)
		}
		p.buf.WriteByte('}')
	case *ast.FunctionLiteral:
		p.anonymousFunctionLiteral(e)
	}
}

// namedFunctionLiteral prints a `recipe name(...) { ... }` declaration
// (statements.go's expressionStatement routes here) — the block-
// statement shape every top-level recipe in examples/*.crust uses.
func (p *printer) namedFunctionLiteral(fl *ast.FunctionLiteral) {
	p.startLine()
	p.functionHeader(fl)
	p.buf.WriteString(" {")
	p.finishLine(fl.Token.Line)
	p.blockStatements(fl.Token.Line, fl.Body.Statements)
	p.rawLine("}")
}

// anonymousFunctionLiteral prints a recipe literal used in expression
// position — assigned to a variable, passed as a callback to map(),
// immediately invoked, or (Name != nil) a nested named declaration
// used as a value in its own right rather than as a bare top-level
// statement. Always the same multi-line block shape
// namedFunctionLiteral uses (matching this language's one established
// style for a recipe body, full stop, rather than also supporting a
// single-line "recipe(x) { serve x }" compact form) — written directly
// into whatever line is already in progress (an assignment's value, a
// call argument, ...), so unlike every other expression kind this one
// does emit embedded newlines and indentation of its own; see
// statements.go's finishLine doc comment for why that's fine.
func (p *printer) anonymousFunctionLiteral(fl *ast.FunctionLiteral) {
	p.functionHeader(fl)
	p.buf.WriteString(" {\n")
	p.blockStatements(fl.Token.Line, fl.Body.Statements)
	p.buf.WriteString(p.indentString())
	p.buf.WriteByte('}')
}

// startsWithMinus reports whether e prints with a leading '-' with
// nothing (no paren, no other character) in front of it — true only
// for a PrefixExpression whose own operator is "-", since
// nodePrecedence(e) == precPrefix always passes writeExpr's own
// precPrefix threshold (see the PrefixExpression case above), meaning
// such a child is guaranteed to print unparenthesized here. No other
// expression kind in this grammar ever renders with a bare leading
// '-' (an Integer/FloatLiteral's own text is always non-negative —
// negation is always a separate PrefixExpression, never part of the
// literal token itself).
func startsWithMinus(e ast.Expression) bool {
	pe, ok := e.(*ast.PrefixExpression)
	return ok && pe.Operator == "-"
}

func (p *printer) functionHeader(fl *ast.FunctionLiteral) {
	p.buf.WriteString("recipe")
	if fl.Name != nil {
		p.buf.WriteByte(' ')
		p.buf.WriteString(fl.Name.Value)
	}
	p.buf.WriteByte('(')
	for i, param := range fl.Parameters {
		if i > 0 {
			p.buf.WriteString(", ")
		}
		p.buf.WriteString(param.Value)
	}
	p.buf.WriteByte(')')
}
