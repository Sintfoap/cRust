package debugger

import (
	"strings"
	"sync"
	"sync/atomic"

	"github.com/Sintfoap/cRust/internal/object"
	"github.com/Sintfoap/cRust/internal/trace"
)

// LiveResume is what a paused LiveTracer should do next, sent back on
// its own Resume channel.
type LiveResume int

const (
	// LiveContinue turns single-step mode off and runs until the next
	// breakpoint (or the program ends).
	LiveContinue LiveResume = iota
	// LiveStep turns single-step mode on: the very next statement,
	// breakpoint or not, pauses again.
	LiveStep
	// LiveStop abandons the run in progress. Step unwinds it via
	// panic(LiveStopSignal{}) — the only way to abort a goroutine
	// blocked deep inside someone else's call stack from the outside,
	// mirroring run.go's own top-level recover()'s "not the primary
	// error path, just an escape hatch" reasoning for a Go panic that
	// isn't a cRust runtime Error.
	LiveStop
)

// LivePause is what a paused run reports: enough for a UI to show "you
// are here" and every variable in scope, without exposing any
// interpreter-internal type across the package boundary.
type LivePause struct {
	Line  int
	Label string
	Env   map[string]object.Object
}

// LiveStopSignal is LiveTracer.Step's panic value when told to stop.
// The caller running the interpreter (cmd/crust's startLiveRun) must
// recover and check for this exact type — an ordinary Go panic here
// would otherwise look identical to a real interpreter bug.
type LiveStopSignal struct{}

// LiveTracer is a trace.Tracer that pauses a run in place — blocking
// its own Step() call, on the same goroutine the interpreter itself
// runs on — whenever the current statement's line has a breakpoint
// set, or single-step mode is active. A fresh LiveTracer starts in
// single-step mode, so the very first traced statement always pauses:
// a live debugging session should show you the very first thing about
// to run, not silently execute an unknown amount of the program before
// you get a say.
//
// Safe for concurrent use — SetBreakpoints is meant to be called from
// a UI goroutine (adding or removing a breakpoint while a run is
// paused, or even while it's mid-flight between statements) while
// Step runs on the interpreter's own goroutine.
type LiveTracer struct {
	mu          sync.Mutex
	breakpoints map[int]bool
	stepMode    bool

	// stopRequested backs RequestStop: a lock-free flag Step checks on
	// *every* call, not only while paused, so a run currently racing
	// toward a breakpoint (or one with no breakpoints ahead of it at
	// all) can still be stopped from outside — LiveStop sent on Resume
	// only reaches a Step call already blocked waiting for it, which
	// covers stopping a paused run but not one mid-flight between
	// pauses.
	stopRequested atomic.Bool

	// Paused reports every pause; Resume is how the caller says what
	// happens next. Both unbuffered: Step blocks sending on Paused
	// until something is listening, then blocks reading Resume until
	// told what to do next — that blocked state *is* "the program is
	// paused," for as long as nothing sends.
	Paused chan LivePause
	Resume chan LiveResume
}

// NewLiveTracer returns a LiveTracer with no breakpoints set, starting
// in single-step mode (see the type's own doc comment).
func NewLiveTracer() *LiveTracer {
	return &LiveTracer{
		breakpoints: map[int]bool{},
		stepMode:    true,
		Paused:      make(chan LivePause),
		Resume:      make(chan LiveResume),
	}
}

// SetBreakpoints replaces the full set of active breakpoint lines —
// copied in, not aliased, so a caller mutating its own map afterward
// (a UI's own m.liveBreakpoints, say) can never reach back in and
// change what a Step call already in flight sees.
func (t *LiveTracer) SetBreakpoints(lines map[int]bool) {
	cp := make(map[int]bool, len(lines))
	for line, on := range lines {
		if on {
			cp[line] = true
		}
	}
	t.mu.Lock()
	t.breakpoints = cp
	t.mu.Unlock()
}

// RequestStop asks the run to abandon itself at the next opportunity —
// safe to call at any time, paused or not (see stopRequested's own
// doc comment on why a mid-flight run needs this and LiveStop alone
// doesn't cover it).
func (t *LiveTracer) RequestStop() {
	t.stopRequested.Store(true)
}

// Step implements trace.Tracer, pausing (see the type doc comment)
// when the statement's line is a breakpoint or single-step mode is on.
func (t *LiveTracer) Step(e trace.StepEvent) {
	if t.stopRequested.Load() {
		panic(LiveStopSignal{})
	}

	line, label := 0, ""
	if e.Node != nil {
		line = e.Node.Pos().Line
		label = firstLine(e.Node.String())
	}

	t.mu.Lock()
	pause := t.stepMode || t.breakpoints[line]
	t.mu.Unlock()
	if !pause {
		return
	}

	t.Paused <- LivePause{Line: line, Label: label, Env: e.Env}
	switch <-t.Resume {
	case LiveStep:
		t.mu.Lock()
		t.stepMode = true
		t.mu.Unlock()
	case LiveContinue:
		t.mu.Lock()
		t.stepMode = false
		t.mu.Unlock()
	case LiveStop:
		panic(LiveStopSignal{})
	}
}

// PushFrame/PopFrame implement trace.Tracer. LiveTracer tracks no tree
// (unlike Recorder) — a live session shows one line at a time, so
// there's no frame structure for it to build or a UI to render.
func (t *LiveTracer) PushFrame(string) {}
func (t *LiveTracer) PopFrame()        {}

// firstLine mirrors Step.Label(): a statement that owns a block (a
// recipe declaration, a loop, an order) renders its *entire* body via
// String(), right for round-tripping source but wrong for a one-line
// "you are here" — this keeps only the first line and marks that
// there's more.
func firstLine(text string) string {
	if idx := strings.IndexByte(text, '\n'); idx != -1 {
		return text[:idx] + " …"
	}
	return text
}
