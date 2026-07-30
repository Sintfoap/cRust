package ast

import (
	"strings"

	"github.com/Sintfoap/cRust/internal/token"
)

// FunctionLiteral is `recipe [name](params) { body }` (SPEC.md §8,
// recipeStmt). Name is nil for an anonymous function literal used in
// expression position (`apply = recipe(x) { serve x * 2 }`) — see
// SPEC.md's note on recipeStmt's optional identifier. A non-nil Name
// is what parseStatement()'s `recipe` case treats as a declaration
// rather than a value-producing expression statement.
type FunctionLiteral struct {
	Token      token.Token // 'recipe'
	Name       *Identifier // nil if anonymous
	Parameters []*Identifier
	Body       *BlockStatement
}

func (fl *FunctionLiteral) expressionNode()      {}
func (fl *FunctionLiteral) TokenLiteral() string { return fl.Token.Literal }
func (fl *FunctionLiteral) String() string {
	params := make([]string, len(fl.Parameters))
	for i, p := range fl.Parameters {
		params[i] = p.String()
	}

	var out strings.Builder
	out.WriteString("recipe ")
	if fl.Name != nil {
		out.WriteString(fl.Name.String())
	}
	out.WriteByte('(')
	out.WriteString(strings.Join(params, ", "))
	out.WriteString(") ")
	out.WriteString(fl.Body.String())
	return out.String()
}
