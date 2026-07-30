package object

import (
	"strings"
	"testing"

	"github.com/Sintfoap/cRust/internal/ast"
)

func TestFunction(t *testing.T) {
	fn := &Function{
		Parameters: []*ast.Identifier{{Value: "a"}, {Value: "b"}},
		Body:       &ast.BlockStatement{},
		Env:        NewEnvironment(),
	}

	if fn.Type() != FUNCTION_OBJ {
		t.Errorf("Type() = %v, want %v", fn.Type(), FUNCTION_OBJ)
	}

	insp := fn.Inspect()
	if !strings.Contains(insp, "recipe(") || !strings.Contains(insp, "a, b") {
		t.Errorf("Inspect() = %q, want it to mention params a, b", insp)
	}
}

func TestFunctionNoParameters(t *testing.T) {
	fn := &Function{Body: &ast.BlockStatement{}, Env: NewEnvironment()}
	if !strings.Contains(fn.Inspect(), "recipe()") {
		t.Errorf("Inspect() = %q, want recipe() with no params", fn.Inspect())
	}
}
