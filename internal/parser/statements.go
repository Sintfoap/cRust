package parser

import (
	"github.com/Sintfoap/cRust/internal/ast"
	"github.com/Sintfoap/cRust/internal/token"
)

// parseStatement dispatches on curToken's type to the right statement
// parse function (SPEC.md §8, statement). It's the entry point both
// ParseProgram and parseBlockStatementNode loop over.
func (p *Parser) parseStatement() ast.Statement {
	switch p.curToken.Type {
	case token.RECIPE:
		return p.parseRecipeStatement()
	case token.ORDER:
		return p.parseIfStatement()
	case token.KNEAD:
		return p.parseKneadStatement()
	case token.BAKE:
		return p.parseBakeStatement()
	case token.SERVE:
		return p.parseReturnStatement()
	case token.BURNT:
		return p.parseBurntStatement()
	case token.FLIP:
		return p.parseFlipStatement()
	case token.LBRACE:
		return p.parseBlockStatementNode()
	default:
		return p.parseSimpleStatementStmt()
	}
}

// parseRecipeStatement handles `recipe name(params) { block }` used as
// a statement. There's no dedicated "recipe declaration" AST node —
// SPEC.md's grammar reuses recipeStmt for both statement and expression
// (primary) position, so this just wraps the FunctionLiteral in an
// ExpressionStatement; deciding what a named FunctionLiteral means
// (binding it in the environment) is an Eval-time concern, not a
// parse-time one.
func (p *Parser) parseRecipeStatement() ast.Statement {
	lit := p.parseFunctionLiteral()
	if lit == nil {
		return nil
	}
	fn := lit.(*ast.FunctionLiteral)
	return &ast.ExpressionStatement{Token: fn.Token, Expression: fn}
}

// parseIfStatement handles `order (cond) {...} combo (cond) {...}...
// [special {...}]` (SPEC.md §8, orderStmt).
func (p *Parser) parseIfStatement() ast.Statement {
	stmt := &ast.IfStatement{Token: p.curToken}

	if !p.expectPeek(token.LPAREN) {
		return nil
	}
	p.nextToken()
	stmt.Condition = p.parseExpression(LOWEST)
	if stmt.Condition == nil {
		return nil
	}
	if !p.expectPeek(token.RPAREN) {
		return nil
	}
	if !p.expectPeek(token.LBRACE) {
		return nil
	}
	stmt.Consequence = p.parseBlockStatementNode()

	for p.peekTokenIs(token.COMBO) {
		p.nextToken() // 'combo'
		combo := ast.ComboClause{}

		if !p.expectPeek(token.LPAREN) {
			return nil
		}
		p.nextToken()
		combo.Condition = p.parseExpression(LOWEST)
		if combo.Condition == nil {
			return nil
		}
		if !p.expectPeek(token.RPAREN) {
			return nil
		}
		if !p.expectPeek(token.LBRACE) {
			return nil
		}
		combo.Body = p.parseBlockStatementNode()

		stmt.Combos = append(stmt.Combos, combo)
	}

	if p.peekTokenIs(token.SPECIAL) {
		p.nextToken() // 'special'
		if !p.expectPeek(token.LBRACE) {
			return nil
		}
		stmt.Alternative = p.parseBlockStatementNode()
	}

	return stmt
}

// parseKneadStatement dispatches between countedHeader and
// forEachHeader (SPEC.md §8) — a '(' right after `knead` always means
// countedHeader, a bare identifier always means forEachHeader, so one
// token of lookahead is enough to tell them apart.
func (p *Parser) parseKneadStatement() ast.Statement {
	if p.peekTokenIs(token.LPAREN) {
		return p.parseCountedLoop()
	}
	return p.parseForEachLoop()
}

// parseCountedLoop handles `knead (init; cond; post) { block }`
// (SPEC.md §8, countedHeader), where each of init/cond/post may be
// omitted. Each step below leaves curToken on the ';' or ')' that ends
// that clause, whether or not the clause itself was present.
func (p *Parser) parseCountedLoop() ast.Statement {
	loop := &ast.CountedLoop{Token: p.curToken} // 'knead'

	if !p.expectPeek(token.LPAREN) {
		return nil
	}

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	} else {
		p.nextToken()
		loop.Init = p.parseSimpleStatement()
		if loop.Init == nil {
			return nil
		}
		if !p.expectPeek(token.SEMICOLON) {
			return nil
		}
	}

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	} else {
		p.nextToken()
		loop.Cond = p.parseExpression(LOWEST)
		if loop.Cond == nil {
			return nil
		}
		if !p.expectPeek(token.SEMICOLON) {
			return nil
		}
	}

	if p.peekTokenIs(token.RPAREN) {
		p.nextToken()
	} else {
		p.nextToken()
		loop.Post = p.parseSimpleStatement()
		if loop.Post == nil {
			return nil
		}
		if !p.expectPeek(token.RPAREN) {
			return nil
		}
	}

	if !p.expectPeek(token.LBRACE) {
		return nil
	}
	loop.Body = p.parseBlockStatementNode()

	return loop
}

// parseForEachLoop handles `knead item in collection { block }`
// (SPEC.md §8, forEachHeader).
func (p *Parser) parseForEachLoop() ast.Statement {
	loop := &ast.ForEachLoop{Token: p.curToken} // 'knead'

	if !p.expectPeek(token.IDENT) {
		return nil
	}
	loop.Identifier = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(token.IN) {
		return nil
	}
	p.nextToken()
	loop.Collection = p.parseExpression(LOWEST)
	if loop.Collection == nil {
		return nil
	}

	if !p.expectPeek(token.LBRACE) {
		return nil
	}
	loop.Body = p.parseBlockStatementNode()

	return loop
}

// parseBakeStatement handles `bake (cond) { block }` (SPEC.md §8,
// bakeStmt).
func (p *Parser) parseBakeStatement() ast.Statement {
	stmt := &ast.BakeStatement{Token: p.curToken}

	if !p.expectPeek(token.LPAREN) {
		return nil
	}
	p.nextToken()
	stmt.Condition = p.parseExpression(LOWEST)
	if stmt.Condition == nil {
		return nil
	}
	if !p.expectPeek(token.RPAREN) {
		return nil
	}
	if !p.expectPeek(token.LBRACE) {
		return nil
	}
	stmt.Body = p.parseBlockStatementNode()

	return stmt
}

// parseReturnStatement handles `serve [expression]` (SPEC.md §8,
// serveStmt) — ReturnValue stays nil for a bare `serve`.
func (p *Parser) parseReturnStatement() ast.Statement {
	stmt := &ast.ReturnStatement{Token: p.curToken}

	if p.peekTokenIs(token.NEWLINE) || p.peekTokenIs(token.SEMICOLON) ||
		p.peekTokenIs(token.RBRACE) || p.peekTokenIs(token.EOF) {
		p.expectTerminator()
		return stmt
	}

	p.nextToken()
	stmt.ReturnValue = p.parseExpression(LOWEST)
	if stmt.ReturnValue == nil {
		return nil
	}
	p.expectTerminator()
	return stmt
}

func (p *Parser) parseBurntStatement() ast.Statement {
	stmt := &ast.BurntStatement{Token: p.curToken}
	p.expectTerminator()
	return stmt
}

func (p *Parser) parseFlipStatement() ast.Statement {
	stmt := &ast.FlipStatement{Token: p.curToken}
	p.expectTerminator()
	return stmt
}

// parseSimpleStatementStmt wraps parseSimpleStatement with the
// terminator that simpleStmt requires when it's used as a full
// statement (SPEC.md §8: `simpleStmt terminator`). knead's counted
// header calls parseSimpleStatement directly instead, since there ';'
// is a structural separator rather than a terminator.
func (p *Parser) parseSimpleStatementStmt() ast.Statement {
	stmt := p.parseSimpleStatement()
	if stmt == nil {
		return nil
	}
	p.expectTerminator()
	return stmt
}

// parseSimpleStatement handles assignStmt | unpackAssign | incDecStmt |
// expression (SPEC.md §8, simpleStmt), without consuming a terminator.
func (p *Parser) parseSimpleStatement() ast.Statement {
	if p.curTokenIs(token.INC) || p.curTokenIs(token.DEC) {
		tok := p.curToken
		p.nextToken()
		target := p.parseLvalue()
		if target == nil {
			return nil
		}
		if !isValidLvalue(target) {
			p.errorf("invalid increment/decrement target %s", target.String())
			return nil
		}
		return &ast.IncDecStatement{Token: tok, Target: target, Operator: tok.Literal}
	}

	if p.curTokenIs(token.IDENT) && p.peekTokenIs(token.COMMA) {
		return p.parseUnpackAssignStatement()
	}

	startTok := p.curToken
	expr := p.parseExpression(LOWEST)
	if expr == nil {
		return nil
	}

	if isAssignOp(p.peekToken.Type) {
		return p.parseAssignStatement(expr)
	}

	if p.peekTokenIs(token.INC) || p.peekTokenIs(token.DEC) {
		if !isValidLvalue(expr) {
			p.errorf("invalid increment/decrement target %s", expr.String())
			return nil
		}
		p.nextToken()
		return &ast.IncDecStatement{Token: p.curToken, Target: expr, Operator: p.curToken.Literal}
	}

	return &ast.ExpressionStatement{Token: startTok, Expression: expr}
}

// parseLvalue parses `identifier { index }` (SPEC.md §8, lvalue) — used
// for the prefix `++x`/`--x` form, where there's no already-parsed
// expression to check. Parsing at CALL precedence lets chained indexing
// through while stopping short of a trailing call, since `++f()` isn't
// a valid target.
func (p *Parser) parseLvalue() ast.Expression {
	return p.parseExpression(CALL)
}

// parseAssignStatement handles `target OP value` (SPEC.md §8,
// assignStmt) given the already-parsed target expression; curToken is
// still on the target's last token when this is called.
func (p *Parser) parseAssignStatement(target ast.Expression) ast.Statement {
	if !isValidLvalue(target) {
		p.errorf("invalid assignment target %s", target.String())
		return nil
	}

	p.nextToken() // move to the assignment operator
	tok := p.curToken
	op := p.curToken.Literal

	p.nextToken() // move to the first token of the value expression
	value := p.parseExpression(LOWEST)
	if value == nil {
		return nil
	}

	return &ast.AssignStatement{Token: tok, Target: target, Operator: op, Value: value}
}

// parseUnpackAssignStatement handles `id, id, ... = value` (SPEC.md
// §3.1, §8 unpackAssign). Called with curToken on the first identifier.
func (p *Parser) parseUnpackAssignStatement() ast.Statement {
	stmt := &ast.UnpackAssignStatement{Token: p.curToken}
	stmt.Targets = append(stmt.Targets, &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal})

	for p.peekTokenIs(token.COMMA) {
		p.nextToken() // ','
		if !p.expectPeek(token.IDENT) {
			return nil
		}
		stmt.Targets = append(stmt.Targets, &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal})
	}

	if !p.expectPeek(token.ASSIGN) {
		return nil
	}

	p.nextToken()
	stmt.Value = p.parseExpression(LOWEST)
	if stmt.Value == nil {
		return nil
	}

	return stmt
}

func isAssignOp(t token.Type) bool {
	switch t {
	case token.ASSIGN, token.PLUS_ASSIGN, token.MINUS_ASSIGN, token.STAR_ASSIGN, token.SLASH_ASSIGN, token.PERCENT_ASSIGN:
		return true
	default:
		return false
	}
}

func isValidLvalue(e ast.Expression) bool {
	switch e.(type) {
	case *ast.Identifier, *ast.IndexExpression:
		return true
	default:
		return false
	}
}
