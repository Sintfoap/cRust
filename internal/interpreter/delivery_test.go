package interpreter

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sintfoap/cRust/internal/lexer"
	"github.com/Sintfoap/cRust/internal/object"
	"github.com/Sintfoap/cRust/internal/parser"
)

// writeDeliveryFile writes content to dir/name (creating any missing
// parent directories, for the nested-relative-import tests), returning
// its full path.
func writeDeliveryFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// evalWithBaseDir lexes/parses input and evaluates it against a fresh
// Interpreter whose BaseDir is dir — the same wiring runner.Run gives a
// real file's own directory — so a `delivery` statement in input
// resolves relative paths against dir. Returns the Interpreter (for
// inspecting env-level bindings via env.Get) and env itself, plus
// captured deliver() output and the eval result.
func evalWithBaseDir(t *testing.T, dir, input string) (*Interpreter, *object.Environment, string, object.Object) {
	t.Helper()
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parser errors for %q: %v", input, errs)
	}

	var buf bytes.Buffer
	interp := New(&buf, strings.NewReader(""))
	interp.BaseDir = dir
	env := object.NewEnvironment()
	result := interp.Eval(program, env)
	return interp, env, buf.String(), result
}

func TestDeliveryBringsRecipeIntoScope(t *testing.T) {
	dir := t.TempDir()
	writeDeliveryFile(t, dir, "helper.crust", "recipe double(x) {\n    serve x * 2\n}\n")

	_, _, out, result := evalWithBaseDir(t, dir, `delivery "helper.crust"
deliver(double(21))
`)
	if errObj, ok := result.(*object.Error); ok {
		t.Fatalf("unexpected error: %s", errObj.Message)
	}
	if out != "42\n" {
		t.Errorf("output = %q, want %q", out, "42\n")
	}
}

func TestDeliveryBringsVariableIntoScope(t *testing.T) {
	dir := t.TempDir()
	writeDeliveryFile(t, dir, "helper.crust", `greeting = "hi"`+"\n")

	_, _, out, result := evalWithBaseDir(t, dir, `delivery "helper.crust"
deliver(greeting)
`)
	if errObj, ok := result.(*object.Error); ok {
		t.Fatalf("unexpected error: %s", errObj.Message)
	}
	if out != "hi\n" {
		t.Errorf("output = %q, want %q", out, "hi\n")
	}
}

func TestDeliveryMissingFileReturnsError(t *testing.T) {
	dir := t.TempDir()

	_, _, _, result := evalWithBaseDir(t, dir, `delivery "nope.crust"`+"\n")
	errObj, ok := result.(*object.Error)
	if !ok {
		t.Fatalf("got %T, want *object.Error", result)
	}
	if !strings.Contains(errObj.Message, "nope.crust") {
		t.Errorf("message = %q, want it to mention nope.crust", errObj.Message)
	}
	if errObj.Line != 1 {
		t.Errorf("Line = %d, want 1 (the delivery statement's own line)", errObj.Line)
	}
}

func TestDeliveryParseErrorInDeliveredFileReturnsError(t *testing.T) {
	dir := t.TempDir()
	writeDeliveryFile(t, dir, "broken.crust", "x = (\n")

	_, _, _, result := evalWithBaseDir(t, dir, `delivery "broken.crust"`+"\n")
	errObj, ok := result.(*object.Error)
	if !ok {
		t.Fatalf("got %T, want *object.Error", result)
	}
	if !strings.Contains(errObj.Message, "broken.crust") || !strings.Contains(errObj.Message, "parse error") {
		t.Errorf("message = %q, want it to mention broken.crust and a parse error", errObj.Message)
	}
}

func TestDeliveryRuntimeErrorInDeliveredFileWrapsClearly(t *testing.T) {
	dir := t.TempDir()
	writeDeliveryFile(t, dir, "bad.crust", "x = 1 / 0\n")

	_, _, _, result := evalWithBaseDir(t, dir, `delivery "bad.crust"`+"\n")
	errObj, ok := result.(*object.Error)
	if !ok {
		t.Fatalf("got %T, want *object.Error", result)
	}
	if !strings.Contains(errObj.Message, "bad.crust") {
		t.Errorf("message = %q, want it to mention bad.crust", errObj.Message)
	}
}

func TestDeliverySameFileOnlyRunsOnce(t *testing.T) {
	dir := t.TempDir()
	writeDeliveryFile(t, dir, "helper.crust", `deliver("loaded")`+"\n")

	_, _, out, result := evalWithBaseDir(t, dir, `delivery "helper.crust"
delivery "helper.crust"
`)
	if errObj, ok := result.(*object.Error); ok {
		t.Fatalf("unexpected error: %s", errObj.Message)
	}
	if out != "loaded\n" {
		t.Errorf("output = %q, want %q (delivered exactly once)", out, "loaded\n")
	}
}

func TestDeliveryLaterDefinitionWinsSameScopeRule(t *testing.T) {
	dir := t.TempDir()
	writeDeliveryFile(t, dir, "helper.crust", "x = 2\n")

	_, env, _, result := evalWithBaseDir(t, dir, `x = 1
delivery "helper.crust"
`)
	if errObj, ok := result.(*object.Error); ok {
		t.Fatalf("unexpected error: %s", errObj.Message)
	}
	val, ok := env.Get("x")
	if !ok {
		t.Fatal("x not bound after delivery")
	}
	wantInteger(t, val, 2)
}

func TestDeliveryResolvesRelativeToBaseDir(t *testing.T) {
	dir := t.TempDir()
	writeDeliveryFile(t, dir, filepath.Join("sub", "helper.crust"), "x = 7\n")

	_, _, out, result := evalWithBaseDir(t, dir, `delivery "sub/helper.crust"
deliver(x)
`)
	if errObj, ok := result.(*object.Error); ok {
		t.Fatalf("unexpected error: %s", errObj.Message)
	}
	if out != "7\n" {
		t.Errorf("output = %q, want %q", out, "7\n")
	}
}

// TestDeliveryNestedRelativeImportResolvesAgainstItsOwnFile confirms a
// delivered file's own `delivery` statements resolve relative to
// *that* file's directory, not the original top-level file's — the
// same relative-imports-are-relative-to-the-current-file rule most
// module systems use. Without the BaseDir swap (evalDeliveryStatement),
// sub/a.crust's own `delivery "b.crust"` would incorrectly look for
// dir/b.crust instead of dir/sub/b.crust.
func TestDeliveryNestedRelativeImportResolvesAgainstItsOwnFile(t *testing.T) {
	dir := t.TempDir()
	writeDeliveryFile(t, dir, filepath.Join("sub", "a.crust"), `delivery "b.crust"`+"\n")
	writeDeliveryFile(t, dir, filepath.Join("sub", "b.crust"), "bval = 99\n")

	_, _, out, result := evalWithBaseDir(t, dir, `delivery "sub/a.crust"
deliver(bval)
`)
	if errObj, ok := result.(*object.Error); ok {
		t.Fatalf("unexpected error: %s", errObj.Message)
	}
	if out != "99\n" {
		t.Errorf("output = %q, want %q", out, "99\n")
	}
}

// TestDeliveryCircularImportTerminates confirms a delivers b, b
// delivers a doesn't recurse forever — a is marked delivered *before*
// its own top level runs, so b's own attempt to deliver a back finds
// it already marked and no-ops, letting both files finish defining
// their own bindings.
func TestDeliveryCircularImportTerminates(t *testing.T) {
	dir := t.TempDir()
	writeDeliveryFile(t, dir, "a.crust", "delivery \"b.crust\"\naval = 1\n")
	writeDeliveryFile(t, dir, "b.crust", "delivery \"a.crust\"\nbval = 2\n")

	src := `delivery "a.crust"
deliver(aval)
deliver(bval)
`
	l := lexer.New(src)
	p := parser.New(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parser errors: %v", errs)
	}

	var buf bytes.Buffer
	interp := New(&buf, strings.NewReader(""))
	interp.BaseDir = dir
	env := object.NewEnvironment()

	done := make(chan object.Object, 1)
	go func() { done <- interp.Eval(program, env) }()

	var result object.Object
	select {
	case result = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("circular delivery did not terminate")
	}

	if errObj, ok := result.(*object.Error); ok {
		t.Fatalf("unexpected error: %s", errObj.Message)
	}
	if got := buf.String(); got != "1\n2\n" {
		t.Errorf("output = %q, want %q", got, "1\n2\n")
	}
}

func TestDeliveryAbsolutePathWorks(t *testing.T) {
	dir := t.TempDir()
	absPath := writeDeliveryFile(t, dir, "helper.crust", "x = 5\n")

	src := "delivery \"" + strings.ReplaceAll(absPath, `\`, `\\`) + "\"\ndeliver(x)\n"
	_, _, out, result := evalWithBaseDir(t, dir, src)
	if errObj, ok := result.(*object.Error); ok {
		t.Fatalf("unexpected error: %s", errObj.Message)
	}
	if out != "5\n" {
		t.Errorf("output = %q, want %q", out, "5\n")
	}
}
