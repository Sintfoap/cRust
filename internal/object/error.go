package object

// Error is a runtime error (ARCHITECTURE.md's Phase 4 section): it
// carries a message and source position and propagates through Eval
// like any other value — evalBlockStatement stops and bubbles it up
// the exact same way it does a ReturnValue/BreakSignal/ContinueSignal,
// rather than using Go panic/recover for ordinary error flow. Line/Col
// are 0 when unset (e.g. a builtin that hasn't had its call site's
// position attached yet — see the interpreter package).
type Error struct {
	Message string
	Line    int
	Col     int
}

func (e *Error) Type() ObjectType { return ERROR_OBJ }
func (e *Error) Inspect() string  { return "Error: " + e.Message }
