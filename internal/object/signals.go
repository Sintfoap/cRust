package object

// ReturnValue wraps the value of a `serve` statement (SPEC.md §4) as it
// propagates up through nested blocks — evalBlockStatement stops and
// bubbles this the moment it sees one, and only a function call
// unwraps it back to the plain Value inside. Not a singleton (unlike
// Break/Continue below): each `serve` carries its own payload.
type ReturnValue struct {
	Value Object
}

func (rv *ReturnValue) Type() ObjectType { return RETURN_VALUE_OBJ }
func (rv *ReturnValue) Inspect() string  { return rv.Value.Inspect() }

// BreakSignal/ContinueSignal are what `burnt`/`flip` (SPEC.md §4)
// evaluate to — sentinel objects a loop's own evaluation catches and
// consumes, the same bubble-then-catch pattern ReturnValue uses for
// function calls. Singletons, like TRUE/FALSE/NULL: there's exactly
// one meaning each, nothing to distinguish between instances of.
type BreakSignal struct{}

func (bs *BreakSignal) Type() ObjectType { return BREAK_OBJ }
func (bs *BreakSignal) Inspect() string  { return "burnt" }

type ContinueSignal struct{}

func (cs *ContinueSignal) Type() ObjectType { return CONTINUE_OBJ }
func (cs *ContinueSignal) Inspect() string  { return "flip" }

var (
	BREAK    = &BreakSignal{}
	CONTINUE = &ContinueSignal{}
)
