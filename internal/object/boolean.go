package object

// Boolean is cRust's Boolean type (SPEC.md §2): `stuffed`/`thin`.
type Boolean struct {
	Value bool
}

func (b *Boolean) Type() ObjectType { return BOOLEAN_OBJ }

func (b *Boolean) Inspect() string {
	if b.Value {
		return "stuffed"
	}
	return "thin"
}

func (b *Boolean) HashKey() HashKey {
	var v uint64
	if b.Value {
		v = 1
	}
	return HashKey{Type: BOOLEAN_OBJ, Value: v}
}

// TRUE and FALSE are the only two Boolean instances that ever exist.
// Every `stuffed`/`thin` literal and every comparison result reuses
// one of these instead of allocating — safe for the same reason
// Integer's small-value cache is (SPEC.md gives Booleans no mutable
// state), and unconditionally worthwhile since there are only two
// possible values to begin with.
var (
	TRUE  = &Boolean{Value: true}
	FALSE = &Boolean{Value: false}
)

// NativeBoolToBooleanObject returns TRUE or FALSE for a Go bool,
// rather than allocating a new Boolean.
func NativeBoolToBooleanObject(input bool) *Boolean {
	if input {
		return TRUE
	}
	return FALSE
}
