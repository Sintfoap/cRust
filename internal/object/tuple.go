package object

import (
	"encoding/binary"
	"hash/fnv"
	"strings"
)

// Tuple is cRust's Tuple type (SPEC.md §2): a fixed-size, immutable,
// heterogeneous sequence. Immutability is what makes it safe to hash
// (see HashKey) — a mutable value's hash could change out from under a
// Map/Set that used it as a key, exactly why List doesn't implement
// Hashable. Reference type (pointer receiver), like List/Map/Set, for
// the same reason: assigning/passing/storing a Tuple shouldn't copy
// its backing slice, even though nothing can mutate it through that
// shared reference. internal/interpreter's evalTupleLiteral guarantees
// every element is itself Hashable before a Tuple is ever constructed
// (mirroring how it already guarantees a Set literal's elements are),
// so HashKey below never has to handle a non-Hashable element.
type Tuple struct {
	Elements []Object
}

func NewTuple(elements []Object) *Tuple {
	return &Tuple{Elements: elements}
}

func (t *Tuple) Type() ObjectType { return TUPLE_OBJ }

func (t *Tuple) Inspect() string {
	var out strings.Builder
	out.WriteByte('(')
	for i, e := range t.Elements {
		if i > 0 {
			out.WriteString(", ")
		}
		out.WriteString(e.Inspect())
	}
	out.WriteByte(')')
	return out.String()
}

// HashKey combines every element's own HashKey (both its ObjectType
// and Value, so e.g. an Integer and a same-Value-but-different-type
// element can't blend together) into one, so two Tuples with equal
// elements in the same order hash equal — the same value-equality
// contract every other Hashable type here upholds.
func (t *Tuple) HashKey() HashKey {
	h := fnv.New64a()
	var buf [8]byte
	for _, e := range t.Elements {
		eh := e.(Hashable).HashKey()
		h.Write([]byte(eh.Type))
		binary.LittleEndian.PutUint64(buf[:], eh.Value)
		h.Write(buf[:])
	}
	return HashKey{Type: TUPLE_OBJ, Value: h.Sum64()}
}
