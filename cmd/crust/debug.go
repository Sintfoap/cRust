// `crust debug <file.crust>` — step through a run and see where the
// time and memory actually went. Concept borrowed from a similar tool
// in another interpreter project (RFuller25/domainlang's `visualize`
// command); see ARCHITECTURE.md's debugger section for what carried
// over and what had to change for cRust's different (imperative,
// statement-based) shape.
package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/Sintfoap/cRust/internal/debugger"
	"github.com/Sintfoap/cRust/internal/interpreter"
	"github.com/Sintfoap/cRust/internal/lexer"
	"github.com/Sintfoap/cRust/internal/object"
	"github.com/Sintfoap/cRust/internal/parser"
)

// debugOptions are the parsed `debug` arguments.
type debugOptions struct {
	Store    string // --store=<name>: which store_<name> to run as the entry point
	MaxSteps int    // --max-steps N: capture bound (0 = the recorder's default)
	Plain    bool   // --plain: print the trace as text instead of opening the TUI
}

// parseDebugArgs parses `crust debug` arguments — the same
// any-order-flags-then-one-path shape parseRunArgs already uses, plus
// the two flags specific to debugging a recording rather than just
// running the program.
func parseDebugArgs(args []string) (string, debugOptions, error) {
	var opts debugOptions
	var path string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--plain":
			opts.Plain = true
		case strings.HasPrefix(a, "--store="):
			opts.Store = strings.TrimPrefix(a, "--store=")
		case a == "--store":
			return "", opts, fmt.Errorf("--store requires a value, e.g. --store=part1")
		case strings.HasPrefix(a, "--max-steps="):
			n, err := strconv.Atoi(strings.TrimPrefix(a, "--max-steps="))
			if err != nil || n <= 0 {
				return "", opts, fmt.Errorf("--max-steps needs a positive number")
			}
			opts.MaxSteps = n
		case a == "--max-steps":
			if i+1 >= len(args) {
				return "", opts, fmt.Errorf("--max-steps needs a number")
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n <= 0 {
				return "", opts, fmt.Errorf("--max-steps needs a positive number, got %q", args[i])
			}
			opts.MaxSteps = n
		case strings.HasPrefix(a, "-"):
			return "", opts, fmt.Errorf("unknown flag %q for debug", a)
		default:
			if path != "" {
				return "", opts, fmt.Errorf("debug takes one program file")
			}
			path = a
		}
	}
	if path == "" {
		return "", opts, fmt.Errorf("debug needs a program file")
	}
	return path, opts, nil
}

// runDebug records a run of path (top-level evaluation, then its
// resolved store/store_<name> entry point — the same two-phase
// execution runFile does, see run.go) under a debugger.Recorder, then
// shows the recording: the interactive stepper on a real terminal, or
// a plain indented text form otherwise (which is also what makes this
// testable, and scriptable in CI or a pipe).
//
// Unlike `crust run`, a runtime Error doesn't make this exit non-zero
// on its own — the whole point of `debug` is to look at a run
// including its failure, not just to report one. It's still shown, in
// place, as the step that produced it.
func runDebug(path string, opts debugOptions, stdin io.Reader, stdout, stderr io.Writer) int {
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "crust debug: %s\n", err)
		return 1
	}

	l := lexer.New(string(src))
	p := parser.New(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		fmt.Fprintf(stderr, "crust debug: %d parse error(s):\n", len(errs))
		for _, e := range errs {
			fmt.Fprintf(stderr, "  %s\n", e)
		}
		return 1
	}

	rec := debugger.NewRecorder(opts.MaxSteps)
	interp := interpreter.New(stdout, stdin)
	interp.Trace = rec
	env := object.NewEnvironment()

	// Only resolve/call an entry point if top-level evaluation itself
	// didn't already fail -- the same order runFile follows. Either
	// way the failing step is already in rec; there's nothing more to
	// report here, since showing the recording (including its
	// failure, in place) is the whole point of this command.
	if _, failed := interp.Eval(program, env).(*object.Error); !failed {
		runDebugEntryPoint(interp, env, opts.Store)
	}

	view := &debugView{path: path, rec: rec}
	if opts.Plain || !isColorTerminal(stdout) {
		view.writePlain(stdout)
		return 0
	}
	return runDebugTUI(view, stdin, stdout, stderr)
}

// isColorTerminal reports whether w is an interactive terminal worth
// opening the TUI on, generalizing main.go's isTerminal (which needs a
// concrete *os.File) to the io.Writer every command here is actually
// handed — stdout is only ever a real terminal when it's genuinely
// os.Stdout itself, not some other io.Writer a test or a pipe supplied.
func isColorTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isTerminal(f)
}

// runDebugEntryPoint mirrors run.go's runEntryPoint, minus the
// exit-code/error-reporting plumbing (the caller doesn't need
// `debug`'s own exit code to depend on whether the program's entry
// point failed -- the recording shows that either way) and minus its
// "no default entry point, list what's available" message, which is
// about guiding a `crust run` invocation, not something `debug` needs
// to duplicate.
func runDebugEntryPoint(interp *interpreter.Interpreter, env *object.Environment, storeFlag string) {
	target := "store"
	if storeFlag != "" {
		target = "store_" + storeFlag
	}
	fn, ok := env.Get(target)
	if !ok {
		return
	}
	interp.Call(fn, nil)
}
