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
	"deliver":    "`deliver(values...)` — print: send output out.",
	"slices":     "`slices(x) -> Integer` — length/count of a String, List, Tuple, Map, or Set.",
	"sauce":      "`sauce(value, fallback) -> Any` — value unless it's nobox, in which case fallback. Same job as the `?:` operator.",
	"chars":      "`chars(s) -> List` — splits a string into a List of one-character strings.",
	"ints":       "`ints(s) -> List` / `ints(list) -> List` — ints(s) splits a string of digits into single-digit Integers; ints(list) parses each String element as a full Integer (e.g. `ints(split(line))`).",
	"push":       "`push(list, item)` — appends item to list in place. For a new List instead of mutating, use `+`.",
	"map":        "`map(iterable, fn) -> List` — applies fn to every element of a List or Tuple, collecting the results. fn can be a recipe or another builtin.",
	"min":        "`min(a, b, ...) -> Any` / `min(list) -> Any` — smallest of 2+ arguments, or of a List/Tuple's elements. Numbers (Integer/Float mixed) or Strings only, same ordering as `<`.",
	"max":        "`max(a, b, ...) -> Any` / `max(list) -> Any` — largest of 2+ arguments, or of a List/Tuple's elements. Numbers (Integer/Float mixed) or Strings only, same ordering as `<`.",
	"combos":     "`combos(list, n) -> List` — every n-element combination of list's elements, each as a Tuple (order within a group doesn't matter, no repeats). combos(xs, 2) is every pair, combos(xs, 3) every triple, etc.",
	"grid":       "`grid(s) -> Grid` — parses s into a Grid at offset (0,0): row-major, one character per cell. Pair with at/setAt/gridBounds/neighbors4/neighbors8.",
	"newGrid":    "`newGrid() -> Grid` — an empty Grid, for building one up entirely through setAt rather than parsing one from text.",
	"at":         "`at(g, pos) -> Any` — bounds-checked read from a Grid (or a plain List of rows) at (row, col) Tuple pos; reads as nobox if out of range instead of erroring, unlike plain g[row][col].",
	"setAt":      "`setAt(g, pos, value)` — write into Grid g at pos, growing g (in any direction, including negative) to include pos if it's currently out of range. g must be a Grid (grid(s)/newGrid()), not a plain List.",
	"gridBounds": "`gridBounds(g) -> Tuple` — g's current (minRow, minCol, maxRow, maxCol), or nobox if g is empty. The only way to learn a Grid's bounds after any number of expanding setAt calls.",
	"neighbors4": "`neighbors4(pos) -> List` — the 4 orthogonal neighbor positions of (row, col) Tuple pos, each as a Tuple. No bounds checking — pair with at to filter to a real grid.",
	"neighbors8": "`neighbors8(pos) -> List` — the 8 orthogonal+diagonal neighbor positions of (row, col) Tuple pos, each as a Tuple. No bounds checking — pair with at to filter to a real grid.",
	"idiv":       "`idiv(a, b) -> Integer` — integer (floor) division; `/` always true-divides to a Float.",
	"gather":     "`gather(list) -> Set` — collects a List into a Set, dropping duplicates.",
	"sprinkle":   "`sprinkle(set, item)` — adds item to set in place.",
	"scrape":     "`scrape(set, item)` — removes item from set in place, no error if absent.",
	"topped":     "`topped(set, item) -> Boolean` — membership test: is item in set?",
	"combine":    "`combine(a, b) -> Set` — union.",
	"shared":     "`shared(a, b) -> Set` — intersection.",
	"strip":      "`strip(a, b) -> Set` — difference: items in a not in b. (For String trimming, see `trim`.)",
	"unbox":      "`unbox() -> String` / `unbox(path) -> String` — reads all of stdin, or a whole file at path.",
	"lines":      "`lines(s) -> List` — splits s into a List of lines (`\\n`/`\\r\\n`; no trailing blank entry).",
	"join":       "`join(list, sep) -> String` — joins a List of Strings with sep between each. Counterpart to `split`; elements must already be Strings (use `str()` first otherwise).",
	"split":      "`split(s) -> List` / `split(s, delim) -> List` — split(s) splits on runs of whitespace; split(s, delim) splits on the literal delim, preserving empty entries. delim can't be \"\" (use `chars`).",
	"trim":       "`trim(s) -> String` — removes leading/trailing whitespace. (For Set difference, see `strip`.)",
	"str":        "`str(x) -> String` — converts any value to its String form (same text `deliver` would print).",
	"int":        "`int(x) -> Integer` — parses a String (base-10) or truncates a Float toward zero; an Integer passes through unchanged.",
	"float":      "`float(x) -> Float` — parses a String or widens an Integer; a Float passes through unchanged.",
	"bool":       "`bool(x) -> Boolean` — normalizes any value to a strict Boolean using cRust's truthiness rule (only `thin`/`nobox` are falsy).",
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
