package lsp

import (
	"github.com/Sintfoap/cRust/internal/ast"
	"github.com/Sintfoap/cRust/internal/token"
)

// declKind distinguishes what an occurrence declares, for completion's
// CompletionItemKind and documentSymbol's SymbolKind — resolution
// itself (resolve, referencesTo) doesn't care which kind it is, only
// whether isDecl is set.
type declKind int

const (
	declNone declKind = iota
	declRecipe
	declParameter
	declVariable
)

// occurrence is one place an identifier's name appears in the source —
// either where it's declared (recipe name, parameter, assignment
// target, unpack target, for-each loop variable) or where it's merely
// referenced (used in an expression). scope is the index into
// fileIndex.scopes the occurrence was recorded under.
type occurrence struct {
	name   string
	tok    token.Token
	scope  int
	isDecl bool
	dk     declKind
}

// scopeInfo is one lexical scope: cRust's interpreter only gives
// `recipe` calls their own scope (SPEC.md §3 — order/knead/bake blocks
// share the enclosing one), so scopes here map 1:1 onto FunctionLiteral
// bodies, with parent chains for closures. Scope 0 is always the
// top-level/global scope.
type scopeInfo struct {
	parent int // -1 for global
}

// fileIndex is a lexical analysis of one parsed document — every
// identifier occurrence, tagged with where it was declared vs merely
// used, resolved through cRust's real function-scope-only nesting
// rather than plain textual name matching. Built fresh per request
// (buildIndex), the same "no incremental reuse" approach diagnostics
// and hover already take.
type fileIndex struct {
	scopes      []scopeInfo
	occurrences []occurrence
}

// buildIndex walks program and returns its fileIndex. Never panics on
// its own — see buildIndexSafe for the recover() wrapper every LSP
// handler actually calls, since a partially-parsed (error-recovered)
// AST from a mid-edit buffer is expected, ordinary input here, not
// something that should be able to crash the server.
func buildIndex(program *ast.Program) *fileIndex {
	idx := &fileIndex{scopes: []scopeInfo{{parent: -1}}}
	for _, stmt := range program.Statements {
		idx.walk(stmt, 0)
	}
	return idx
}

// buildIndexSafe is buildIndex with a recover() net: internal/lsp
// re-parses on every request against whatever the buffer currently
// contains, which is very often invalid mid-edit — the walker below
// wasn't written defensively against every partial-AST shape a parser
// error might leave behind, so one recover() here does that job in one
// place instead of littering nil checks through every case.
func buildIndexSafe(program *ast.Program) (idx *fileIndex) {
	defer func() {
		if recover() != nil {
			idx = &fileIndex{scopes: []scopeInfo{{parent: -1}}}
		}
	}()
	return buildIndex(program)
}

func (idx *fileIndex) pushScope(parent int) int {
	idx.scopes = append(idx.scopes, scopeInfo{parent: parent})
	return len(idx.scopes) - 1
}

func (idx *fileIndex) addDecl(id *ast.Identifier, scope int, dk declKind) {
	if id == nil {
		return
	}
	idx.occurrences = append(idx.occurrences, occurrence{
		name: id.Value, tok: id.Token, scope: scope, isDecl: true, dk: dk,
	})
}

func (idx *fileIndex) addRef(id *ast.Identifier, scope int) {
	if id == nil {
		return
	}
	idx.occurrences = append(idx.occurrences, occurrence{
		name: id.Value, tok: id.Token, scope: scope,
	})
}

// walk recursively records every identifier occurrence reachable from
// node, tracking which lexical scope each one belongs to. Bare
// *ast.Identifier nodes default to references; the handful of places
// that actually bind a new name (assignment targets, unpack targets,
// for-each loop variables, recipe names/parameters) are special-cased
// below to record a declaration instead before recursing normally into
// whatever they contain.
func (idx *fileIndex) walk(node ast.Node, scope int) {
	if node == nil {
		return
	}

	switch n := node.(type) {
	case *ast.BlockStatement:
		for _, s := range n.Statements {
			idx.walk(s, scope)
		}
	case *ast.ExpressionStatement:
		idx.walk(n.Expression, scope)
	case *ast.AssignStatement:
		if id, ok := n.Target.(*ast.Identifier); ok {
			idx.addDecl(id, scope, declVariable)
		} else {
			idx.walk(n.Target, scope)
		}
		idx.walk(n.Value, scope)
	case *ast.UnpackAssignStatement:
		for _, t := range n.Targets {
			idx.addDecl(t, scope, declVariable)
		}
		idx.walk(n.Value, scope)
	case *ast.IncDecStatement:
		idx.walk(n.Target, scope)
	case *ast.ReturnStatement:
		idx.walk(n.ReturnValue, scope)
	case *ast.BurntStatement, *ast.FlipStatement:
		// No identifiers.
	case *ast.IfStatement:
		idx.walk(n.Condition, scope)
		idx.walk(n.Consequence, scope)
		for _, c := range n.Combos {
			idx.walk(c.Condition, scope)
			idx.walk(c.Body, scope)
		}
		if n.Alternative != nil {
			idx.walk(n.Alternative, scope)
		}
	case *ast.CountedLoop:
		idx.walk(n.Init, scope)
		idx.walk(n.Cond, scope)
		idx.walk(n.Post, scope)
		idx.walk(n.Body, scope)
	case *ast.ForEachLoop:
		idx.addDecl(n.Identifier, scope, declVariable)
		idx.walk(n.Collection, scope)
		idx.walk(n.Body, scope)
	case *ast.BakeStatement:
		idx.walk(n.Condition, scope)
		idx.walk(n.Body, scope)
	case *ast.FunctionLiteral:
		if n.Name != nil {
			idx.addDecl(n.Name, scope, declRecipe)
		}
		inner := idx.pushScope(scope)
		for _, p := range n.Parameters {
			idx.addDecl(p, inner, declParameter)
		}
		idx.walk(n.Body, inner)
	case *ast.Identifier:
		idx.addRef(n, scope)
	case *ast.PrefixExpression:
		idx.walk(n.Right, scope)
	case *ast.InfixExpression:
		idx.walk(n.Left, scope)
		idx.walk(n.Right, scope)
	case *ast.RangeExpression:
		idx.walk(n.Start, scope)
		idx.walk(n.End, scope)
	case *ast.CallExpression:
		idx.walk(n.Function, scope)
		for _, a := range n.Arguments {
			idx.walk(a, scope)
		}
	case *ast.IndexExpression:
		idx.walk(n.Left, scope)
		idx.walk(n.Index, scope)
	case *ast.ListLiteral:
		for _, e := range n.Elements {
			idx.walk(e, scope)
		}
	case *ast.MapLiteral:
		for _, p := range n.Pairs {
			idx.walk(p.Key, scope)
			idx.walk(p.Value, scope)
		}
	case *ast.SetLiteral:
		for _, e := range n.Elements {
			idx.walk(e, scope)
		}
	case *ast.TernaryExpression:
		idx.walk(n.Cond, scope)
		idx.walk(n.Then, scope)
		idx.walk(n.Else, scope)
	case *ast.ElvisExpression:
		idx.walk(n.Left, scope)
		idx.walk(n.Right, scope)
	case *ast.IntegerLiteral, *ast.FloatLiteral, *ast.StringLiteral, *ast.BooleanLiteral, *ast.NilLiteral:
		// No identifiers.
	}
}

// resolve finds name's declaration as seen from fromScope: the
// textually-first declaration of name in the nearest enclosing scope
// that declares it at all, walking outward through parent scopes for
// closures — cRust's actual scoping rule (SPEC.md §3), not a plain
// same-name text search. Returns nil if name isn't declared anywhere
// in fromScope's chain (a builtin or keyword, most commonly).
func (idx *fileIndex) resolve(name string, fromScope int) *occurrence {
	for s := fromScope; s != -1; s = idx.scopes[s].parent {
		var best *occurrence
		for i := range idx.occurrences {
			o := &idx.occurrences[i]
			if !o.isDecl || o.scope != s || o.name != name {
				continue
			}
			if best == nil || tokenBefore(o.tok, best.tok) {
				best = o
			}
		}
		if best != nil {
			return best
		}
	}
	return nil
}

func tokenBefore(a, b token.Token) bool {
	if a.Line != b.Line {
		return a.Line < b.Line
	}
	return a.Col < b.Col
}

// occurrenceAt returns the occurrence recorded at exactly (line, col)
// — the same 1-indexed positions internal/lexer's tokens carry — if
// any. Positions are unique per occurrence (each corresponds to one
// distinct identifier token in the source), so equality here is exact,
// not a range containment check.
func (idx *fileIndex) occurrenceAt(line, col int) (*occurrence, bool) {
	for i := range idx.occurrences {
		if idx.occurrences[i].tok.Line == line && idx.occurrences[i].tok.Col == col {
			return &idx.occurrences[i], true
		}
	}
	return nil, false
}

// referencesTo returns every occurrence (including def itself) whose
// own resolve() lands on def — i.e., every use of the same binding,
// not just every occurrence that happens to share def's name. A
// same-named local in an unrelated function is correctly excluded,
// since its own resolve() finds a different, nearer declaration.
func (idx *fileIndex) referencesTo(def *occurrence) []occurrence {
	var out []occurrence
	for i := range idx.occurrences {
		o := &idx.occurrences[i]
		if idx.resolve(o.name, o.scope) == def {
			out = append(out, *o)
		}
	}
	return out
}

// declarations returns one occurrence per distinct declared name
// (first one seen), for completion — deliberately whole-document
// scoped rather than resolved against the cursor's actual lexical
// scope (see hoverAt's doc comment for the same "deliberately
// shallow" philosophy): internal/ast's nodes don't carry a block's
// closing position, so there's no cheap way to know which scope a
// blank/whitespace cursor position (as opposed to an existing
// identifier token) falls inside. Offering every name in the document
// is occasionally over-inclusive but never wrong in the sense of
// hiding a real completion.
func (idx *fileIndex) declarations() []occurrence {
	seen := map[string]bool{}
	var out []occurrence
	for _, o := range idx.occurrences {
		if !o.isDecl || seen[o.name] {
			continue
		}
		seen[o.name] = true
		out = append(out, o)
	}
	return out
}

// recipes returns every recipe (named function) declaration, in
// source order, for documentSymbol.
func (idx *fileIndex) recipes() []occurrence {
	var out []occurrence
	for _, o := range idx.occurrences {
		if o.isDecl && o.dk == declRecipe {
			out = append(out, o)
		}
	}
	return out
}
