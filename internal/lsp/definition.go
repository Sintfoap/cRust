package lsp

import (
	"github.com/Sintfoap/cRust/internal/lexer"
	"github.com/Sintfoap/cRust/internal/parser"
	"github.com/Sintfoap/cRust/internal/token"
)

// keywordSpellings lists every reserved word's actual source spelling
// (token.Keywords() keys) once, for completion — keywordDocs in
// hover.go is keyed by token.Type instead (e.g. token.RECIPE, whose
// own string value is "RECIPE", not "recipe"), which is the right key
// for a token lookup but the wrong one for a completion label.
var keywordSpellings = func() []string {
	kws := token.Keywords()
	out := make([]string, 0, len(kws))
	for spelling := range kws {
		out = append(out, spelling)
	}
	return out
}()

// identTokenAt returns the IDENT token whose span contains pos, if
// any — unlike hoverAt's lenient "last token at or before the cursor"
// match, this requires the cursor to genuinely be inside the
// identifier, since jumping to a definition (or renaming) from a
// position that isn't really on a name would be surprising rather than
// helpful.
func identTokenAt(text string, pos Position, encoding string) (token.Token, bool) {
	lines := splitLines(text)
	var lineText string
	if pos.Line >= 0 && pos.Line < len(lines) {
		lineText = lines[pos.Line]
	}
	targetLine := pos.Line + 1
	targetCol := decodeOffset(lineText, pos.Character, encoding) + 1

	l := lexer.New(text)
	for {
		tok := l.NextToken()
		if tok.Type == token.EOF {
			return token.Token{}, false
		}
		if tok.Line > targetLine {
			return token.Token{}, false
		}
		if tok.Line != targetLine || tok.Type != token.IDENT {
			continue
		}
		end := tok.Col + len([]rune(tok.Literal))
		if targetCol >= tok.Col && targetCol < end {
			return tok, true
		}
	}
}

// indexAndOccurrenceAt lexes+parses text fresh and finds the
// occurrence at pos, if the cursor is on an identifier at all.
func indexAndOccurrenceAt(text string, pos Position, encoding string) (*fileIndex, *occurrence) {
	tok, ok := identTokenAt(text, pos, encoding)
	if !ok {
		return nil, nil
	}
	p := parser.New(lexer.New(text))
	program := p.ParseProgram()
	idx := buildIndexSafe(program)
	o, ok := idx.occurrenceAt(tok.Line, tok.Col)
	if !ok {
		return idx, nil
	}
	return idx, o
}

// occurrenceRange turns o's position and name into an LSP Range —
// [start, start+len(name)) — encoded per encoding, the same
// start/end-width convention diagnostics.go uses for lexer/parser
// errors.
func occurrenceRange(lines []string, o *occurrence, encoding string) Range {
	start := toPosition(lines, o.tok.Line, o.tok.Col, encoding)
	lineIdx := o.tok.Line - 1
	var lineText string
	if lineIdx >= 0 && lineIdx < len(lines) {
		lineText = lines[lineIdx]
	}
	runeCol := (o.tok.Col - 1) + len([]rune(o.name))
	end := Position{Line: start.Line, Character: encodeOffset(lineText, runeCol, encoding)}
	return Range{Start: start, End: end}
}

// definitionAt resolves the identifier at pos to its declaration site,
// per fileIndex.resolve — cRust's real function-scope-only nesting, not
// a same-name text search. Returns nil if the cursor isn't on an
// identifier, or that identifier has no in-source declaration (a
// builtin or keyword, most commonly).
func definitionAt(text string, pos Position, encoding, uri string) *Location {
	idx, o := indexAndOccurrenceAt(text, pos, encoding)
	if o == nil {
		return nil
	}
	def := idx.resolve(o.name, o.scope)
	if def == nil {
		return nil
	}
	lines := splitLines(text)
	return &Location{URI: uri, Range: occurrenceRange(lines, def, encoding)}
}

// referencesAt resolves the identifier at pos to its declaration
// (exactly like definitionAt) and returns every occurrence that
// resolves to that same declaration — real binding-based references,
// so an unrelated same-named local in another function is correctly
// excluded. includeDeclaration controls whether the declaration site
// itself is included in the result, per the request's own
// ReferenceContext.
func referencesAt(text string, pos Position, encoding, uri string, includeDeclaration bool) []Location {
	idx, o := indexAndOccurrenceAt(text, pos, encoding)
	if o == nil {
		return nil
	}
	def := idx.resolve(o.name, o.scope)
	if def == nil {
		return nil
	}
	refs := idx.referencesTo(def)
	lines := splitLines(text)
	var out []Location
	for i := range refs {
		r := &refs[i]
		if !includeDeclaration && r.tok.Line == def.tok.Line && r.tok.Col == def.tok.Col {
			continue
		}
		out = append(out, Location{URI: uri, Range: occurrenceRange(lines, r, encoding)})
	}
	return out
}

// renameAt resolves the identifier at pos exactly like referencesAt
// (includeDeclaration always true — a rename has to touch the
// declaration site too) and turns every occurrence into a TextEdit
// replacing it with newName.
func renameAt(text string, pos Position, encoding, uri, newName string) *WorkspaceEdit {
	idx, o := indexAndOccurrenceAt(text, pos, encoding)
	if o == nil {
		return nil
	}
	def := idx.resolve(o.name, o.scope)
	if def == nil {
		return nil
	}
	refs := idx.referencesTo(def)
	lines := splitLines(text)
	edits := make([]TextEdit, len(refs))
	for i := range refs {
		edits[i] = TextEdit{Range: occurrenceRange(lines, &refs[i], encoding), NewText: newName}
	}
	return &WorkspaceEdit{Changes: map[string][]TextEdit{uri: edits}}
}

// documentSymbols lists every recipe declaration in text as a
// DocumentSymbol (SymbolKind.Function), in source order.
func documentSymbols(text string, encoding string) []DocumentSymbol {
	p := parser.New(lexer.New(text))
	program := p.ParseProgram()
	idx := buildIndexSafe(program)
	lines := splitLines(text)

	recipes := idx.recipes()
	out := make([]DocumentSymbol, len(recipes))
	for i := range recipes {
		r := occurrenceRange(lines, &recipes[i], encoding)
		out[i] = DocumentSymbol{Name: recipes[i].name, Kind: symbolKindFunction, Range: r, SelectionRange: r}
	}
	return out
}

// completionsAt lists keywords, builtins, and every declared name in
// text — see fileIndex.declarations' doc comment for why this is
// whole-document scoped rather than resolved against the cursor's
// actual lexical scope.
func completionsAt(text string) []CompletionItem {
	p := parser.New(lexer.New(text))
	program := p.ParseProgram()
	idx := buildIndexSafe(program)

	items := make([]CompletionItem, 0, len(keywordSpellings)+len(builtinDocs)+len(idx.occurrences))
	for _, kw := range keywordSpellings {
		items = append(items, CompletionItem{Label: kw, Kind: completionKindKeyword})
	}
	for name := range builtinDocs {
		items = append(items, CompletionItem{Label: name, Kind: completionKindFunction, Detail: "builtin"})
	}
	for _, d := range idx.declarations() {
		kind := completionKindVariable
		detail := "variable"
		if d.dk == declRecipe {
			kind = completionKindFunction
			detail = "recipe"
		} else if d.dk == declParameter {
			detail = "parameter"
		}
		items = append(items, CompletionItem{Label: d.name, Kind: kind, Detail: detail})
	}
	return items
}
