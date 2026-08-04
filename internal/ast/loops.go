package ast

import "github.com/Sintfoap/cRust/internal/token"

// CountedLoop is `knead (init; cond; post) { block }` (SPEC.md §8,
// countedHeader). Init and Post are simpleStmt — in practice
// *AssignStatement, *IncDecStatement, or *ExpressionStatement — and
// each of Init/Cond/Post is nil if that clause was omitted
// (`knead (; i < 10; i++)` is legal, an empty Init).
//
// CountedLoop and ForEachLoop are both produced by the same `knead`
// keyword; the parser picks which one to build based on whether it
// sees a `(` or a bare identifier followed by `in` right after
// `knead`.
type CountedLoop struct {
	Token token.Token // 'knead'
	Init  Statement
	Cond  Expression
	Post  Statement
	Body  *BlockStatement
}

func (cl *CountedLoop) statementNode()       {}
func (cl *CountedLoop) TokenLiteral() string { return cl.Token.Literal }
func (cl *CountedLoop) Pos() token.Token     { return cl.Token }
func (cl *CountedLoop) String() string {
	out := "knead ("
	if cl.Init != nil {
		out += cl.Init.String()
	}
	out += "; "
	if cl.Cond != nil {
		out += cl.Cond.String()
	}
	out += "; "
	if cl.Post != nil {
		out += cl.Post.String()
	}
	out += ") " + cl.Body.String()
	return out
}

// ForEachLoop is `knead item in collection { block }` (SPEC.md §8,
// forEachHeader). Iterates a List/Set's elements or a Map's keys
// (SPEC.md §4's `knead` row) — which one is a runtime decision based
// on Collection's evaluated type, not something the parser resolves.
type ForEachLoop struct {
	Token      token.Token // 'knead'
	Identifier *Identifier
	Collection Expression
	Body       *BlockStatement
}

func (fel *ForEachLoop) statementNode()       {}
func (fel *ForEachLoop) TokenLiteral() string { return fel.Token.Literal }
func (fel *ForEachLoop) Pos() token.Token     { return fel.Token }
func (fel *ForEachLoop) String() string {
	return "knead " + fel.Identifier.String() + " in " + fel.Collection.String() + " " + fel.Body.String()
}

// BakeStatement is `bake (cond) { block }` (SPEC.md §8, bakeStmt) — a
// plain conditional loop, "while" by any other name.
type BakeStatement struct {
	Token     token.Token // 'bake'
	Condition Expression
	Body      *BlockStatement
}

func (bs *BakeStatement) statementNode()       {}
func (bs *BakeStatement) TokenLiteral() string { return bs.Token.Literal }
func (bs *BakeStatement) Pos() token.Token     { return bs.Token }
func (bs *BakeStatement) String() string {
	return "bake (" + bs.Condition.String() + ") " + bs.Body.String()
}
