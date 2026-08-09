// Package debugger implements the `crust develop` step-by-step debugger:
// a Recorder that consumes internal/trace's Tracer hook and builds a
// bounded tree of the run, plus a timing/KPI pass over that tree
// (timing.go). This design borrows its shape from a similar tool in
// another interpreter project (RFuller25/domainlang's `visualize`
// command) — see ARCHITECTURE.md's debugger section for exactly what
// carried over unchanged and what had to be redesigned for cRust's
// different (imperative, statement-based, not value-pipeline) shape.
package debugger

import (
	"fmt"
	"strings"
	"time"

	"github.com/Sintfoap/cRust/internal/ast"
	"github.com/Sintfoap/cRust/internal/object"
	"github.com/Sintfoap/cRust/internal/trace"
)

// DefaultMaxSteps is how many steps a recording keeps before it stops.
// A Domain-sized -- sorry, cRust-sized -- AoC program can loop a
// million times over a million-element list; past this cap the
// Recorder keeps running the program (correctness doesn't depend on
// recording it) but stops storing, and Truncated/Summary say so.
const DefaultMaxSteps = 20000

// Step is one recorded statement evaluation.
type Step struct {
	Index  int
	Node   ast.Statement
	Out    object.Object
	Size   int
	SizeOK bool
	Dur    time.Duration

	// Env is every variable visible at this step, straight from
	// trace.StepEvent.Env — already a snapshot taken at trace time
	// (object.Environment.Snapshot's own doc comment), so it's safe to
	// hold onto after the run finishes and read at any point later, the
	// same as every other field on Step. The Stepper tab's watch panel
	// (cmd/crust) is this field's only reader.
	Env map[string]object.Object
}

// Line is the statement's source line, or 0 if unknown (Pos wasn't
// set — shouldn't happen for anything the parser produced, but a hand-
// built AST in a test might leave it zero).
func (s *Step) Line() int {
	if s == nil {
		return 0
	}
	return s.Node.Pos().Line
}

// Label renders the step's headline: the statement's own source text,
// so the debugger reuses exactly what the user typed rather than
// inventing a second description of it. A statement that owns a block
// (a recipe declaration, a knead/bake loop, an order) renders its
// *entire* body via String() — right for round-tripping source, wrong
// for a one-line table row — so this keeps only the first line and
// marks that there's more, e.g. "recipe fib(n) {" for a whole
// function declaration.
func (s *Step) Label() string {
	text := s.Node.String()
	if idx := strings.IndexByte(text, '\n'); idx != -1 {
		return text[:idx] + " …"
	}
	return text
}

// Failed reports whether this step's result is a runtime Error.
func (s *Step) Failed() bool {
	return s.Out != nil && s.Out.Type() == object.ERROR_OBJ
}

// TraceNode is one row of the recorded tree: either a step or a frame
// (a recipe call, or one lap of a knead/bake loop body) that holds
// steps and, in turn, whatever frames those steps opened.
type TraceNode struct {
	Frame    string // set when this node is a frame
	Step     *Step  // set when this node is a step
	Children []*TraceNode

	// Folded marks the synthetic row standing in for a run of sibling
	// frames -- the laps of one loop, gathered so they can be opened as
	// a group instead of burying everything after them. See fold.
	Folded bool

	// pend holds frames closed inside this one whose owning step hasn't
	// reported yet; attached to that step when it does. See Recorder.Step.
	pend []*TraceNode
}

// IsFrame reports whether this row is a frame rather than a step.
func (n *TraceNode) IsFrame() bool { return n.Step == nil }

// Label renders the row's headline.
func (n *TraceNode) Label() string {
	if n.IsFrame() {
		return n.Frame
	}
	return n.Step.Label()
}

// Iterations reports how many frames a folded row stands for, and
// whether it is one at all.
func (n *TraceNode) Iterations() (int, bool) {
	if !n.Folded {
		return 0, false
	}
	return len(n.Children), true
}

// Recorder is a trace.Tracer that keeps a bounded tree of the run.
//
// It builds a *tree*, not a flat log: a loop's laps and a recipe call's
// body are frames a reader can step into. This needs the same ordering
// fact the run itself already guarantees -- a frame's PushFrame/
// PopFrame calls both happen *during* the step that opened it, so
// they're always fully closed by the time that step's own Step() call
// reports (evalFramed runs to completion, then evalTracedStatement's
// Step() call fires after). A closed frame is therefore held in a
// pending list at the level that opened it until the step that
// produced it reports, and that's the step whose children the frame
// becomes.
type Recorder struct {
	roots []*TraceNode

	maxSteps  int
	steps     int
	truncated bool

	stack   []*TraceNode // open frames, innermost last
	pending []*TraceNode // top-level frames awaiting their enclosing step
}

// NewRecorder returns a Recorder keeping at most maxSteps steps (0
// means DefaultMaxSteps).
func NewRecorder(maxSteps int) *Recorder {
	if maxSteps <= 0 {
		maxSteps = DefaultMaxSteps
	}
	return &Recorder{maxSteps: maxSteps}
}

// Step records one statement evaluation (trace.Tracer).
func (r *Recorder) Step(e trace.StepEvent) {
	if r.steps >= r.maxSteps {
		r.truncated = true
		return
	}
	r.steps++

	size, sizeOK := trace.SizeOf(e.Out)
	st := &Step{
		Index:  r.steps - 1,
		Node:   e.Node,
		Out:    e.Out,
		Size:   size,
		SizeOK: sizeOK,
		Dur:    e.Dur,
		Env:    e.Env,
	}

	node := &TraceNode{Step: st}
	if len(r.stack) > 0 {
		parent := r.stack[len(r.stack)-1]
		node.Children = fold(parent.pend)
		parent.pend = nil
		parent.Children = append(parent.Children, node)
		return
	}
	node.Children = fold(r.pending)
	r.pending = nil
	r.roots = append(r.roots, node)
}

// PushFrame opens a frame (trace.Tracer). label is empty when
// internal/interpreter's caller decided tracing wasn't active for this
// particular call -- can't happen through the real Tracer path (the
// interpreter only calls PushFrame at all when i.Trace is non-nil),
// but is defended here anyway rather than trusted, since a blank row
// in the stepper would be a confusing way to find out.
func (r *Recorder) PushFrame(label string) {
	if label == "" {
		label = "(frame)"
	}
	r.stack = append(r.stack, &TraceNode{Frame: label})
}

// PopFrame closes the innermost frame (trace.Tracer). The frame is
// held at the level that opened it until the step that produced it
// reports (Step adopts it via pend), since that's the row it belongs
// under.
func (r *Recorder) PopFrame() {
	if len(r.stack) == 0 {
		return
	}
	f := r.stack[len(r.stack)-1]
	r.stack = r.stack[:len(r.stack)-1]

	// Frames this one opened whose step never reported -- a call that
	// panicked or a run that hit the step cap mid-body -- are folded in
	// directly rather than lost with the level they were waiting on.
	f.Children = append(f.Children, f.pend...)
	f.pend = nil

	if len(r.stack) == 0 {
		r.pending = append(r.pending, f)
		return
	}
	parent := r.stack[len(r.stack)-1]
	parent.pend = append(parent.pend, f)
}

// foldFrom is how many sibling frames a row needs before they fold
// into one. Two laps read fine in place; a hundred bury the program.
const foldFrom = 3

// fold gathers a run of sibling frames into one collapsed row when
// there are foldFrom or more of them and nothing else is mixed in --
// the laps of one loop, the same shape over and over. The laps
// themselves are kept, not summarized away: the fold is a row that
// opens onto all of them.
func fold(children []*TraceNode) []*TraceNode {
	if len(children) < foldFrom {
		return children
	}
	for _, c := range children {
		if !c.IsFrame() {
			return children
		}
	}
	return []*TraceNode{{
		Frame:    fmt.Sprintf("%d iterations", len(children)),
		Folded:   true,
		Children: children,
	}}
}

// Roots returns the recorded top-level rows.
//
// A frame can be left in r.pending for two entirely different reasons,
// and only one of them means anything is actually missing. The
// expected one: the step cap was hit while the frame was still open,
// so the step that would have adopted it (Recorder.Step) never runs
// again — genuinely an incomplete recording, so it's surfaced under a
// synthetic row rather than silently dropped, since that's exactly the
// run a reader most wants to look at. The unremarkable one: a frame
// opened by a call that didn't happen through a traced *statement* at
// all -- `crust develop`'s own entry-point call (interp.CallNamed,
// invoked directly from Go, not from anything internal/interpreter's
// evalTracedStatement ever sees) is the one place this happens. That
// frame will *never* have an owning step to adopt it, on a perfectly
// ordinary, complete run — so treating every pending frame as
// "incomplete" would mislabel the most common `crust develop` recording
// of all: a file whose whole program lives inside its store() entry
// point.
func (r *Recorder) Roots() []*TraceNode {
	if len(r.pending) == 0 {
		return r.roots
	}
	if !r.truncated {
		out := make([]*TraceNode, len(r.roots), len(r.roots)+len(r.pending))
		copy(out, r.roots)
		return append(out, r.pending...)
	}
	orphan := &TraceNode{
		Frame:    "(incomplete — the enclosing statement did not finish)",
		Children: fold(r.pending),
	}
	out := make([]*TraceNode, len(r.roots), len(r.roots)+1)
	copy(out, r.roots)
	return append(out, orphan)
}

// Merge folds other's recorded rows into r as one new top-level frame
// node labeled label, with other's Roots() as that frame's children.
// For combining several independent runs into one Stepper tree and one
// shared Timing/KPI pass — `crust develop`'s Run tab "run all stores"
// option runs each store/store_<name> entry point as its own complete,
// independent run (its own Interpreter, environment, and stdin reader;
// see cmd/crust/debug_run.go's runAllStoresCmd), since a shared
// Interpreter would mean every store after the first sees stdin
// already drained by the one before it — Merge is how those separate
// recordings still end up displayed together afterward, without
// pretending the runs shared state they didn't. Timing.measure and
// buildRows both walk purely by tree structure (recorder.go, timing.go)
// with no notion of "one run" baked in, so a synthetic wrapper frame
// here is all merging needs — nothing downstream has to know the
// difference between this and one big recording.
func (r *Recorder) Merge(label string, other *Recorder) {
	r.roots = append(r.roots, &TraceNode{Frame: label, Children: other.Roots()})
	r.steps += other.steps
	if other.truncated {
		r.truncated = true
	}
}

// Steps reports how many steps were recorded.
func (r *Recorder) Steps() int { return r.steps }

// Truncated reports whether the step cap was reached.
func (r *Recorder) Truncated() bool { return r.truncated }

// Summary is a one-line description of the recording, for a status bar.
func (r *Recorder) Summary() string {
	if r.truncated {
		return fmt.Sprintf("%d steps (capped — the run continued past this point)", r.steps)
	}
	return fmt.Sprintf("%d steps", r.steps)
}

// family collapses a frame label down to "the same recurring thing"
// for KPI aggregation: every lap of one loop should land in a single
// bucket rather than one per lap. A recipe-call label never contains
// " lap " and passes through unchanged; a loop-lap label always does
// (evalCountedLoop/evalForEachLoop/evalBakeStatement all build theirs
// as "<header> lap <N>"), so splitting on the first occurrence strips
// exactly the lap number and nothing else.
func family(label string) string {
	if idx := strings.Index(label, " lap "); idx != -1 {
		return label[:idx]
	}
	return label
}
