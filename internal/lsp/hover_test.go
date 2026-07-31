package lsp

import (
	"io"
	"strings"
	"testing"

	"github.com/Sintfoap/cRust/internal/builtins"
	"github.com/Sintfoap/cRust/internal/token"
)

// TestBuiltinDocsCoversEveryRealBuiltin guards against exactly the bug
// that motivated this test: builtinDocs is a hand-maintained copy of
// internal/builtins' own table, and every builtin added there since
// this package was first built (ints, push, unbox, lines, join, split,
// trim, str, int, float, bool) had silently gone undocumented in hover
// and completion until this was caught and fixed. Comparing directly
// against builtins.New()'s real key set — not another hand-maintained
// list — is what makes this catch the next one automatically.
func TestBuiltinDocsCoversEveryRealBuiltin(t *testing.T) {
	real := builtins.New(io.Discard, strings.NewReader(""), nil) // nil: this test only enumerates keys, never calls map
	for name := range real {
		if _, ok := builtinDocs[name]; !ok {
			t.Errorf("builtinDocs is missing %q — every builtins.New() entry needs a hover/completion doc", name)
		}
	}
	for name := range builtinDocs {
		if _, ok := real[name]; !ok {
			t.Errorf("builtinDocs has %q, which builtins.New() no longer registers — stale entry", name)
		}
	}
}

func TestHoverKeyword(t *testing.T) {
	text := "recipe foo() {\n}\n"
	// "recipe" starts at line 0, char 0.
	got := hoverAt(text, Position{Line: 0, Character: 2}, "utf-16")
	if got == nil {
		t.Fatal("hoverAt = nil, want a hover result for 'recipe'")
	}
	want := keywordDocs[token.RECIPE]
	if got.Contents.Value != want {
		t.Errorf("hover value = %q, want %q", got.Contents.Value, want)
	}
}

func TestHoverBuiltin(t *testing.T) {
	text := `deliver("hi")` + "\n"
	got := hoverAt(text, Position{Line: 0, Character: 3}, "utf-16")
	if got == nil {
		t.Fatal("hoverAt = nil, want a hover result for 'deliver'")
	}
	if got.Contents.Value != builtinDocs["deliver"] {
		t.Errorf("hover value = %q, want %q", got.Contents.Value, builtinDocs["deliver"])
	}
}

func TestHoverUserIdentifierNoDoc(t *testing.T) {
	text := "myVar = 1\n"
	got := hoverAt(text, Position{Line: 0, Character: 2}, "utf-16")
	if got != nil {
		t.Errorf("hoverAt on a user identifier = %+v, want nil", got)
	}
}

func TestHoverIntegerLiteral(t *testing.T) {
	text := "x = 42\n"
	got := hoverAt(text, Position{Line: 0, Character: 5}, "utf-16")
	if got == nil {
		t.Fatal("hoverAt = nil, want a hover result for '42'")
	}
	want := "Integer literal `42`."
	if got.Contents.Value != want {
		t.Errorf("hover value = %q, want %q", got.Contents.Value, want)
	}
}

func TestHoverOutOfRange(t *testing.T) {
	text := "x = 1\n"
	got := hoverAt(text, Position{Line: 50, Character: 0}, "utf-16")
	if got != nil {
		t.Errorf("hoverAt past EOF = %+v, want nil", got)
	}
}
