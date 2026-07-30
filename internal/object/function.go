package object

import (
	"strings"

	"github.com/Sintfoap/cRust/internal/ast"
)

// Function is cRust's Function type (SPEC.md §2): first-class, closing
// over the Environment active at its definition site (SPEC.md §3) —
// that captured Env, not the caller's, is what a call extends via
// NewEnclosedEnvironment, which is what makes closures work.
type Function struct {
	Parameters []*ast.Identifier
	Body       *ast.BlockStatement
	Env        *Environment
}

func (f *Function) Type() ObjectType { return FUNCTION_OBJ }

func (f *Function) Inspect() string {
	params := make([]string, len(f.Parameters))
	for i, p := range f.Parameters {
		params[i] = p.Value
	}

	var out strings.Builder
	out.WriteString("recipe(")
	out.WriteString(strings.Join(params, ", "))
	out.WriteString(") { ... }")
	return out.String()
}
