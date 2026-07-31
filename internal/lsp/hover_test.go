package lsp

import (
	"testing"

	"github.com/Sintfoap/cRust/internal/token"
)

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
