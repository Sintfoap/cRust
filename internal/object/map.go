package object

import "strings"

// MapPair keeps the original key Object alongside its value — the map
// underneath is keyed by HashKey, which is lossy (e.g. two different
// String values never collide, but the HashKey alone can't reproduce
// the original string), so Inspect() and iteration need the real key
// object, not just its hash.
type MapPair struct {
	Key   Object
	Value Object
}

// Map is cRust's Map type (SPEC.md §2): mutable, keyed by String or
// Integer (that restriction is enforced by the evaluator, not here —
// see hashkey.go). A pointer to a struct wrapping a Go map, for the
// same reference-semantics reason as List.
type Map struct {
	Pairs map[HashKey]MapPair
}

func NewMap() *Map {
	return &Map{Pairs: make(map[HashKey]MapPair)}
}

func (m *Map) Type() ObjectType { return MAP_OBJ }

// Set stores key => value, keyed by the key's HashKey. ok is false if
// key isn't Hashable, in which case nothing is stored — the evaluator
// (which has source position info this package doesn't) is responsible
// for turning that into a proper runtime error.
func (m *Map) Set(key, value Object) (ok bool) {
	h, ok := key.(Hashable)
	if !ok {
		return false
	}
	m.Pairs[h.HashKey()] = MapPair{Key: key, Value: value}
	return true
}

// Get returns the value for key and whether it was present. ok is also
// false if key isn't Hashable.
func (m *Map) Get(key Object) (value Object, ok bool) {
	h, ok := key.(Hashable)
	if !ok {
		return nil, false
	}
	pair, ok := m.Pairs[h.HashKey()]
	if !ok {
		return nil, false
	}
	return pair.Value, true
}

func (m *Map) Inspect() string {
	var out strings.Builder
	out.WriteByte('{')
	first := true
	for _, pair := range m.Pairs {
		if !first {
			out.WriteString(", ")
		}
		first = false
		out.WriteString(pair.Key.Inspect())
		out.WriteString(": ")
		out.WriteString(pair.Value.Inspect())
	}
	out.WriteByte('}')
	return out.String()
}
