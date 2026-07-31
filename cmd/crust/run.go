package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Sintfoap/cRust/internal/ast"
	"github.com/Sintfoap/cRust/internal/interpreter"
	"github.com/Sintfoap/cRust/internal/lexer"
	"github.com/Sintfoap/cRust/internal/object"
	"github.com/Sintfoap/cRust/internal/parser"
)

// parseRunArgs pulls a file path and an optional --store=<name> out of
// args, in either order (`crust run day01.crust --store=part1` and
// `crust run --store=part1 day01.crust` both work) — Go's flag package
// can't do this on its own, since it stops parsing flags at the first
// positional argument it sees.
func parseRunArgs(args []string) (path, store string, err error) {
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "--store="):
			store = strings.TrimPrefix(a, "--store=")
		case a == "--store":
			return "", "", fmt.Errorf("--store requires a value, e.g. --store=part1")
		case strings.HasPrefix(a, "-"):
			return "", "", fmt.Errorf("unknown flag: %s", a)
		default:
			if path != "" {
				return "", "", fmt.Errorf("unexpected extra argument: %s", a)
			}
			path = a
		}
	}
	return path, store, nil
}

// runFile reads path, runs it end to end (lex, parse, Eval), and then
// resolves and calls a store/store_<name> entry point per SPEC.md §9,
// if the file defines one. storeFlag is the --store value ("" selects
// the bare `store`). A single recover() is the last-resort safety net
// for an interpreter bug (a Go panic, not a cRust runtime Error) —
// this is not the primary error-handling mechanism, which is Error
// propagating through Eval as an ordinary value; it exists purely so a
// bug here prints a message instead of a raw Go stack trace.
func runFile(path, storeFlag string, stdin io.Reader, stdout, stderr io.Writer) (code int) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(stderr, "crust run: internal error: %v\n", r)
			code = 1
		}
	}()

	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "crust run: %s\n", err)
		return 1
	}

	l := lexer.New(string(src))
	p := parser.New(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		fmt.Fprintf(stderr, "crust run: %d parse error(s):\n", len(errs))
		for _, e := range errs {
			fmt.Fprintf(stderr, "  %s\n", e)
		}
		return 1
	}

	interp := interpreter.New(stdout, stdin)
	env := object.NewEnvironment()

	result := interp.Eval(program, env)
	if errObj, ok := result.(*object.Error); ok {
		reportRuntimeError(stderr, path, errObj)
		return 1
	}

	return runEntryPoint(interp, env, program, storeFlag, stderr)
}

// runEntryPoint resolves store/store_<name> (SPEC.md §9) against the
// environment top-level evaluation just populated. A file with no
// store-family recipe at all has already run top-to-bottom by the time
// this is called, so that case is a silent, successful no-op here.
func runEntryPoint(interp *interpreter.Interpreter, env *object.Environment, program *ast.Program, storeFlag string, stderr io.Writer) int {
	target := "store"
	if storeFlag != "" {
		target = "store_" + storeFlag
	}

	fn, ok := env.Get(target)
	if !ok {
		if storeFlag != "" {
			fmt.Fprintf(stderr, "crust run: no entry point named %q (looked for recipe %s)\n", storeFlag, target)
			return 1
		}
		if names := collectEntryPoints(program); len(names) > 0 {
			fmt.Fprintf(stderr, "crust run: no default entry point; pick one: %s\n", strings.Join(storeFlags(names), ", "))
			return 1
		}
		return 0
	}

	result := interp.Call(fn, nil)
	if errObj, ok := result.(*object.Error); ok {
		reportRuntimeError(stderr, "", errObj)
		return 1
	}
	return 0
}

// collectEntryPoints scans program's top-level statements for
// store/store_<name> recipe declarations (SPEC.md §9) and returns
// their suffixes ("" for the bare `store`).
func collectEntryPoints(program *ast.Program) []string {
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

func storeFlags(suffixes []string) []string {
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

func reportRuntimeError(stderr io.Writer, path string, errObj *object.Error) {
	if path == "" {
		fmt.Fprintf(stderr, "crust run: %d:%d: %s\n", errObj.Line, errObj.Col, errObj.Message)
		return
	}
	fmt.Fprintf(stderr, "crust run: %s:%d:%d: %s\n", path, errObj.Line, errObj.Col, errObj.Message)
}
