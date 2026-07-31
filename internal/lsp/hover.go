package lsp

import (
	"fmt"

	"github.com/Sintfoap/cRust/internal/lexer"
	"github.com/Sintfoap/cRust/internal/token"
)

// keywordDocs mirrors SPEC.md §4's keyword table — one short blurb per
// reserved word, phrased against its "standard equivalent" so hovering
// teaches the mapping instead of just repeating the keyword back.
var keywordDocs = map[token.Type]string{
	token.RECIPE:   "`recipe` — function definition (like `func`). A reusable procedure for making something.",
	token.ORDER:    "`order` — conditional (like `if`). An order comes in; it's handled if the condition holds.",
	token.COMBO:    "`combo` — chained alternative order (like `else if`).",
	token.SPECIAL:  "`special` — fallback branch (like `else`). Today's special.",
	token.KNEAD:    "`knead` — loop (like `for`), counted or for-each. Repetitive, bounded action, like kneading dough.",
	token.IN:       "`in` — for-each clause: `knead item in collection`.",
	token.BAKE:     "`bake` — conditional loop (like `while`). Keep going while the condition holds.",
	token.BURNT:    "`burnt` — loop exit (like `break`). Pull it out of the oven early.",
	token.FLIP:     "`flip` — loop continue (like `continue`). Skip the rest of this pass.",
	token.SERVE:    "`serve` — return (like `return`). Hand back the finished result.",
	token.STUFFED:  "`stuffed` — Boolean `true`. Crust is stuffed — full/true.",
	token.THIN:     "`thin` — Boolean `false`. Thin crust — empty/false.",
	token.NOBOX:    "`nobox` — nil/null. An empty pizza box: nothing inside.",
	token.TOPPINGS: "`toppings` — Set literal/type: no duplicates, no order.",
	token.WITH:     "`with` — logical AND (`&&`). \"Pepperoni **with** mushrooms.\"",
	token.OR:       "`or` — logical OR (`||`).",
	token.HOLD:     "`hold` — logical NOT (`!`). \"Hold the onions.\"",
}

// builtinDocs mirrors SPEC.md §7's standard library table.
var builtinDocs = map[string]string{
	"deliver":  "`deliver(values...)` — print: send output out.",
	"slices":   "`slices(x) -> Integer` — length/count of a String, List, Map, or Set.",
	"sauce":    "`sauce(value, fallback) -> Any` — value unless it's nobox, in which case fallback. Same job as the `?:` operator.",
	"chars":    "`chars(s) -> List` — splits a string into a List of one-character strings.",
	"idiv":     "`idiv(a, b) -> Integer` — integer (floor) division; `/` always true-divides to a Float.",
	"gather":   "`gather(list) -> Set` — collects a List into a Set, dropping duplicates.",
	"sprinkle": "`sprinkle(set, item)` — adds item to set in place.",
	"scrape":   "`scrape(set, item)` — removes item from set in place, no error if absent.",
	"topped":   "`topped(set, item) -> Boolean` — membership test: is item in set?",
	"combine":  "`combine(a, b) -> Set` — union.",
	"shared":   "`shared(a, b) -> Set` — intersection.",
	"strip":    "`strip(a, b) -> Set` — difference: items in a not in b.",
}

// hoverDoc returns hover text for tok, if any: keyword docs, builtin
// docs (only for IDENT tokens, since builtins are predeclared
// identifiers rather than reserved words — see SPEC.md §4), and a
// short type note for literal tokens. Deliberately shallow — no type
// inference over the surrounding expression, no evaluating user code.
func hoverDoc(tok token.Token) (string, bool) {
	if doc, ok := keywordDocs[tok.Type]; ok {
		return doc, true
	}
	if tok.Type == token.IDENT {
		doc, ok := builtinDocs[tok.Literal]
		return doc, ok
	}
	switch tok.Type {
	case token.INT:
		return fmt.Sprintf("Integer literal `%s`.", tok.Literal), true
	case token.FLOAT:
		return fmt.Sprintf("Float literal `%s`.", tok.Literal), true
	case token.STRING:
		return "String literal (UTF-8).", true
	}
	return "", false
}

// hoverAt lexes text fresh and finds whichever token the given
// position falls on or just after (tokens never span multiple lines in
// cRust, so this only needs to walk one line's worth of tokens), then
// looks up static hover content for it.
func hoverAt(text string, pos Position, encoding string) *hoverResult {
	lines := splitLines(text)
	var lineText string
	if pos.Line >= 0 && pos.Line < len(lines) {
		lineText = lines[pos.Line]
	}
	targetLine := pos.Line + 1
	targetCol := decodeOffset(lineText, pos.Character, encoding) + 1

	l := lexer.New(text)
	var candidate token.Token
	haveCandidate := false
	for {
		tok := l.NextToken()
		if tok.Type == token.EOF {
			break
		}
		if tok.Line > targetLine {
			break
		}
		if tok.Line != targetLine {
			continue
		}
		if tok.Col > targetCol {
			break
		}
		candidate = tok
		haveCandidate = true
	}
	if !haveCandidate {
		return nil
	}

	doc, ok := hoverDoc(candidate)
	if !ok {
		return nil
	}
	return &hoverResult{Contents: markupContent{Kind: "markdown", Value: doc}}
}
