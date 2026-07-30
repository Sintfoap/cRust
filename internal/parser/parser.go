// Package parser turns a token.Token stream (from internal/lexer) into
// an internal/ast tree. It's a recursive-descent parser for statements
// with a Pratt (top-down operator precedence) core for expressions —
// see ARCHITECTURE.md's Phase 3 section for the design this implements,
// and SPEC.md §8 for the grammar it parses.
package parser

import (
	"fmt"

	"github.com/Sintfoap/cRust/internal/ast"
	"github.com/Sintfoap/cRust/internal/lexer"
	"github.com/Sintfoap/cRust/internal/token"
)

type (
	prefixParseFn func() ast.Expression
	infixParseFn  func(left ast.Expression) ast.Expression
)

// Precedence levels, lowest to highest, matching SPEC.md §5's ladder:
// ternary < elvis < or < with < equality < comparison < range < sum <
// product < unary < call/index.
const (
	LOWEST int = iota
	TERNARY
	ELVIS
	OR
	AND
	EQUALS
	LESSGREATER
	RANGE
	SUM
	PRODUCT
	PREFIX
	CALL
	INDEX
)

var precedences = map[token.Type]int{
	token.TERN_THEN: TERNARY,
	token.ELVIS:     ELVIS,
	token.OR:        OR,
	token.WITH:      AND,
	token.EQ:        EQUALS,
	token.NOT_EQ:    EQUALS,
	token.LT:        LESSGREATER,
	token.GT:        LESSGREATER,
	token.LE:        LESSGREATER,
	token.GE:        LESSGREATER,
	token.DOTDOT:    RANGE,
	token.DOTLT:     RANGE,
	token.PLUS:      SUM,
	token.MINUS:     SUM,
	token.STAR:      PRODUCT,
	token.SLASH:     PRODUCT,
	token.PERCENT:   PRODUCT,
	token.LPAREN:    CALL,
	token.LBRACKET:  INDEX,
}

// Parser holds a two-token lookahead window (curToken/peekToken) over a
// Lexer and the parse function tables the Pratt core dispatches
// through. curToken is always the last token consumed by whatever parse
// function is running — every parse function in this package leaves the
// Parser in that state on return, error or not.
type Parser struct {
	l *lexer.Lexer

	curToken  token.Token
	peekToken token.Token

	errors []string

	prefixParseFns map[token.Type]prefixParseFn
	infixParseFns  map[token.Type]infixParseFn
}

// New returns a Parser ready to call ParseProgram on l's token stream.
func New(l *lexer.Lexer) *Parser {
	p := &Parser{l: l, errors: []string{}}

	p.prefixParseFns = make(map[token.Type]prefixParseFn)
	p.registerPrefix(token.IDENT, p.parseIdentifier)
	p.registerPrefix(token.INT, p.parseIntegerLiteral)
	p.registerPrefix(token.FLOAT, p.parseFloatLiteral)
	p.registerPrefix(token.STRING, p.parseStringLiteral)
	p.registerPrefix(token.STUFFED, p.parseBooleanLiteral)
	p.registerPrefix(token.THIN, p.parseBooleanLiteral)
	p.registerPrefix(token.NOBOX, p.parseNilLiteral)
	p.registerPrefix(token.MINUS, p.parsePrefixExpression)
	p.registerPrefix(token.HOLD, p.parsePrefixExpression)
	p.registerPrefix(token.LPAREN, p.parseGroupedExpression)
	p.registerPrefix(token.LBRACKET, p.parseListLiteral)
	p.registerPrefix(token.LBRACE, p.parseMapLiteral)
	p.registerPrefix(token.TOPPINGS, p.parseSetLiteral)
	p.registerPrefix(token.RECIPE, p.parseFunctionLiteral)

	p.infixParseFns = make(map[token.Type]infixParseFn)
	p.registerInfix(token.PLUS, p.parseInfixExpression)
	p.registerInfix(token.MINUS, p.parseInfixExpression)
	p.registerInfix(token.STAR, p.parseInfixExpression)
	p.registerInfix(token.SLASH, p.parseInfixExpression)
	p.registerInfix(token.PERCENT, p.parseInfixExpression)
	p.registerInfix(token.EQ, p.parseInfixExpression)
	p.registerInfix(token.NOT_EQ, p.parseInfixExpression)
	p.registerInfix(token.LT, p.parseInfixExpression)
	p.registerInfix(token.GT, p.parseInfixExpression)
	p.registerInfix(token.LE, p.parseInfixExpression)
	p.registerInfix(token.GE, p.parseInfixExpression)
	p.registerInfix(token.WITH, p.parseInfixExpression)
	p.registerInfix(token.OR, p.parseInfixExpression)
	p.registerInfix(token.DOTDOT, p.parseRangeExpression)
	p.registerInfix(token.DOTLT, p.parseRangeExpression)
	p.registerInfix(token.ELVIS, p.parseElvisExpression)
	p.registerInfix(token.TERN_THEN, p.parseTernaryExpression)
	p.registerInfix(token.LPAREN, p.parseCallExpression)
	p.registerInfix(token.LBRACKET, p.parseIndexExpression)

	// Prime curToken/peekToken.
	p.nextToken()
	p.nextToken()

	return p
}

func (p *Parser) registerPrefix(t token.Type, fn prefixParseFn) { p.prefixParseFns[t] = fn }
func (p *Parser) registerInfix(t token.Type, fn infixParseFn)   { p.infixParseFns[t] = fn }

// Errors returns every parse error collected so far, in source order.
func (p *Parser) Errors() []string { return p.errors }

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

func (p *Parser) curTokenIs(t token.Type) bool  { return p.curToken.Type == t }
func (p *Parser) peekTokenIs(t token.Type) bool { return p.peekToken.Type == t }

// expectPeek advances and returns true if peekToken has type t;
// otherwise it records an error and leaves the Parser positioned where
// it was.
func (p *Parser) expectPeek(t token.Type) bool {
	if p.peekTokenIs(t) {
		p.nextToken()
		return true
	}
	p.peekError(t)
	return false
}

func (p *Parser) peekPrecedence() int {
	if pr, ok := precedences[p.peekToken.Type]; ok {
		return pr
	}
	return LOWEST
}

func (p *Parser) curPrecedence() int {
	if pr, ok := precedences[p.curToken.Type]; ok {
		return pr
	}
	return LOWEST
}

func (p *Parser) errorf(format string, args ...any) {
	p.errors = append(p.errors, fmt.Sprintf("line %d:%d: %s", p.curToken.Line, p.curToken.Col, fmt.Sprintf(format, args...)))
}

func (p *Parser) peekError(t token.Type) {
	p.errors = append(p.errors, fmt.Sprintf("line %d:%d: expected next token to be %s, got %s (%q) instead",
		p.peekToken.Line, p.peekToken.Col, t, p.peekToken.Type, p.peekToken.Literal))
}

func (p *Parser) noPrefixParseFnError(t token.Type) {
	p.errorf("no prefix parse function for %s found", t)
}

// ParseProgram parses the whole token stream into a *ast.Program. Parse
// errors don't stop the walk — a statement that fails to parse is
// skipped via synchronize() and collected in Errors() — so a single
// call reports as many independent errors as it can find, the same way
// the lexer's TestExampleFiles-style tests expect from this package.
func (p *Parser) ParseProgram() *ast.Program {
	program := &ast.Program{Statements: []ast.Statement{}}

	p.skipTerminators()
	for !p.curTokenIs(token.EOF) {
		stmt := p.parseStatement()
		if stmt != nil {
			program.Statements = append(program.Statements, stmt)
		} else {
			p.synchronize()
		}
		p.nextToken()
		p.skipTerminators()
	}

	return program
}

func (p *Parser) skipTerminators() {
	for p.curTokenIs(token.NEWLINE) || p.curTokenIs(token.SEMICOLON) {
		p.nextToken()
	}
}

// expectTerminator consumes a statement terminator (NEWLINE or ';') if
// one is next. A following '}' or EOF also counts as an implicit
// terminator and is left unconsumed, so a block's last statement never
// needs a trailing newline before its closing brace.
func (p *Parser) expectTerminator() {
	if p.peekTokenIs(token.NEWLINE) || p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
		return
	}
	if p.peekTokenIs(token.RBRACE) || p.peekTokenIs(token.EOF) {
		return
	}
	p.errorf("expected statement terminator, got %s (%q) instead", p.peekToken.Type, p.peekToken.Literal)
}

// parseBlockStatementNode parses `{ statement... }` (SPEC.md §8, block).
// Called with curToken on the opening '{'; always returns a non-nil
// *ast.BlockStatement (even for a malformed block — errors are recorded
// via Errors(), not surfaced through a nil return) and leaves curToken
// on the closing '}' (or EOF, if the block was never closed).
func (p *Parser) parseBlockStatementNode() *ast.BlockStatement {
	block := &ast.BlockStatement{Token: p.curToken, Statements: []ast.Statement{}}

	p.nextToken()
	p.skipTerminators()

	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		} else {
			p.synchronize()
		}
		p.nextToken()
		p.skipTerminators()
	}

	if !p.curTokenIs(token.RBRACE) {
		p.errorf("expected '}' to close block, got %s instead", p.curToken.Type)
	}

	return block
}

// synchronize skips tokens after a parse error until it reaches a
// plausible statement boundary: a terminator, a block's closing '}', or
// a keyword that starts a new statement. This keeps one bad statement
// from cascading into a wall of unrelated errors.
func (p *Parser) synchronize() {
	for !p.curTokenIs(token.EOF) {
		if p.curTokenIs(token.NEWLINE) || p.curTokenIs(token.SEMICOLON) || p.curTokenIs(token.RBRACE) {
			return
		}
		switch p.peekToken.Type {
		case token.RECIPE, token.ORDER, token.KNEAD, token.BAKE, token.SERVE, token.BURNT, token.FLIP:
			return
		}
		p.nextToken()
	}
}
