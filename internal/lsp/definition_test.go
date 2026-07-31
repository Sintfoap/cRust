package lsp

import (
	"strings"
	"testing"
)

// posOf finds the (occurrence)-th 0-indexed match of needle in text and
// returns it as an LSP Position — robust to the exact column math
// changing as test source gets edited, unlike hand-counted columns.
func posOf(t *testing.T, text, needle string, occurrence int) Position {
	t.Helper()
	idx, from := -1, 0
	for i := 0; i <= occurrence; i++ {
		rel := strings.Index(text[from:], needle)
		if rel < 0 {
			t.Fatalf("posOf: occurrence %d of %q not found in text", occurrence, needle)
		}
		idx = from + rel
		from = idx + 1
	}
	line := strings.Count(text[:idx], "\n")
	lineStart := strings.LastIndex(text[:idx], "\n") + 1
	col := len([]rune(text[lineStart:idx]))
	return Position{Line: line, Character: col}
}

const uri = "file:///t.crust"

func TestDefinitionAtParameterUse(t *testing.T) {
	src := "recipe add(aParam, bParam) {\n    serve aParam + bParam\n}\n"
	use := posOf(t, src, "aParam", 1)
	loc := definitionAt(src, use, "utf-16", uri)
	if loc == nil {
		t.Fatal("definitionAt = nil, want the parameter declaration")
	}
	want := posOf(t, src, "aParam", 0)
	if loc.Range.Start != want {
		t.Errorf("definition at %+v, want %+v", loc.Range.Start, want)
	}
}

func TestDefinitionAtRecipeCall(t *testing.T) {
	src := "recipe helper(x) {\n    serve x * 2\n}\n\ntotal = helper(5)\ndeliver(total)\n"
	call := posOf(t, src, "helper", 1)
	loc := definitionAt(src, call, "utf-16", uri)
	if loc == nil {
		t.Fatal("definitionAt = nil, want helper's declaration")
	}
	want := posOf(t, src, "helper", 0)
	if loc.Range.Start != want {
		t.Errorf("definition at %+v, want %+v", loc.Range.Start, want)
	}
}

func TestDefinitionAtBuiltinIsNil(t *testing.T) {
	src := `deliver("hi")` + "\n"
	loc := definitionAt(src, posOf(t, src, "deliver", 0), "utf-16", uri)
	if loc != nil {
		t.Errorf("definitionAt(deliver) = %+v, want nil (no in-source declaration)", loc)
	}
}

func TestDefinitionAtNonIdentifierIsNil(t *testing.T) {
	src := "x = 1 + 2\n"
	loc := definitionAt(src, posOf(t, src, "+", 0), "utf-16", uri)
	if loc != nil {
		t.Errorf("definitionAt('+') = %+v, want nil", loc)
	}
}

func TestReferencesAtIncludeDeclaration(t *testing.T) {
	src := "total = 1\ndeliver(total)\n"
	pos := posOf(t, src, "total", 1)

	withDecl := referencesAt(src, pos, "utf-16", uri, true)
	if len(withDecl) != 2 {
		t.Fatalf("len(withDecl) = %d, want 2", len(withDecl))
	}

	withoutDecl := referencesAt(src, pos, "utf-16", uri, false)
	if len(withoutDecl) != 1 {
		t.Fatalf("len(withoutDecl) = %d, want 1", len(withoutDecl))
	}
	if withoutDecl[0].Range.Start != posOf(t, src, "total", 1) {
		t.Errorf("withoutDecl[0] = %+v, want the reference site", withoutDecl[0])
	}
}

func TestReferencesAtExcludesOtherFunctionsShadow(t *testing.T) {
	src := "recipe outer(x) {\n    serve x + 1\n}\n\nrecipe inner(x) {\n    serve x + 2\n}\n"
	refs := referencesAt(src, posOf(t, src, "x", 0), "utf-16", uri, true)
	if len(refs) != 2 {
		t.Fatalf("len(refs) = %d, want 2 (outer's param + its use)", len(refs))
	}
	for _, r := range refs {
		if r.Range.Start.Line > 2 {
			t.Errorf("reference leaked into inner: %+v", r)
		}
	}
}

func TestRenameAtBuildsWorkspaceEdit(t *testing.T) {
	src := "x = 1\ndeliver(x)\n"
	edit := renameAt(src, posOf(t, src, "x", 0), "utf-16", uri, "count")
	if edit == nil {
		t.Fatal("renameAt = nil, want a WorkspaceEdit")
	}
	edits := edit.Changes[uri]
	if len(edits) != 2 {
		t.Fatalf("len(edits) = %d, want 2", len(edits))
	}
	for _, e := range edits {
		if e.NewText != "count" {
			t.Errorf("NewText = %q, want %q", e.NewText, "count")
		}
	}
}

func TestRenameAtNonIdentifierIsNil(t *testing.T) {
	src := "x = 1\n"
	edit := renameAt(src, Position{Line: 0, Character: 4}, "utf-16", uri, "y") // on '1'
	if edit != nil {
		t.Errorf("renameAt on a literal = %+v, want nil", edit)
	}
}

func TestDocumentSymbolsListsRecipesInOrder(t *testing.T) {
	src := "recipe first() { serve 1 }\nrecipe second() { serve 2 }\n"
	syms := documentSymbols(src, "utf-16")
	if len(syms) != 2 {
		t.Fatalf("len(syms) = %d, want 2", len(syms))
	}
	if syms[0].Name != "first" || syms[1].Name != "second" {
		t.Errorf("names = [%s, %s], want [first, second]", syms[0].Name, syms[1].Name)
	}
	for _, s := range syms {
		if s.Kind != symbolKindFunction {
			t.Errorf("%s.Kind = %d, want %d", s.Name, s.Kind, symbolKindFunction)
		}
	}
}

func TestCompletionsAtIncludesKeywordsBuiltinsAndLocals(t *testing.T) {
	src := "myVar = 5\ndeliver(myVar)\n"
	items := completionsAt(src)

	byLabel := map[string]CompletionItem{}
	for _, it := range items {
		byLabel[it.Label] = it
	}

	kw, ok := byLabel["recipe"]
	if !ok || kw.Kind != completionKindKeyword {
		t.Errorf("completions missing keyword 'recipe' with Kind=%d, got %+v", completionKindKeyword, kw)
	}
	bi, ok := byLabel["deliver"]
	if !ok || bi.Kind != completionKindFunction || bi.Detail != "builtin" {
		t.Errorf("completions missing builtin 'deliver', got %+v", bi)
	}
	v, ok := byLabel["myVar"]
	if !ok || v.Kind != completionKindVariable {
		t.Errorf("completions missing local 'myVar' with Kind=%d, got %+v", completionKindVariable, v)
	}
}
