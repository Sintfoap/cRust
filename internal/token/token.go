// Package token defines cRust's token types and the pizza-jargon
// keyword table (SPEC.md §4). See ARCHITECTURE.md's Phase 2 section
// for the lexer design this feeds.
package token

// Type identifies what kind of token a Token is. Operator/delimiter
// types use their literal symbol as the value (e.g. PLUS = "+") so
// error messages can print a Type directly without a lookup table.
type Type string

// Token is one lexical unit: its kind, the exact source text it came
// from, and its 1-indexed source position for error messages.
type Token struct {
	Type    Type
	Literal string
	Line    int
	Col     int
}

const (
	ILLEGAL Type = "ILLEGAL"
	EOF     Type = "EOF"

	// Identifiers and literals.
	IDENT  Type = "IDENT"
	INT    Type = "INT"
	FLOAT  Type = "FLOAT"
	STRING Type = "STRING"

	// Keywords (SPEC.md §4).
	RECIPE   Type = "RECIPE"
	ORDER    Type = "ORDER"
	COMBO    Type = "COMBO"
	SPECIAL  Type = "SPECIAL"
	KNEAD    Type = "KNEAD"
	IN       Type = "IN"
	BAKE     Type = "BAKE"
	BURNT    Type = "BURNT"
	FLIP     Type = "FLIP"
	SERVE    Type = "SERVE"
	STUFFED  Type = "STUFFED"
	THIN     Type = "THIN"
	NOBOX    Type = "NOBOX"
	TOPPINGS Type = "TOPPINGS"
	WITH     Type = "WITH"
	OR       Type = "OR"
	HOLD     Type = "HOLD"
	DELIVERY Type = "DELIVERY"

	// Arithmetic.
	PLUS    Type = "+"
	MINUS   Type = "-"
	STAR    Type = "*"
	SLASH   Type = "/"
	PERCENT Type = "%"

	// Assignment (SPEC.md §3, §5).
	ASSIGN         Type = "="
	PLUS_ASSIGN    Type = "+="
	MINUS_ASSIGN   Type = "-="
	STAR_ASSIGN    Type = "*="
	SLASH_ASSIGN   Type = "/="
	PERCENT_ASSIGN Type = "%="

	// Increment/decrement (SPEC.md §5.2).
	INC Type = "++"
	DEC Type = "--"

	// Comparison.
	EQ     Type = "=="
	NOT_EQ Type = "!="
	LT     Type = "<"
	GT     Type = ">"
	LE     Type = "<="
	GE     Type = ">="

	// Range (SPEC.md §5.1).
	DOTDOT Type = ".."
	DOTLT  Type = ".<"

	// Ternary and Elvis (SPEC.md §5.3, §5.4).
	TERN_THEN Type = "(|"
	TERN_ELSE Type = "|)"
	ELVIS     Type = "?:"

	// Delimiters.
	COMMA     Type = ","
	COLON     Type = ":"
	SEMICOLON Type = ";"
	NEWLINE   Type = "NEWLINE"
	LPAREN    Type = "("
	RPAREN    Type = ")"
	LBRACE    Type = "{"
	RBRACE    Type = "}"
	LBRACKET  Type = "["
	RBRACKET  Type = "]"
)

// keywords is the pizza-jargon vocabulary table (SPEC.md §4). Anything
// not in this table lexes as IDENT, including every builtin function
// name (deliver, slices, sauce, ...) — builtins are predeclared
// identifiers, not reserved words, so user code can shadow them.
var keywords = map[string]Type{
	"recipe":   RECIPE,
	"order":    ORDER,
	"combo":    COMBO,
	"special":  SPECIAL,
	"knead":    KNEAD,
	"in":       IN,
	"bake":     BAKE,
	"burnt":    BURNT,
	"flip":     FLIP,
	"serve":    SERVE,
	"stuffed":  STUFFED,
	"thin":     THIN,
	"nobox":    NOBOX,
	"toppings": TOPPINGS,
	"with":     WITH,
	"or":       OR,
	"hold":     HOLD,
	"delivery": DELIVERY,
}

// LookupIdent returns the keyword Type for ident, or IDENT if it isn't
// one of the reserved words.
func LookupIdent(ident string) Type {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}

// Keywords returns the spelling -> Type table LookupIdent uses,
// for callers (internal/lsp's completion, most notably) that need
// every reserved word's actual source spelling rather than just being
// able to test one candidate string. Returns the package's own map
// directly rather than a defensive copy — internal/token has no
// mutating API for it, so there's nothing for a caller to corrupt.
func Keywords() map[string]Type {
	return keywords
}
