package debugger

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/Sintfoap/cRust/internal/interpreter"
	"github.com/Sintfoap/cRust/internal/lexer"
	"github.com/Sintfoap/cRust/internal/object"
	"github.com/Sintfoap/cRust/internal/parser"
)

// runLive parses and evaluates input on its own goroutine under lt,
// the same way startLiveRun (cmd/crust) will. done receives the
// program's final result, or nil if the run was unwound by a
// LiveStop's panic(LiveStopSignal{}) — recovered here exactly the way
// the real caller must, so a stopped test run doesn't crash the test
// binary the way an unrecovered goroutine panic would.
func runLive(t *testing.T, input string, lt *LiveTracer) <-chan object.Object {
	t.Helper()
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parser errors for %q: %v", input, errs)
	}
	interp := interpreter.New(&bytes.Buffer{}, strings.NewReader(""))
	interp.Trace = lt

	done := make(chan object.Object, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(LiveStopSignal); ok {
					done <- nil
					return
				}
				panic(r)
			}
		}()
		done <- interp.Eval(program, object.NewEnvironment())
	}()
	return done
}

func mustPause(t *testing.T, lt *LiveTracer) LivePause {
	t.Helper()
	select {
	case p := <-lt.Paused:
		return p
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a pause")
		return LivePause{}
	}
}

func mustNotPause(t *testing.T, lt *LiveTracer) {
	t.Helper()
	select {
	case p := <-lt.Paused:
		t.Fatalf("got an unexpected pause at line %d (%q)", p.Line, p.Label)
	case <-time.After(100 * time.Millisecond):
	}
}

func mustFinish(t *testing.T, done <-chan object.Object) object.Object {
	t.Helper()
	select {
	case out := <-done:
		return out
	case <-time.After(2 * time.Second):
		t.Fatal("run never finished")
		return nil
	}
}

func TestLiveTracerPausesOnFirstStatement(t *testing.T) {
	lt := NewLiveTracer()
	done := runLive(t, "x = 1\ny = 2\n", lt)

	p := mustPause(t, lt)
	if p.Line != 1 {
		t.Errorf("Line = %d, want 1", p.Line)
	}
	if p.Label != "x = 1" {
		t.Errorf("Label = %q, want %q", p.Label, "x = 1")
	}

	lt.Resume <- LiveStop
	if out := mustFinish(t, done); out != nil {
		t.Errorf("result after LiveStop = %v, want nil", out)
	}
}

func TestLiveTracerStepAdvancesOneStatementAtATime(t *testing.T) {
	lt := NewLiveTracer()
	done := runLive(t, "x = 1\ny = 2\nz = 3\n", lt)

	p := mustPause(t, lt)
	if p.Line != 1 {
		t.Fatalf("first pause Line = %d, want 1", p.Line)
	}
	// Step() fires after Eval returns for that same statement (the same
	// "after the fact" timing trace.StepEvent.Env's own doc comment
	// describes), so x is already bound by the time line 1's own pause
	// reports -- this statement's *own* effect belongs in its own
	// snapshot, not the next one's.
	if xv, ok := p.Env["x"]; !ok || xv.Inspect() != "1" {
		t.Errorf("Env at line 1 x = %v, want 1", p.Env["x"])
	}
	if _, ok := p.Env["y"]; ok {
		t.Errorf("Env at line 1 already has y bound, want it unset until line 2 runs")
	}

	lt.Resume <- LiveStep
	p = mustPause(t, lt)
	if p.Line != 2 {
		t.Fatalf("second pause Line = %d, want 2", p.Line)
	}
	if xv, ok := p.Env["x"]; !ok || xv.Inspect() != "1" {
		t.Errorf("Env at line 2 x = %v, want 1", p.Env["x"])
	}

	lt.Resume <- LiveStep
	p = mustPause(t, lt)
	if p.Line != 3 {
		t.Fatalf("third pause Line = %d, want 3", p.Line)
	}
	if yv, ok := p.Env["y"]; !ok || yv.Inspect() != "2" {
		t.Errorf("Env at line 3 y = %v, want 2", p.Env["y"])
	}

	lt.Resume <- LiveStop
	mustFinish(t, done)
}

func TestLiveTracerContinueRunsToBreakpoint(t *testing.T) {
	lt := NewLiveTracer()
	done := runLive(t, "x = 1\ny = 2\nz = 3\n", lt)

	mustPause(t, lt) // line 1, the always-pause-first entry point
	lt.SetBreakpoints(map[int]bool{3: true})
	lt.Resume <- LiveContinue

	p := mustPause(t, lt)
	if p.Line != 3 {
		t.Fatalf("Line = %d, want 3 (line 2 should have run through without pausing)", p.Line)
	}
	if xv, ok := p.Env["x"]; !ok || xv.Inspect() != "1" {
		t.Errorf("Env at the breakpoint x = %v, want 1 (line 1 already ran)", p.Env["x"])
	}
	if yv, ok := p.Env["y"]; !ok || yv.Inspect() != "2" {
		t.Errorf("Env at the breakpoint y = %v, want 2 (line 2 already ran)", p.Env["y"])
	}

	lt.Resume <- LiveStop
	mustFinish(t, done)
}

func TestLiveTracerContinueWithNoBreakpointsRunsToCompletion(t *testing.T) {
	lt := NewLiveTracer()
	done := runLive(t, "x = 1\ny = 2\n", lt)

	mustPause(t, lt) // line 1
	lt.Resume <- LiveContinue

	mustNotPause(t, lt)
	mustFinish(t, done)
}

func TestLiveTracerSetBreakpointsWhilePaused(t *testing.T) {
	lt := NewLiveTracer()
	done := runLive(t, "x = 1\ny = 2\nz = 3\nw = 4\n", lt)

	mustPause(t, lt) // line 1
	lt.Resume <- LiveContinue
	// No breakpoints yet -- should run straight to completion unless
	// one gets added before it does. Adding it here, immediately after
	// sending Continue but before the run has necessarily reached line
	// 4, is exactly the "toggle a breakpoint on a run already in
	// flight" case a live UI needs to support.
	lt.SetBreakpoints(map[int]bool{4: true})

	p := mustPause(t, lt)
	if p.Line != 4 {
		t.Fatalf("Line = %d, want 4", p.Line)
	}

	lt.Resume <- LiveStop
	mustFinish(t, done)
}

func TestLiveTracerLabelTruncatesMultilineStatement(t *testing.T) {
	lt := NewLiveTracer()
	done := runLive(t, "recipe store() {\n deliver(1)\n}\n", lt)

	p := mustPause(t, lt)
	if p.Label != "recipe store() { …" {
		t.Errorf("Label = %q, want the recipe header only, marked truncated", p.Label)
	}

	lt.Resume <- LiveStop
	mustFinish(t, done)
}

func TestLiveTracerRequestStopAbandonsAMidFlightRun(t *testing.T) {
	lt := NewLiveTracer()
	done := runLive(t, "x = 1\ny = 2\nz = 3\nw = 4\n", lt)

	mustPause(t, lt) // line 1
	lt.Resume <- LiveContinue
	// Nothing paused it again yet -- it's racing toward completion (no
	// breakpoints set). RequestStop must still cut it off before it
	// gets there, unlike LiveStop sent on Resume (which only reaches a
	// Step call already blocked waiting for it).
	lt.RequestStop()

	out := mustFinish(t, done)
	if out != nil {
		t.Errorf("result after RequestStop = %v, want nil (the run should never have reached its final statement)", out)
	}
}

func TestLiveTracerRequestStopWhilePaused(t *testing.T) {
	lt := NewLiveTracer()
	done := runLive(t, "x = 1\ny = 2\n", lt)

	mustPause(t, lt) // line 1
	lt.RequestStop()
	lt.Resume <- LiveContinue // any value unblocks Step, which then re-checks stopRequested on its very next call

	mustFinish(t, done)
}

func TestLiveTracerSetBreakpointsCopiesInput(t *testing.T) {
	lt := NewLiveTracer()
	lines := map[int]bool{2: true}
	lt.SetBreakpoints(lines)
	delete(lines, 2) // mutating the caller's own map afterward must not un-set it on lt

	done := runLive(t, "x = 1\ny = 2\n", lt)
	mustPause(t, lt) // line 1
	lt.Resume <- LiveContinue

	p := mustPause(t, lt)
	if p.Line != 2 {
		t.Fatalf("Line = %d, want 2 (breakpoint should be unaffected by the caller mutating its own map afterward)", p.Line)
	}

	lt.Resume <- LiveStop
	mustFinish(t, done)
}
