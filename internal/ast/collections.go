package ast

import (
	"strings"

	"github.com/Sintfoap/cRust/internal/token"
)

// ListLiteral is `[expr, expr, ...]` (SPEC.md §8, listLiteral).
type ListLiteral struct {
	Token    token.Token // '['
	Elements []Expression
}

func (ll *ListLiteral) expressionNode()      {}
func (ll *ListLiteral) TokenLiteral() string { return ll.Token.Literal }
func (ll *ListLiteral) String() string {
	parts := make([]string, len(ll.Elements))
	for i, e := range ll.Elements {
		parts[i] = e.String()
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// MapPair is one `key: value` entry in a MapLiteral. Kept as an
// ordered slice on MapLiteral (not a Go map) so String() round-trips
// the source order — object.Map, by contrast, is unordered by design
// since it backs the runtime value, not the source text.
type MapPair struct {
	Key   Expression
	Value Expression
}

// MapLiteral is `{key: value, ...}` (SPEC.md §8, mapLiteral). An empty
// `{}` is a MapLiteral with no pairs, not a SetLiteral — see SetLiteral
// for how the two are told apart at parse time.
type MapLiteral struct {
	Token token.Token // '{'
	Pairs []MapPair
}

func (ml *MapLiteral) expressionNode()      {}
func (ml *MapLiteral) TokenLiteral() string { return ml.Token.Literal }
func (ml *MapLiteral) String() string {
	parts := make([]string, len(ml.Pairs))
	for i, p := range ml.Pairs {
		parts[i] = p.Key.String() + ": " + p.Value.String()
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// SetLiteral is `toppings{expr, expr, ...}` (SPEC.md §2.2) — the
// `toppings` keyword directly followed by a brace list is what
// disambiguates it from a bare MapLiteral at parse time.
type SetLiteral struct {
	Token    token.Token // TOPPINGS
	Elements []Expression
}

func (sl *SetLiteral) expressionNode()      {}
func (sl *SetLiteral) TokenLiteral() string { return sl.Token.Literal }
func (sl *SetLiteral) String() string {
	parts := make([]string, len(sl.Elements))
	for i, e := range sl.Elements {
		parts[i] = e.String()
	}
	return "toppings{" + strings.Join(parts, ", ") + "}"
}
