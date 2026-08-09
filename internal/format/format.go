// Package format is cRust's source formatter — the engine behind
// `crust fmt` (cmd/crust) and `crust lsp`'s textDocument/formatting.
// Format walks an already-parsed *ast.Program and re-prints it in a
// single canonical style: 4-space indentation, K&R braces, and
// minimal-but-correct parenthesization recomputed from operator
// precedence rather than whatever the original source happened to
// contain (internal/ast's own parser drops grouping parens entirely
// during parsing — see internal/parser/expressions.go's
// parseGroupedExpression — so there is no "original parens" to
// preserve in the first place; recomputing them from scratch is the
// only option, and it has the added benefit of stripping genuinely
// redundant ones the author wrote).
//
// See comments.go for why this package also takes the raw source text
// (not just the AST) — `//` comments are invisible to the parser, so
// recovering and re-weaving them is a separate, standalone pass.
package format

import (
	"strconv"
	"strings"

	"github.com/Sintfoap/cRust/internal/ast"
)

const indentUnit = "    " // 4 spaces, matching every hand-written example in examples/*.crust

// Precedence levels, mirroring internal/parser/parser.go's own ladder
// exactly (SPEC.md §5) plus one level above everything real parser
// precedence uses: atomIsh, for nodes that never need parens no matter
// the context (literals, identifiers, and anything already
// self-delimiting like a bracketed/braced literal or a recipe body).
// Keeping this table local rather than importing internal/parser's
// (unexported) one avoids a format->parser dependency for a handful of
// integers; TestPrecedenceMatchesParser cross-checks it against actual
// parse results instead, so the two can't silently drift apart.
const (
	precLowest = iota
	precTernary
	precElvis
	precOr
	precAnd // with
	precEquals
	precCompare
	precRange
	precSum
	precProduct
	precPrefix
	precCall
	precIndex
	precAtom
)

var infixPrecedence = map[string]int{
	"with": precAnd,
	"or":   precOr,
	"==":   precEquals,
	"!=":   precEquals,
	"<":    precCompare,
	">":    precCompare,
	"<=":   precCompare,
	">=":   precCompare,
	"+":    precSum,
	"-":    precSum,
	"*":    precProduct,
	"/":    precProduct,
	"%":    precProduct,
}

// nodePrecedence reports e's own binding strength for
// parenthesization purposes — see the package doc and printer.expr's
// own comment for how this is used.
func nodePrecedence(e ast.Expression) int {
	switch e := e.(type) {
	case *ast.InfixExpression:
		if p, ok := infixPrecedence[e.Operator]; ok {
			return p
		}
		return precLowest // unreachable for a successfully parsed program
	case *ast.PrefixExpression:
		return precPrefix
	case *ast.RangeExpression:
		return precRange
	case *ast.TernaryExpression:
		return precTernary
	case *ast.ElvisExpression:
		return precElvis
	case *ast.CallExpression:
		return precCall
	case *ast.IndexExpression:
		return precIndex
	default:
		return precAtom
	}
}

// Format returns program re-printed in cRust's canonical style, with
// src's comments woven back in. src must be the exact text program was
// parsed from — Format has no way to detect a mismatch, and a mismatch
// would misplace or drop comments (see comments.go).
func Format(program *ast.Program, src string) string {
	p := &printer{comments: scanComments(src)}
	p.programStatements(program.Statements)
	p.flushRemainingComments()
	out := p.buf.String()
	return strings.TrimRight(out, "\n") + "\n"
}

// printer holds the mutable state one Format call threads through
// every print method: the output being built, the current indent
// depth, the comment-recovery cursor (see comments.go), and
// lastEmittedLine (see blankLineIfGap).
type printer struct {
	buf    strings.Builder
	indent int

	comments []Comment
	nextC    int // index into comments of the first not-yet-emitted one

	// lastEmittedLine is the source line whatever was most recently
	// written to buf — a comment or a statement — came from (0 before
	// anything has been emitted at all). blankLineIfGap is the only
	// reader; flushStandaloneBefore and programStatements are the only
	// writers, both right after actually emitting something. A single
	// package-level (not per-nesting-depth) cursor is exactly right
	// here: comments and statements are both processed in strict
	// global source order regardless of how deep the current block
	// nesting is, the same reason the comment cursor itself
	// (nextC) is never reset or scoped per block either.
	lastEmittedLine int
}

func (p *printer) indentString() string { return strings.Repeat(indentUnit, p.indent) }

// blankLineIfGap inserts one blank line if nextLine is more than one
// past lastEmittedLine — the single shared gap check behind every kind
// of blank-line preservation this package does: between two top-level
// statements, between two statements inside a block, before a leading
// comment (even when the previous thing was itself a statement, not
// another comment), between two consecutive leading comments, and
// after the last comment in a leading block and before the statement
// it precedes. Collapses any run of 2+ source blank lines to exactly
// 1, the same normalization gofmt applies. A lastEmittedLine of 0
// (nothing emitted yet) never inserts one — there's nothing to leave a
// gap after at the very start of the file.
func (p *printer) blankLineIfGap(nextLine int) {
	if p.lastEmittedLine != 0 && nextLine > p.lastEmittedLine+1 {
		p.buf.WriteByte('\n')
	}
}

// flushStandaloneBefore emits every not-yet-consumed comment whose
// line is less than beforeLine, each on its own indented line (with
// blankLineIfGap ahead of it, so a blank line anywhere in a run of
// leading comments — before the first one or between any two —
// survives), and advances the cursor past them. Called right before
// printing whatever comes next (a statement, or nothing — see
// flushRemainingComments), so any comment sitting between the
// previous line and beforeLine surfaces in its original position.
//
// A comment recorded as a trailing (non-standalone) one that reaches
// this function unconsumed means the statement it originally trailed
// never claimed it (documented limitation: only a statement's first
// line supports trailing-comment attachment — see statements.go's
// finishLine) — rather than ever dropping it, it still prints here,
// just deferred to its own line instead of staying glued to the
// original one.
func (p *printer) flushStandaloneBefore(beforeLine int) {
	for p.nextC < len(p.comments) && p.comments[p.nextC].Line < beforeLine {
		c := p.comments[p.nextC]
		p.blankLineIfGap(c.Line)
		p.buf.WriteString(p.indentString())
		p.buf.WriteString(c.Text)
		p.buf.WriteByte('\n')
		p.lastEmittedLine = c.Line
		p.nextC++
	}
}

// flushRemainingComments is flushStandaloneBefore with no upper bound
// — for whatever's left after the last statement in the whole
// program: genuine end-of-file trailing comments, and (per the
// tradeoff documented on flushStandaloneBefore) any comment that was
// sitting just before a closing brace deep inside the file with
// nothing of its own to attach to on the way back out.
func (p *printer) flushRemainingComments() {
	p.flushStandaloneBefore(1<<31 - 1)
}

// trailingComment returns the exactly-next comment's text if it sits
// on forLine and was written on the same line as code (Standalone
// false), consuming it — or "", false otherwise. Comments are
// processed in strict source order via a single forward cursor, so
// checking only the immediate next one (not scanning ahead) is
// sufficient: nothing later in the list can have a smaller line
// number.
func (p *printer) trailingComment(forLine int) (string, bool) {
	if p.nextC >= len(p.comments) {
		return "", false
	}
	c := p.comments[p.nextC]
	if c.Line != forLine || c.Standalone {
		return "", false
	}
	p.nextC++
	return c.Text, true
}

// rawLine writes text as a complete output line at the current indent
// with no trailing-comment lookup — for lines (a block's closing
// brace, a combo/special continuation) that a statement's one-
// trailing-comment-per-statement budget (see statements.go's
// finishLine) doesn't cover.
func (p *printer) rawLine(text string) {
	p.buf.WriteString(p.indentString())
	p.buf.WriteString(text)
	p.buf.WriteByte('\n')
}

// programStatements prints Program's top-level statements, and
// blockStatements (statements.go) does the same for a block's body one
// indent level deeper — both go through this shared loop so comment
// interleaving and blank-line preservation (flushStandaloneBefore,
// blankLineIfGap) are identical at every nesting depth, not just
// between top-level statements. lastEmittedLine is advanced to s's own
// estimated last line (lastLine) after each statement, so the next
// iteration's gap check — whether for a following statement or a
// leading comment ahead of one — measures from the right place.
func (p *printer) programStatements(stmts []ast.Statement) {
	for _, s := range stmts {
		p.flushStandaloneBefore(s.Pos().Line)
		p.blankLineIfGap(s.Pos().Line)
		p.statement(s)
		p.lastEmittedLine = lastLine(s)
	}
}

// lastLine best-effort estimates the last source line s's own tokens
// span, for programStatements' lastEmittedLine tracking (in turn used
// by blankLineIfGap). cRust's AST doesn't record a closing brace's
// position (see comments.go's design note), so a block-bodied
// statement's "last line" here is approximated by its last inner
// statement's own last line, recursively — close enough to tell "was
// there a blank line" apart from "was there not," which is all this
// needs. Any statement carrying an expression is routed through
// lastLineOfExpr too, not just s.Pos().Line — a plain-looking
// ExpressionStatement can still embed a FunctionLiteral arbitrarily
// deep (`deliver(map(xs, recipe(x) { ... }))`), the one *expression*
// node with a block body of its own, and printing one always expands
// it onto multiple lines regardless of how compact the source was.
// Missing that case isn't just a cosmetic miss the way the package
// doc comment above allows for: formatting that output a second time
// would then see a "gap" ahead of the next statement that was never a
// gap in the original source, inserting a blank line
// TestFormatIsIdempotent doesn't expect — caught by exactly that
// scenario in examples/day1_essentials.crust before this existed.
func lastLine(s ast.Statement) int {
	switch s := s.(type) {
	case *ast.IfStatement:
		body := s.Consequence
		if len(s.Combos) > 0 {
			body = s.Combos[len(s.Combos)-1].Body
		}
		if s.Alternative != nil {
			body = s.Alternative
		}
		return lastLineOfBlock(body, s.Token.Line)
	case *ast.CountedLoop:
		return lastLineOfBlock(s.Body, s.Token.Line)
	case *ast.ForEachLoop:
		return lastLineOfBlock(s.Body, s.Token.Line)
	case *ast.BakeStatement:
		return lastLineOfBlock(s.Body, s.Token.Line)
	case *ast.ExpressionStatement:
		return max(s.Pos().Line, lastLineOfExpr(s.Expression))
	case *ast.AssignStatement:
		return max(s.Pos().Line, lastLineOfExpr(s.Value))
	case *ast.UnpackAssignStatement:
		return max(s.Pos().Line, lastLineOfExpr(s.Value))
	case *ast.IncDecStatement:
		return max(s.Pos().Line, lastLineOfExpr(s.Target))
	case *ast.ReturnStatement:
		if s.ReturnValue == nil {
			return s.Pos().Line
		}
		return max(s.Pos().Line, lastLineOfExpr(s.ReturnValue))
	default:
		return s.Pos().Line
	}
}

// lastLineOfExpr is lastLine's own counterpart for expressions: an
// expression's tokens never span multiple *printed* lines on their
// own, except when it embeds a FunctionLiteral — recursed into since
// one can sit arbitrarily deep inside a call's arguments, an infix
// operand, a collection literal's elements, and so on. Every other
// node either has no sub-expressions (a literal, an identifier — the
// base cases, each returning its own single token's line) or simply
// takes the widest span across whichever it has. ast.Expression has no
// Pos() of its own (unlike ast.Statement — see ast.go's own doc
// comment on why), so every case here reads .Token directly off the
// concrete type instead.
func lastLineOfExpr(e ast.Expression) int {
	switch e := e.(type) {
	case *ast.FunctionLiteral:
		return lastLineOfBlock(e.Body, e.Token.Line)
	case *ast.CallExpression:
		m := lastLineOfExpr(e.Function)
		for _, a := range e.Arguments {
			m = max(m, lastLineOfExpr(a))
		}
		return m
	case *ast.IndexExpression:
		return max(lastLineOfExpr(e.Left), lastLineOfExpr(e.Index))
	case *ast.InfixExpression:
		return max(lastLineOfExpr(e.Left), lastLineOfExpr(e.Right))
	case *ast.PrefixExpression:
		return lastLineOfExpr(e.Right)
	case *ast.RangeExpression:
		return max(lastLineOfExpr(e.Start), lastLineOfExpr(e.End))
	case *ast.TernaryExpression:
		return max(lastLineOfExpr(e.Cond), lastLineOfExpr(e.Then), lastLineOfExpr(e.Else))
	case *ast.ElvisExpression:
		return max(lastLineOfExpr(e.Left), lastLineOfExpr(e.Right))
	case *ast.ListLiteral:
		return maxOverExprs(e.Elements, e.Token.Line)
	case *ast.TupleLiteral:
		return maxOverExprs(e.Elements, e.Token.Line)
	case *ast.SetLiteral:
		return maxOverExprs(e.Elements, e.Token.Line)
	case *ast.MapLiteral:
		m := e.Token.Line
		for _, p := range e.Pairs {
			m = max(m, lastLineOfExpr(p.Key), lastLineOfExpr(p.Value))
		}
		return m
	case *ast.Identifier:
		return e.Token.Line
	case *ast.IntegerLiteral:
		return e.Token.Line
	case *ast.FloatLiteral:
		return e.Token.Line
	case *ast.StringLiteral:
		return e.Token.Line
	case *ast.BooleanLiteral:
		return e.Token.Line
	case *ast.NilLiteral:
		return e.Token.Line
	default:
		// Every ast.Expression type is listed above; this only matters
		// if a new one is ever added without a case here, in which
		// case 0 (blankLineIfGap's own "nothing emitted yet" sentinel)
		// is the safe fallback — it never triggers a spurious blank
		// line, it can only ever miss a legitimate one, the same
		// "worst outcome is one cosmetic blank line" ceiling every
		// other estimate in this file already has.
		return 0
	}
}

// maxOverExprs is lastLineOfExpr's shared helper for the three
// collection literals (List/Tuple/Set) whose only sub-expressions are
// a flat Elements slice.
func maxOverExprs(exprs []ast.Expression, base int) int {
	m := base
	for _, e := range exprs {
		m = max(m, lastLineOfExpr(e))
	}
	return m
}

// lastLineOfBlock estimates the line b's own closing brace sits on:
// headerLine+1 for an empty block (nothing between the header and the
// brace but the brace's own line), or the last inner statement's own
// lastLine+1 otherwise — the "+1" in both cases standing in for the
// brace's line itself, one past whatever content precedes it. Exact
// whenever a block has no interior blank lines/comments hugging its
// closing brace (the overwhelmingly common case); when it doesn't,
// the worst outcome is blankLineBetweenTopLevel's caller adding or
// omitting one cosmetic blank line, never a correctness issue.
func lastLineOfBlock(b *ast.BlockStatement, headerLine int) int {
	if b == nil || len(b.Statements) == 0 {
		return headerLine + 1
	}
	return lastLine(b.Statements[len(b.Statements)-1]) + 1
}

// stringLiteral renders s the way it would have to be written back as
// cRust source: escaping exactly the characters internal/lexer's
// readString decodes (`\n \t \r \" \\`) and nothing else — Go's own
// %q/strconv.Quote is close but not guaranteed identical (it can use
// escapes cRust's own lexer doesn't understand, like \x/\u forms for
// other control or non-printable runes), so this is hand-rolled rather
// than reused.
func stringLiteral(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// floatLiteral renders f the same way the lexer's own FLOAT token
// text would read for a canonical, re-parseable form — strconv's
// shortest round-tripping representation ('g' format, -1 precision),
// the same choice object.Float.Inspect() already makes for exactly
// this reason.
func floatLiteral(f float64) string {
	return strconv.FormatFloat(f, 'g', -1, 64)
}
