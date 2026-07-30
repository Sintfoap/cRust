// Package ast defines cRust's abstract syntax tree — what
// internal/parser builds from a token.Token stream, and what
// internal/interpreter will eventually walk. Node names and shapes
// follow ARCHITECTURE.md's Phase 3 section exactly; SPEC.md §8 is the
// grammar this is a tree form of.
package ast

import "strings"

// Node is anything in the tree: every Statement and Expression.
// TokenLiteral returns the literal of the token the node started at
// (mainly for error messages); String reconstructs roughly-valid
// cRust source, for debug-printing and tests.
type Node interface {
	TokenLiteral() string
	String() string
}

// Statement is a Node that doesn't produce a value: order/knead/bake,
// serve/burnt/flip, assignment, recipe declarations, blocks.
type Statement interface {
	Node
	statementNode()
}

// Expression is a Node that produces a value.
type Expression interface {
	Node
	expressionNode()
}

// Program is the root of every AST — the result of parsing one whole
// .crust file.
type Program struct {
	Statements []Statement
}

func (p *Program) TokenLiteral() string {
	if len(p.Statements) > 0 {
		return p.Statements[0].TokenLiteral()
	}
	return ""
}

func (p *Program) String() string {
	var out strings.Builder
	for _, s := range p.Statements {
		out.WriteString(s.String())
		out.WriteByte('\n')
	}
	return out.String()
}
