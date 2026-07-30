package object

import "strings"

// List is cRust's List type (SPEC.md §2): ordered, heterogeneous,
// mutable. Represented as a pointer to a struct wrapping a slice —
// not a value type — specifically so that assigning a List to a
// variable, passing it as a function argument, or storing it in
// another collection never deep-copies it. That's required for
// correctness (SPEC.md's List is a reference type: two variables can
// point at "the same" list and see each other's mutations) as much as
// it is for performance.
type List struct {
	Elements []Object
}

func NewList(elements []Object) *List {
	return &List{Elements: elements}
}

func (l *List) Type() ObjectType { return LIST_OBJ }

func (l *List) Inspect() string {
	var out strings.Builder
	out.WriteByte('[')
	for i, e := range l.Elements {
		if i > 0 {
			out.WriteString(", ")
		}
		out.WriteString(e.Inspect())
	}
	out.WriteByte(']')
	return out.String()
}
