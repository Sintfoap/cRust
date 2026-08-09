package interpreter

import (
	"time"

	"github.com/Sintfoap/cRust/internal/ast"
	"github.com/Sintfoap/cRust/internal/object"
	"github.com/Sintfoap/cRust/internal/trace"
)

// evalTracedStatement is evalBlockStatement's per-statement eval,
// reporting a trace.StepEvent when i.Trace is set. Timing wraps the
// call unconditionally when tracing is on — cheap relative to the Eval
// itself, and simpler than threading a "was this the traced path"
// bool back out to the caller. env.Snapshot() is taken *after* Eval
// returns, the same "after the fact" timing Out/Dur already use —
// this statement's own effect (a new assignment, a loop variable's
// binding) belongs in the snapshot the debugger's watch panel
// attaches to this step, not left for the next one to pick up.
func (i *Interpreter) evalTracedStatement(stmt ast.Statement, env *object.Environment) object.Object {
	if i.Trace == nil {
		return i.Eval(stmt, env)
	}
	start := time.Now()
	out := i.Eval(stmt, env)
	i.Trace.Step(trace.StepEvent{
		Node: stmt,
		Out:  out,
		Dur:  time.Since(start),
		Env:  env.Snapshot(),
	})
	return out
}

// evalFramed runs body as a new frame (one recipe call, or one lap of
// a knead/bake loop) when i.Trace is set, otherwise it's exactly
// i.Eval(body, env) — the same zero-cost-when-untraced shape as
// evalTracedStatement. label names the frame for the debugger UI: the
// call-site's callee text for a recipe call ("sum(...)"), or the loop
// header plus a lap number for a loop body.
func (i *Interpreter) evalFramed(label string, body *ast.BlockStatement, env *object.Environment) object.Object {
	if i.Trace == nil {
		return i.Eval(body, env)
	}
	i.Trace.PushFrame(label)
	defer i.Trace.PopFrame()
	return i.Eval(body, env)
}
