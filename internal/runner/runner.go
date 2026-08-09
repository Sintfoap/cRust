// Package runner holds the "run a whole cRust program" pipeline shared
// by every frontend that needs it verbatim: the crust CLI's `run`
// subcommand, `develop`'s Run tab, and the WASM build behind `crust bake
// playground`. It owns lex → parse → Eval → resolve-and-call-entry-point
// (SPEC.md §9) so those frontends can't drift from each other on edge
// cases like "no default entry point, multiple named ones exist".
package runner

import (
	"fmt"
	"io"
	"strings"

	"github.com/Sintfoap/cRust/internal/ast"
	"github.com/Sintfoap/cRust/internal/interpreter"
	"github.com/Sintfoap/cRust/internal/lexer"
	"github.com/Sintfoap/cRust/internal/object"
	"github.com/Sintfoap/cRust/internal/parser"
)

// Run lexes, parses, and evaluates src (top-level code runs first, same
// as Python module-level code), then resolves and calls a
// store/store_<name> entry point per SPEC.md §9, if src defines one.
// storeFlag is the --store value ("" selects the bare `store`). path is
// used only to prefix runtime error locations that come from top-level
// evaluation ("" omits it — the REPL and playground have no backing
// file). Every message this writes to stderr is prefixed with msgPrefix
// verbatim, so each caller keeps its own framing (`crust run: `, or
// none at all) without this package hardcoding one. Returns 0 on
// success, 1 on any parse/runtime/entry-point error. A single recover()
// is the last-resort safety net for an interpreter bug (a Go panic, not
// a cRust runtime Error, which instead propagates through Eval as an
// ordinary value) — it exists purely so a bug here prints a message
// instead of a raw Go stack trace.
func Run(src, storeFlag, path, msgPrefix string, stdin io.Reader, stdout, stderr io.Writer) (code int) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(stderr, "%sinternal error: %v\n", msgPrefix, r)
			code = 1
		}
	}()

	l := lexer.New(src)
	p := parser.New(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		fmt.Fprintf(stderr, "%s%d parse error(s):\n", msgPrefix, len(errs))
		for _, e := range errs {
			fmt.Fprintf(stderr, "  %s\n", e)
		}
		return 1
	}

	interp := interpreter.New(stdout, stdin)
	env := object.NewEnvironment()

	result := interp.Eval(program, env)
	if errObj, ok := result.(*object.Error); ok {
		reportRuntimeError(stderr, msgPrefix, path, errObj)
		return 1
	}

	return runEntryPoint(interp, env, program, storeFlag, msgPrefix, stderr)
}

// runEntryPoint resolves store/store_<name> (SPEC.md §9) against the
// environment top-level evaluation just populated. A file with no
// store-family recipe at all has already run top-to-bottom by the time
// this is called, so that case is a silent, successful no-op here.
func runEntryPoint(interp *interpreter.Interpreter, env *object.Environment, program *ast.Program, storeFlag, msgPrefix string, stderr io.Writer) int {
	target := "store"
	if storeFlag != "" {
		target = "store_" + storeFlag
	}

	fn, ok := env.Get(target)
	if !ok {
		if storeFlag != "" {
			fmt.Fprintf(stderr, "%sno entry point named %q (looked for recipe %s)\n", msgPrefix, storeFlag, target)
			return 1
		}
		if names := CollectEntryPoints(program); len(names) > 0 {
			fmt.Fprintf(stderr, "%sno default entry point; pick one: %s\n", msgPrefix, strings.Join(StoreFlags(names), ", "))
			return 1
		}
		return 0
	}

	result := interp.CallNamed(fn, nil, target)
	if errObj, ok := result.(*object.Error); ok {
		reportRuntimeError(stderr, msgPrefix, "", errObj)
		return 1
	}
	return 0
}

// CollectEntryPoints scans program's top-level statements for
// store/store_<name> recipe declarations (SPEC.md §9) and returns
// their suffixes ("" for the bare `store`).
func CollectEntryPoints(program *ast.Program) []string {
	var names []string
	for _, stmt := range program.Statements {
		es, ok := stmt.(*ast.ExpressionStatement)
		if !ok {
			continue
		}
		fl, ok := es.Expression.(*ast.FunctionLiteral)
		if !ok || fl.Name == nil {
			continue
		}
		switch {
		case fl.Name.Value == "store":
			names = append(names, "")
		case strings.HasPrefix(fl.Name.Value, "store_"):
			names = append(names, strings.TrimPrefix(fl.Name.Value, "store_"))
		}
	}
	return names
}

// StoreFlags formats entry-point suffixes (as returned by
// CollectEntryPoints) the way error messages list them: "(default)"
// for the bare `store`, "--store=<name>" for everything else.
func StoreFlags(suffixes []string) []string {
	out := make([]string, len(suffixes))
	for i, s := range suffixes {
		if s == "" {
			out[i] = "(default)"
		} else {
			out[i] = "--store=" + s
		}
	}
	return out
}

// reportRuntimeError prints errObj's primary "path:line:col: message"
// line exactly as before, plus — when the error unwound through more
// than one recipe call — an indented call chain underneath it
// (errObj.FrameLines' own doc comment on exactly when that's nil vs.
// populated), innermost call first, so "which recipe, called from
// where" is visible without reaching for `crust develop`.
func reportRuntimeError(stderr io.Writer, msgPrefix, path string, errObj *object.Error) {
	if path == "" {
		fmt.Fprintf(stderr, "%s%d:%d: %s\n", msgPrefix, errObj.Line, errObj.Col, errObj.Message)
	} else {
		fmt.Fprintf(stderr, "%s%s:%d:%d: %s\n", msgPrefix, path, errObj.Line, errObj.Col, errObj.Message)
	}
	for _, line := range errObj.FrameLines() {
		fmt.Fprintln(stderr, line)
	}
}
