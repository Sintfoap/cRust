package object

// Null is cRust's Nil type (SPEC.md §2): `nobox`.
type Null struct{}

func (n *Null) Type() ObjectType { return NULL_OBJ }
func (n *Null) Inspect() string  { return "nobox" }

// NULL is the only Null instance that ever exists — every `nobox`
// reuses it rather than allocating.
var NULL = &Null{}
