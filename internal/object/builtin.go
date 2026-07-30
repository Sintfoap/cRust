package object

// BuiltinFunction is the Go function signature every builtin (SPEC.md
// §7) implements. It returns an *Error (not a Go error) on misuse, so
// it propagates through Eval exactly like any other value — see
// evalBlockStatement in the interpreter package. A returned Error's
// Line/Col are left unset (0, 0); the interpreter attaches the call
// site's position when it sees one come back, so builtins don't need
// to know anything about source positions themselves.
type BuiltinFunction func(args ...Object) Object

// Builtin wraps a BuiltinFunction as an Object so it can be looked up,
// passed around, and called the same way a user-defined Function is.
type Builtin struct {
	Fn BuiltinFunction
}

func (b *Builtin) Type() ObjectType { return BUILTIN_OBJ }
func (b *Builtin) Inspect() string  { return "builtin function" }
