package object

import "fmt"

// Error is a runtime error (ARCHITECTURE.md's Phase 4 section): it
// carries a message and source position and propagates through Eval
// like any other value — evalBlockStatement stops and bubbles it up
// the exact same way it does a ReturnValue/BreakSignal/ContinueSignal,
// rather than using Go panic/recover for ordinary error flow. Line/Col
// are 0 when unset (e.g. a builtin that hasn't had its call site's
// position attached yet — see the interpreter package).
//
// Frames is the call chain the error unwound through on its way back
// up, innermost call first — appended one entry per recipe call by
// internal/interpreter's applyFunction as the error bubbles through it
// (see that function's own doc comment for why this needs no separate
// call-stack bookkeeping: it falls out of Go's own call stack
// unwinding for free). nil for an error that never propagated through
// any recipe call at all (a plain top-level statement) — the ordinary
// case, and the reason FrameLines only ever adds output when there's
// real nesting to show.
type Error struct {
	Message string
	Line    int
	Col     int
	Frames  []Frame
}

// Frame is one level of an Error's call chain: the recipe that was
// entered, and where it was called from (its own caller's position —
// 0, 0 for the outermost frame, an entry point invoked directly by the
// host rather than from another call expression, e.g. `crust run`'s
// store/store_<name> resolution).
type Frame struct {
	Name string
	Line int
	Col  int
}

func (e *Error) Type() ObjectType { return ERROR_OBJ }
func (e *Error) Inspect() string  { return "Error: " + e.Message }

// maxErrorFrames caps how many levels FrameLines actually prints —
// deep but finite legitimate recursion (thousands of levels) hitting
// an error at the bottom would otherwise dump thousands of near-
// identical lines; the innermost ones (closest to the actual failure)
// are the ones worth seeing, so those are what's kept.
const maxErrorFrames = 12

// FrameLines renders e.Frames as the indented "in recipe(...), called
// from L:C" lines a caller prints after the primary "path:line:col:
// message" line — nil (not just empty) when there are fewer than two
// frames, since a single level of call wrapping (an error happening
// directly inside a `store()` entry point's own body, with no further
// nesting beneath it) adds nothing beyond what the primary line
// already says: the failing statement's own position. The outermost
// frame (no caller of its own — Line/Col both 0) prints its name alone
// with no "called from" clause.
func (e *Error) FrameLines() []string {
	if len(e.Frames) < 2 {
		return nil
	}
	frames := e.Frames
	omitted := 0
	if len(frames) > maxErrorFrames {
		omitted = len(frames) - maxErrorFrames
		frames = frames[:maxErrorFrames]
	}
	lines := make([]string, 0, len(frames)+1)
	for _, f := range frames {
		if f.Line == 0 && f.Col == 0 {
			lines = append(lines, fmt.Sprintf("    in %s", f.Name))
			continue
		}
		lines = append(lines, fmt.Sprintf("    in %s, called from %d:%d", f.Name, f.Line, f.Col))
	}
	if omitted > 0 {
		lines = append(lines, fmt.Sprintf("    ... and %d more frame(s)", omitted))
	}
	return lines
}
