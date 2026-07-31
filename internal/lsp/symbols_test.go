package lsp

import (
	"testing"

	"github.com/Sintfoap/cRust/internal/lexer"
	"github.com/Sintfoap/cRust/internal/parser"
)

func indexOf(t *testing.T, src string) *fileIndex {
	t.Helper()
	p := parser.New(lexer.New(src))
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("unexpected parse errors in test source: %v", errs)
	}
	return buildIndex(program)
}

func TestBuildIndexParameterAndBodyResolveTogether(t *testing.T) {
	src := "recipe add(aParam, bParam) {\n    serve aParam + bParam\n}\n"
	idx := indexOf(t, src)

	decl := idx.resolve("aParam", 1) // scope 1 is add's body scope (0 is global)
	if decl == nil {
		t.Fatal("resolve(aParam) = nil, want the parameter declaration")
	}
	if decl.tok.Line != 1 || decl.tok.Col != 12 {
		t.Errorf("aParam decl at %d:%d, want 1:12", decl.tok.Line, decl.tok.Col)
	}
}

func TestBuildIndexShadowingAcrossFunctions(t *testing.T) {
	src := "recipe outer(x) {\n    serve x + 1\n}\n\nrecipe inner(x) {\n    serve x + 2\n}\n"
	idx := indexOf(t, src)

	// Two distinct declarations named "x", one per function's own scope.
	// referencesTo compares by pointer identity into idx.occurrences, so
	// these must stay pointers into that slice, not copies.
	var xDecls []*occurrence
	for i := range idx.occurrences {
		o := &idx.occurrences[i]
		if o.isDecl && o.name == "x" {
			xDecls = append(xDecls, o)
		}
	}
	if len(xDecls) != 2 {
		t.Fatalf("got %d declarations of x, want 2 (one per function)", len(xDecls))
	}

	outerX := xDecls[0]
	refs := idx.referencesTo(outerX)
	for _, r := range refs {
		if r.tok.Line > 3 {
			t.Errorf("reference to outer's x leaked into inner's function: %+v", r)
		}
	}
	if len(refs) != 2 { // the parameter itself + the one use in `serve x + 1`
		t.Errorf("len(refs to outer x) = %d, want 2", len(refs))
	}
}

func TestBuildIndexClosureResolvesOuterScope(t *testing.T) {
	src := "recipe makeAdder(base) {\n    recipe add(n) {\n        serve base + n\n    }\n    serve add\n}\n"
	idx := indexOf(t, src)

	// "base" inside add's body has no local declaration, so resolve
	// must climb the parent-scope chain to makeAdder's parameter.
	var baseRef *occurrence
	for i := range idx.occurrences {
		o := &idx.occurrences[i]
		if o.name == "base" && !o.isDecl {
			baseRef = o
		}
	}
	if baseRef == nil {
		t.Fatal("no reference to base found")
	}
	def := idx.resolve(baseRef.name, baseRef.scope)
	if def == nil {
		t.Fatal("resolve(base) = nil, want makeAdder's parameter declaration")
	}
	if def.dk != declParameter {
		t.Errorf("resolved base to a %v, want declParameter", def.dk)
	}
}

func TestBuildIndexRecipeDeclaredInEnclosingScope(t *testing.T) {
	src := "recipe helper(x) {\n    serve x * 2\n}\n\ntotal = helper(5)\ndeliver(total)\n"
	idx := indexOf(t, src)

	recipes := idx.recipes()
	if len(recipes) != 1 || recipes[0].name != "helper" {
		t.Fatalf("recipes() = %+v, want just helper", recipes)
	}
	// helper's own name-declaration lives in scope 0 (global), not
	// inside its own body scope.
	if recipes[0].scope != 0 {
		t.Errorf("helper declared in scope %d, want 0 (global)", recipes[0].scope)
	}
}

func TestFileIndexDeclarationsDedupesByName(t *testing.T) {
	src := "x = 1\nx = 2\ndeliver(x)\n"
	idx := indexOf(t, src)

	decls := idx.declarations()
	count := 0
	for _, d := range decls {
		if d.name == "x" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("declarations() has %d entries named x, want 1 (deduped)", count)
	}
}
