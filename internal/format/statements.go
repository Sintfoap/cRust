package format

import "github.com/Sintfoap/cRust/internal/ast"

// startLine writes the current indent prefix — the first thing every
// statement-printing method does.
func (p *printer) startLine() { p.buf.WriteString(p.indentString()) }

// finishLine closes out whatever the current statement has already
// written to p.buf (which may itself contain embedded newlines — a
// nested *ast.FunctionLiteral value prints its own multi-line body
// directly, see expressions.go's function method) by appending a
// same-line trailing comment recovered for sourceLine, if there is
// one, and exactly one newline. Every statement-printing method calls
// this exactly once, however many lines its own content actually
// spanned.
func (p *printer) finishLine(sourceLine int) {
	if c, ok := p.trailingComment(sourceLine); ok {
		p.buf.WriteString("   ")
		p.buf.WriteString(c)
	}
	p.buf.WriteByte('\n')
}

// blockStatements prints a block's statements one indent level deeper
// — shared by every block-bodied construct (recipe, order/combo/
// special, knead, bake). Goes through the exact same per-statement
// loop programStatements uses for Program's own top-level statements
// (comment interleaving, blank-line preservation), just at one more
// indent level, so both look identical regardless of nesting depth.
//
// headerLine is the source line the block's own opening "{" sits on
// (`order (...) {`, `} combo (...) {`, `recipe foo() {`, ...) — every
// caller writes that header line itself, outside programStatements'
// own loop, so without this, lastEmittedLine would still be whatever
// it was before the header line was ever written, and the block's
// first inner statement's blank-line check (blankLineIfGap) would
// measure the gap from the wrong place, potentially inserting a blank
// line right after a header that had none following it in the source.
func (p *printer) blockStatements(headerLine int, stmts []ast.Statement) {
	p.lastEmittedLine = headerLine
	p.indent++
	p.programStatements(stmts)
	p.indent--
}

// statement dispatches to the right printer for s's concrete type.
// Every case ends with exactly one finishLine call (some indirectly,
// via a helper) — see finishLine's own doc comment for why that holds
// regardless of how many lines the statement's own content spans.
func (p *printer) statement(s ast.Statement) {
	switch s := s.(type) {
	case *ast.ExpressionStatement:
		p.expressionStatement(s)
	case *ast.AssignStatement:
		p.startLine()
		p.writeExpr(s.Target, precLowest)
		p.buf.WriteByte(' ')
		p.buf.WriteString(s.Operator)
		p.buf.WriteByte(' ')
		p.writeExpr(s.Value, precLowest)
		p.finishLine(s.Token.Line)
	case *ast.UnpackAssignStatement:
		p.startLine()
		for i, t := range s.Targets {
			if i > 0 {
				p.buf.WriteString(", ")
			}
			p.buf.WriteString(t.Value)
		}
		p.buf.WriteString(" = ")
		p.writeExpr(s.Value, precLowest)
		p.finishLine(s.Token.Line)
	case *ast.IncDecStatement:
		p.startLine()
		p.writeExpr(s.Target, precLowest)
		p.buf.WriteString(s.Operator)
		p.finishLine(s.Token.Line)
	case *ast.ReturnStatement:
		p.startLine()
		p.buf.WriteString("serve")
		if s.ReturnValue != nil {
			p.buf.WriteByte(' ')
			p.writeExpr(s.ReturnValue, precLowest)
		}
		p.finishLine(s.Token.Line)
	case *ast.BurntStatement:
		p.startLine()
		p.buf.WriteString("burnt")
		p.finishLine(s.Token.Line)
	case *ast.FlipStatement:
		p.startLine()
		p.buf.WriteString("flip")
		p.finishLine(s.Token.Line)
	case *ast.DeliveryStatement:
		p.startLine()
		p.buf.WriteString(s.String())
		p.finishLine(s.Token.Line)
	case *ast.IfStatement:
		p.ifStatement(s)
	case *ast.CountedLoop:
		p.countedLoop(s)
	case *ast.ForEachLoop:
		p.forEachLoop(s)
	case *ast.BakeStatement:
		p.bakeStatement(s)
	}
}

// expressionStatement special-cases a named *ast.FunctionLiteral (a
// `recipe name(...) { ... }` declaration — SPEC.md §8 recipeStmt) so
// it prints exactly like every other block-bodied statement (header
// line, indented body, closing brace on its own line), matching every
// hand-written example in examples/*.crust, rather than through the
// generic single-line "write an expression, finish the line" path
// every other bare expression statement uses (`deliver(x)`, a bare
// call to a recipe for side effects, ...).
func (p *printer) expressionStatement(es *ast.ExpressionStatement) {
	if fl, ok := es.Expression.(*ast.FunctionLiteral); ok && fl.Name != nil {
		p.namedFunctionLiteral(fl)
		return
	}
	p.startLine()
	if es.Expression != nil {
		p.writeExpr(es.Expression, precLowest)
	}
	p.finishLine(es.Token.Line)
}

func (p *printer) ifStatement(is *ast.IfStatement) {
	p.startLine()
	p.buf.WriteString("order (")
	p.writeExpr(is.Condition, precLowest)
	p.buf.WriteString(") {")
	p.finishLine(is.Token.Line)
	p.blockStatements(is.Token.Line, is.Consequence.Statements)

	for _, c := range is.Combos {
		p.startLine()
		p.buf.WriteString("} combo (")
		p.writeExpr(c.Condition, precLowest)
		p.buf.WriteString(") {")
		p.buf.WriteByte('\n')
		p.blockStatements(c.Body.Token.Line, c.Body.Statements)
	}

	if is.Alternative != nil {
		p.rawLine("} special {")
		p.blockStatements(is.Alternative.Token.Line, is.Alternative.Statements)
	}

	p.rawLine("}")
}

func (p *printer) countedLoop(cl *ast.CountedLoop) {
	p.startLine()
	p.buf.WriteString("knead (")
	if cl.Init != nil {
		p.simpleStatementText(cl.Init)
	}
	p.buf.WriteString("; ")
	if cl.Cond != nil {
		p.writeExpr(cl.Cond, precLowest)
	}
	p.buf.WriteString("; ")
	if cl.Post != nil {
		p.simpleStatementText(cl.Post)
	}
	p.buf.WriteString(") {")
	p.finishLine(cl.Token.Line)
	p.blockStatements(cl.Token.Line, cl.Body.Statements)
	p.rawLine("}")
}

func (p *printer) forEachLoop(fel *ast.ForEachLoop) {
	p.startLine()
	p.buf.WriteString("knead ")
	p.buf.WriteString(fel.Identifier.Value)
	p.buf.WriteString(" in ")
	p.writeExpr(fel.Collection, precLowest)
	p.buf.WriteString(" {")
	p.finishLine(fel.Token.Line)
	p.blockStatements(fel.Token.Line, fel.Body.Statements)
	p.rawLine("}")
}

func (p *printer) bakeStatement(bs *ast.BakeStatement) {
	p.startLine()
	p.buf.WriteString("bake (")
	p.writeExpr(bs.Condition, precLowest)
	p.buf.WriteString(") {")
	p.finishLine(bs.Token.Line)
	p.blockStatements(bs.Token.Line, bs.Body.Statements)
	p.rawLine("}")
}

// simpleStatementText prints a *ast.CountedLoop Init/Post clause
// in-line (no indent, no trailing newline of its own) — in practice
// always an *ast.AssignStatement or *ast.IncDecStatement (SPEC.md §8's
// countedHeader), never anything block-bodied, so there's no
// recursion-depth or comment-placement concern here the way a full
// p.statement call would raise.
func (p *printer) simpleStatementText(s ast.Statement) {
	switch s := s.(type) {
	case *ast.AssignStatement:
		p.writeExpr(s.Target, precLowest)
		p.buf.WriteByte(' ')
		p.buf.WriteString(s.Operator)
		p.buf.WriteByte(' ')
		p.writeExpr(s.Value, precLowest)
	case *ast.IncDecStatement:
		p.writeExpr(s.Target, precLowest)
		p.buf.WriteString(s.Operator)
	case *ast.ExpressionStatement:
		if s.Expression != nil {
			p.writeExpr(s.Expression, precLowest)
		}
	}
}
