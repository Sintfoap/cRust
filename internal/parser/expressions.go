package parser

import (
	"strconv"

	"github.com/Sintfoap/cRust/internal/ast"
	"github.com/Sintfoap/cRust/internal/token"
)

// parseExpression is the Pratt core: build a left operand with curToken's
// prefix parse function, then repeatedly fold in infix operators whose
// precedence beats the caller's threshold. Every parse function it
// calls follows the same convention it does: curToken ends on the last
// token consumed.
func (p *Parser) parseExpression(precedence int) ast.Expression {
	prefix := p.prefixParseFns[p.curToken.Type]
	if prefix == nil {
		p.noPrefixParseFnError(p.curToken.Type)
		return nil
	}
	leftExp := prefix()
	if leftExp == nil {
		return nil
	}

	for precedence < p.peekPrecedence() {
		infix := p.infixParseFns[p.peekToken.Type]
		if infix == nil {
			return leftExp
		}
		p.nextToken()
		leftExp = infix(leftExp)
		if leftExp == nil {
			return nil
		}
	}

	return leftExp
}

func (p *Parser) parseIdentifier() ast.Expression {
	return &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseIntegerLiteral() ast.Expression {
	val, err := strconv.ParseInt(p.curToken.Literal, 10, 64)
	if err != nil {
		p.errorf("could not parse %q as an integer", p.curToken.Literal)
		return nil
	}
	return &ast.IntegerLiteral{Token: p.curToken, Value: val}
}

func (p *Parser) parseFloatLiteral() ast.Expression {
	val, err := strconv.ParseFloat(p.curToken.Literal, 64)
	if err != nil {
		p.errorf("could not parse %q as a float", p.curToken.Literal)
		return nil
	}
	return &ast.FloatLiteral{Token: p.curToken, Value: val}
}

func (p *Parser) parseStringLiteral() ast.Expression {
	return &ast.StringLiteral{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseBooleanLiteral() ast.Expression {
	return &ast.BooleanLiteral{Token: p.curToken, Value: p.curTokenIs(token.STUFFED)}
}

func (p *Parser) parseNilLiteral() ast.Expression {
	return &ast.NilLiteral{Token: p.curToken}
}

// parsePrefixExpression handles unary `-x` and `hold x` (SPEC.md §5,
// unary).
func (p *Parser) parsePrefixExpression() ast.Expression {
	tok := p.curToken
	op := p.curToken.Literal

	p.nextToken()
	right := p.parseExpression(PREFIX)
	if right == nil {
		return nil
	}

	return &ast.PrefixExpression{Token: tok, Operator: op, Right: right}
}

// parseInfixExpression handles every left-associative binary operator
// that's just "left OP right": arithmetic, comparison, with/or.
// Recursing at the operator's own precedence (rather than precedence-1)
// is what makes it left-associative — a same-precedence follow-on
// operator is left for the outer loop, not swallowed here.
func (p *Parser) parseInfixExpression(left ast.Expression) ast.Expression {
	tok := p.curToken
	op := p.curToken.Literal
	prec := p.curPrecedence()

	p.nextToken()
	right := p.parseExpression(prec)
	if right == nil {
		return nil
	}

	return &ast.InfixExpression{Token: tok, Left: left, Operator: op, Right: right}
}

// parseRangeExpression handles `start..end` / `start.<end` (SPEC.md
// §5.1). The End operand parses at SUM (term) precedence, per the
// range = term [ (".."|".<") term ] production. That alone stops the
// recursive call from consuming a second range operator, but it doesn't
// stop the *outer* Pratt loop from doing so — parseExpression was
// entered at whatever precedence its caller used, which for a bare
// range statement is LOWEST, and RANGE > LOWEST — so `1..5..10` would
// otherwise re-enter this function with left = (1..5). The explicit
// peek check below rejects that chain instead of silently accepting it.
func (p *Parser) parseRangeExpression(left ast.Expression) ast.Expression {
	tok := p.curToken // '..' or '.<'
	inclusive := tok.Type == token.DOTDOT

	p.nextToken()
	end := p.parseExpression(SUM)
	if end == nil {
		return nil
	}

	if p.peekTokenIs(token.DOTDOT) || p.peekTokenIs(token.DOTLT) {
		p.errorf("ranges cannot be chained")
		return nil
	}

	return &ast.RangeExpression{Token: tok, Start: left, End: end, Inclusive: inclusive}
}

// parseElvisExpression handles `a ?: b` (SPEC.md §5.4). elvis is
// right-associative (`elvis = logicalOr [ "?:" elvis ]` — the
// right-hand side is a full elvis, not just a logicalOr), so the
// recursive call uses ELVIS-1: that's high enough to exclude another
// ternary (TERNARY, one level below ELVIS) but low enough to still pick
// up a further "?:", which is exactly what right-associativity needs.
func (p *Parser) parseElvisExpression(left ast.Expression) ast.Expression {
	tok := p.curToken // '?:'

	p.nextToken()
	right := p.parseExpression(ELVIS - 1)
	if right == nil {
		return nil
	}

	return &ast.ElvisExpression{Token: tok, Left: left, Right: right}
}

// parseTernaryExpression handles `cond (| then |) else` (SPEC.md §5.3).
// Then parses as a full expression (LOWEST) between the delimiters;
// Else parses at LOWEST too, which — since TERNARY sits directly above
// LOWEST in the precedence table — is equivalent to parsing another
// full ternary, matching the `ternary = elvis [ "(|" expression "|)"
// ternary ]` production's recursive Else.
func (p *Parser) parseTernaryExpression(cond ast.Expression) ast.Expression {
	tok := p.curToken // '(|'

	p.nextToken()
	then := p.parseExpression(LOWEST)
	if then == nil {
		return nil
	}

	if !p.expectPeek(token.TERN_ELSE) {
		return nil
	}

	p.nextToken()
	els := p.parseExpression(LOWEST)
	if els == nil {
		return nil
	}

	return &ast.TernaryExpression{Token: tok, Cond: cond, Then: then, Else: els}
}

// parseGroupedExpression handles a parenthesized expression. It doesn't
// build its own node — the parens exist only to override precedence,
// so the inner expression is returned as-is.
func (p *Parser) parseGroupedExpression() ast.Expression {
	p.nextToken()
	exp := p.parseExpression(LOWEST)
	if exp == nil {
		return nil
	}
	if !p.expectPeek(token.RPAREN) {
		return nil
	}
	return exp
}

// parseCallExpression handles `function(arg, ...)` (SPEC.md §8, call)
// as an infix operator on '(' bound to whatever expression preceded it.
func (p *Parser) parseCallExpression(function ast.Expression) ast.Expression {
	tok := p.curToken // '('
	args := p.parseExpressionList(token.RPAREN)
	if args == nil {
		return nil
	}
	return &ast.CallExpression{Token: tok, Function: function, Arguments: args}
}

// parseIndexExpression handles `left[index]` (SPEC.md §8, index).
func (p *Parser) parseIndexExpression(left ast.Expression) ast.Expression {
	tok := p.curToken // '['
	p.nextToken()
	index := p.parseExpression(LOWEST)
	if index == nil {
		return nil
	}
	if !p.expectPeek(token.RBRACKET) {
		return nil
	}
	return &ast.IndexExpression{Token: tok, Left: left, Index: index}
}

func (p *Parser) parseListLiteral() ast.Expression {
	tok := p.curToken // '['
	elements := p.parseExpressionList(token.RBRACKET)
	if elements == nil {
		return nil
	}
	return &ast.ListLiteral{Token: tok, Elements: elements}
}

// parseExpressionList parses a comma-separated list of expressions up
// to (and consuming) end. Called with curToken on the opening
// delimiter.
func (p *Parser) parseExpressionList(end token.Type) []ast.Expression {
	list := []ast.Expression{}

	if p.peekTokenIs(end) {
		p.nextToken()
		return list
	}

	p.nextToken()
	first := p.parseExpression(LOWEST)
	if first == nil {
		return nil
	}
	list = append(list, first)

	for p.peekTokenIs(token.COMMA) {
		p.nextToken() // ','
		p.nextToken() // first token of next expression
		e := p.parseExpression(LOWEST)
		if e == nil {
			return nil
		}
		list = append(list, e)
	}

	if !p.expectPeek(end) {
		return nil
	}

	return list
}

// parseMapLiteral handles `{key: value, ...}` (SPEC.md §8, mapLiteral).
// `{` starting a fresh statement is always a block (see parseStatement)
// — this prefix function only ever runs in expression position, so
// there's no ambiguity to resolve here.
func (p *Parser) parseMapLiteral() ast.Expression {
	tok := p.curToken // '{'
	pairs := []ast.MapPair{}

	if p.peekTokenIs(token.RBRACE) {
		p.nextToken()
		return &ast.MapLiteral{Token: tok, Pairs: pairs}
	}

	pair, ok := p.parseMapPair()
	if !ok {
		return nil
	}
	pairs = append(pairs, pair)

	for p.peekTokenIs(token.COMMA) {
		p.nextToken() // ','
		pair, ok = p.parseMapPair()
		if !ok {
			return nil
		}
		pairs = append(pairs, pair)
	}

	if !p.expectPeek(token.RBRACE) {
		return nil
	}

	return &ast.MapLiteral{Token: tok, Pairs: pairs}
}

// parseMapPair parses one `key: value` pair. Called with curToken on
// the token before the pair's key (either '{' or ',').
func (p *Parser) parseMapPair() (ast.MapPair, bool) {
	p.nextToken()
	key := p.parseExpression(LOWEST)
	if key == nil {
		return ast.MapPair{}, false
	}
	if !p.expectPeek(token.COLON) {
		return ast.MapPair{}, false
	}
	p.nextToken()
	value := p.parseExpression(LOWEST)
	if value == nil {
		return ast.MapPair{}, false
	}
	return ast.MapPair{Key: key, Value: value}, true
}

// parseSetLiteral handles `toppings{expr, ...}` (SPEC.md §2.2). Called
// with curToken on the TOPPINGS keyword.
func (p *Parser) parseSetLiteral() ast.Expression {
	tok := p.curToken // 'toppings'
	if !p.expectPeek(token.LBRACE) {
		return nil
	}
	elements := p.parseExpressionList(token.RBRACE)
	if elements == nil {
		return nil
	}
	return &ast.SetLiteral{Token: tok, Elements: elements}
}

// parseFunctionLiteral handles `recipe [name](params) { block }`
// (SPEC.md §8, recipeStmt) in both its statement and expression
// (anonymous literal) forms — Name is nil when no identifier follows
// `recipe`.
func (p *Parser) parseFunctionLiteral() ast.Expression {
	tok := p.curToken // 'recipe'

	var name *ast.Identifier
	if p.peekTokenIs(token.IDENT) {
		p.nextToken()
		name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	}

	if !p.expectPeek(token.LPAREN) {
		return nil
	}
	params := p.parseFunctionParameters()
	if params == nil {
		return nil
	}

	if !p.expectPeek(token.LBRACE) {
		return nil
	}
	body := p.parseBlockStatementNode()

	return &ast.FunctionLiteral{Token: tok, Name: name, Parameters: params, Body: body}
}

func (p *Parser) parseFunctionParameters() []*ast.Identifier {
	params := []*ast.Identifier{}

	if p.peekTokenIs(token.RPAREN) {
		p.nextToken()
		return params
	}

	if !p.expectPeek(token.IDENT) {
		return nil
	}
	params = append(params, &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal})

	for p.peekTokenIs(token.COMMA) {
		p.nextToken() // ','
		if !p.expectPeek(token.IDENT) {
			return nil
		}
		params = append(params, &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal})
	}

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	return params
}
