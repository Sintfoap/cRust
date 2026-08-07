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

// runDebug shows a recording of path: the interactive stepper on a
// real terminal, or a plain indented text form otherwise (which is
// also what makes this testable, and scriptable in CI or a pipe).
//
// The two modes differ in more than presentation: --plain (or any
// non-tty stdout) runs path immediately, top-level evaluation then its
// resolved store/store_<name> entry point (the same two-phase
// execution runFile does, see run.go), under a debugger.Recorder --
// there's no interactive Run tab to defer to, so showing anything at
// all means running it now. The real terminal case starts empty
// instead (emptyDebugView): parsed, so a syntax error still surfaces
// immediately and the Run tab's entry-point list still works, but not
// executed. Only the Run tab's own "run" action (debug_run.go) ever
// actually records a run once the TUI has the screen -- running eagerly
// here, before bubbletea takes over stdin for its own keyboard input,
// used to mean any store/store_<name> recipe that reads unbox() would
// consume whatever the user typed next as puzzle input instead of it
// reaching the TUI at all, breaking keyboard input outright on the very
// first launch.
//
// Unlike `crust run`, a runtime Error from an eagerly-run --plain
// recording doesn't make this exit non-zero on its own — the whole
// point of `develop` is to look at a run including its failure, not
// just to report one. It's still shown, in place, as the step that
// produced it.
//
// applySavedStore fills in an unset --store from this file's
// remembered settings (debug_state.go) before anything else runs, so
// a --plain recording (and the TUI's Run-tab entry-point selector's
// starting position) already reflects whichever entry point was last
// used here.
//
// ensureFileExists runs first, ahead of even that: starting a new
// AoC day's file is the single most common reason to invoke `develop`
// on a path that isn't there yet, so a missing file is created (empty)
// rather than treated as an error, letting the rest of this function
// proceed exactly as it would for a file that already existed — the
// interactive TUI opens on its usual default (Time) tab, empty and
// harmless for a file with nothing recorded yet, with the Editor tab's
// nvim hand-off one tab-key away (or, --plain, an (empty, harmless)
// "0 steps" printout) instead of a dead end.
func runDebug(path string, opts debugOptions, stdin io.Reader, stdout, stderr io.Writer) int {
	if err := ensureFileExists(path, stderr); err != nil {
		fmt.Fprintf(stderr, "crust develop: %s\n", err)
		return 1
	}

	opts = applySavedStore(path, opts)

	if opts.Plain || !isColorTerminal(stdout) {
		view, err := buildDebugView(path, opts, stdin, stdout)
		if err != nil {
			fmt.Fprintf(stderr, "crust develop: %s\n", err)
			return 1
		}
		view.writePlain(stdout)
		return 0
	}

	view, err := emptyDebugView(path)
	if err != nil {
		fmt.Fprintf(stderr, "crust develop: %s\n", err)
		return 1
	}
	return runDebugTUI(view, opts, stdin, stdout, stderr)
}

// ensureFileExists creates path as an empty file when it doesn't
// exist yet, so `crust develop newday.crust` on a file that hasn't
// been written yet lands in the tool ready to type instead of just
// erroring out. Deliberately narrow: only the file itself is created,
// never a missing parent directory — a typo'd path (the wrong
// directory, not a new file to scaffold) should still fail exactly
// the way it always has, rather than silently creating directories
// nobody asked for. O_EXCL makes the existence check and the create
// atomic (no separate Stat-then-Write race), and os.IsExist on the
// resulting error is what tells "someone already put a file there"
// apart from every other reason the open could have failed (a missing
// parent directory, a permissions problem, ...) — only the latter is
// actually reported.
func ensureFileExists(path string, stderr io.Writer) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()
	fmt.Fprintf(stderr, "crust develop: %s doesn't exist yet — created it\n", path)
	return nil
}

// parseDebugFile parses path, the shared first step buildDebugView and
// emptyDebugView both need — reading the file and reporting a syntax
// error the same way for either one.
func parseDebugFile(path string) (*ast.Program, error) {
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
	return program, nil
}

// buildDebugView parses and records one real run of path — shared with
// the Editor tab's save-triggered reload (debug_editor.go's
// reloadCmd), which needs the same recording rebuilt from scratch
// after every save. progOut is where the program's own output
// (deliver, etc.) goes; --plain's eager run writes it straight to the
// real terminal since nothing owns the screen yet, but a reload
// triggered from inside an already-running TUI must not.
func buildDebugView(path string, opts debugOptions, stdin io.Reader, progOut io.Writer) (*debugView, error) {
	program, err := parseDebugFile(path)
	if err != nil {
		return nil, err
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

// emptyDebugView parses path (catching a syntax error immediately, and
// making entryPoints() work for the Run tab's selector) without
// running anything at all — the interactive TUI's starting point, so
// the very first launch never touches real process stdin the way an
// eager run would. Time/Memory/Stepper simply show nothing recorded
// yet (already-handled empty states, the same ones a genuinely
// step-free run would produce) until the Run tab's own "run" action
// builds a real recording against an explicit input file instead.
func emptyDebugView(path string) (*debugView, error) {
	if _, err := parseDebugFile(path); err != nil {
		return nil, err
	}
	return &debugView{path: path, rec: debugger.NewRecorder(0)}, nil
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
