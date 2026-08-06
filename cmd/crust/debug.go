// `crust develop <file.crust>` — step through a run and see where the
// time and memory actually went. Concept borrowed from a similar tool
// in another interpreter project (RFuller25/domainlang's `visualize`
// command); see ARCHITECTURE.md's debugger section for what carried
// over and what had to change for cRust's different (imperative,
// statement-based) shape.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/Sintfoap/cRust/internal/ast"
	"github.com/Sintfoap/cRust/internal/debugger"
	"github.com/Sintfoap/cRust/internal/interpreter"
	"github.com/Sintfoap/cRust/internal/lexer"
	"github.com/Sintfoap/cRust/internal/object"
	"github.com/Sintfoap/cRust/internal/parser"
)

// debugOptions are the parsed `develop` arguments.
type debugOptions struct {
	Store    string // --store=<name>: which store_<name> to run as the entry point
	MaxSteps int    // --max-steps N: capture bound (0 = the recorder's default)
	Plain    bool   // --plain: print the trace as text instead of opening the TUI
}

// parseDebugArgs parses `crust develop` arguments — the same
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
			return "", opts, fmt.Errorf("unknown flag %q for develop", a)
		default:
			if path != "" {
				return "", opts, fmt.Errorf("develop takes one program file")
			}
			path = a
		}
	}
	if path == "" {
		return "", opts, fmt.Errorf("develop needs a program file")
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
// on its own — the whole point of `develop` is to look at a run
// including its failure, not just to report one. It's still shown, in
// place, as the step that produced it.
//
// applySavedStore fills in an unset --store from this file's
// remembered settings (debug_state.go) before anything else runs, so
// the very first recording — not just the Run tab after it's re-run
// once — already reflects whichever entry point was last used here.
func runDebug(path string, opts debugOptions, stdin io.Reader, stdout, stderr io.Writer) int {
	opts = applySavedStore(path, opts)
	view, err := buildDebugView(path, opts, stdin, stdout)
	if err != nil {
		fmt.Fprintf(stderr, "crust develop: %s\n", err)
		return 1
	}
	if opts.Plain || !isColorTerminal(stdout) {
		view.writePlain(stdout)
		return 0
	}
	return runDebugTUI(view, opts, stdin, stdout, stderr)
}

// buildDebugView parses and records one run of path exactly as runDebug
// always has, without deciding how (or whether) to display it — shared
// with the TUI's Editor tab (debug_editor.go), which needs the same
// recording rebuilt from scratch after every save. progOut is where the
// program's own output (deliver, etc.) goes; the initial run writes it
// straight to the real terminal since nothing owns the screen yet, but
// a reload triggered from inside an already-running TUI must not — see
// debug_editor.go's reloadCmd.
func buildDebugView(path string, opts debugOptions, stdin io.Reader, progOut io.Writer) (*debugView, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	l := lexer.New(string(src))
	p := parser.New(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		var b strings.Builder
		fmt.Fprintf(&b, "%d parse error(s):", len(errs))
		for _, e := range errs {
			fmt.Fprintf(&b, "\n  %s", e)
		}
		return nil, errors.New(b.String())
	}

	rec := debugger.NewRecorder(opts.MaxSteps)
	interp := interpreter.New(progOut, stdin)
	interp.Trace = rec
	env := object.NewEnvironment()

	// Only resolve/call an entry point if top-level evaluation itself
	// didn't already fail -- the same order runFile follows. Either
	// way the failing step is already in rec; there's nothing more to
	// report here, since showing the recording (including its
	// failure, in place) is the whole point of this command. A failure
	// to even *find* the right entry point (below) is a different kind
	// of problem -- nothing ran at all, so there's no recording for it
	// to show in place of an error.
	if _, failed := interp.Eval(program, env).(*object.Error); !failed {
		if err := runDebugEntryPoint(interp, env, program, opts.Store); err != nil {
			return nil, err
		}
	}

	return &debugView{path: path, rec: rec}, nil
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
// exit-code plumbing (the caller doesn't need `develop`'s own exit
// code to depend on whether the program's entry point *ran and
// failed* -- the recording shows that either way, same as before).
// The "no default entry point, list what's available" case is
// different: nothing gets to run at all, so there's no recording to
// show it in place of an error the way a runtime Error gets shown --
// silently returning here used to mean a file with store_part1/
// store_part2 but no bare `store`, invoked with no --store, recorded
// nothing but the top-level recipe declarations and looked exactly
// like `develop` itself was broken rather than like "you forgot
// --store". Returning the same message run.go's runEntryPoint already
// builds (via the same collectEntryPoints/storeFlags helpers) fixes
// that without duplicating the wording.
func runDebugEntryPoint(interp *interpreter.Interpreter, env *object.Environment, program *ast.Program, storeFlag string) error {
	target := "store"
	if storeFlag != "" {
		target = "store_" + storeFlag
	}
	fn, ok := env.Get(target)
	if !ok {
		if storeFlag != "" {
			return fmt.Errorf("no entry point named %q (looked for recipe %s)", storeFlag, target)
		}
		if names := collectEntryPoints(program); len(names) > 0 {
			return fmt.Errorf("no default entry point; pick one: %s", strings.Join(storeFlags(names), ", "))
		}
		return nil
	}
	interp.CallNamed(fn, nil, target)
	return nil
}
