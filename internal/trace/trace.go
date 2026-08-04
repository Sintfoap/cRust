// Package trace defines the hook internal/interpreter reports through
// while a program runs, and internal/debugger's Recorder consumes to
// build the step-by-step debugger's data.
//
// This is deliberately a *statement*-shaped trace, not a value-pipeline
// one. Some debuggers (e.g. a dataflow language, where every construct
// is a pipeline stage with one input value and one output value) can
// report a uniform "In -> Out" per step and get "watch the data change
// shape" for free. cRust is a general imperative language: a `knead`
// loop has no value of its own, an `order` statement picks a branch
// rather than producing one, and an assignment's interesting fact is
// *which name* got a new value, not an anonymous output. So StepEvent
// carries the ast.Statement itself (Recorder/the UI can switch on its
// concrete type to describe what happened) plus whatever value Eval-ing
// it actually produced — meaningful for an assignment (the new value)
// or a `serve` (the returned value), and simply NULL for a bare `order`
// or loop header, same as Eval already returns NULL for those today.
package trace

import (
	"time"

	"github.com/Sintfoap/cRust/internal/ast"
	"github.com/Sintfoap/cRust/internal/object"
)

// StepEvent is one statement evaluation, reported after Eval returns —
// so Dur and Out are already known, the same after-the-fact timing
// every other cRust position-attaching error path already relies on
// (see internal/interpreter's newError/applyFunction).
//
// There's no separate Err field: cRust represents a runtime failure as
// an ordinary object.Object value (*object.Error, ERROR_OBJ — see
// internal/interpreter's isError), never Go's own error interface, so
// a failed statement's Out already *is* its error — check
// Out.Type() == object.ERROR_OBJ rather than a parallel field that
// could disagree with it.
//
// There's no Depth/Frame field either: unlike a flat trace log, a
// Tracer receives Step/PushFrame/PopFrame calls in the actual order
// they happen, so a tree-building Tracer (internal/debugger.Recorder)
// already knows its own current nesting from that call sequence alone
// — carrying the same information again on every StepEvent would just
// be a second, redundant way to ask "how deep am I."
type StepEvent struct {
	Node ast.Statement
	Out  object.Object
	Dur  time.Duration
}

// Tracer observes a run, statement by statement. A nil Tracer costs one
// nil check per statement (internal/interpreter's evalBlockStatement),
// so an untraced `crust run` is unaffected — mirrored by
// BenchmarkTracedVsUntraced in internal/interpreter.
//
// PushFrame/PopFrame bracket a nested sub-run: one recipe call, or one
// lap of a knead/bake loop body. Unlike a value-pipeline debugger's
// frame (which has to hand back what the frame's body produced, since
// a pass-through stage's own result is otherwise invisible), a cRust
// frame needs no value handed back: a recipe call's result already
// flows out as the enclosing CallExpression's own value, visible on
// whatever statement used it, and a loop lap's result isn't a thing
// SPEC.md gives any meaning to in the first place (knead always
// evaluates to nobox). So both methods take just a label.
type Tracer interface {
	Step(StepEvent)
	PushFrame(label string)
	PopFrame()
}
