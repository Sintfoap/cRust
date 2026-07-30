package object

import "strings"

// Set is cRust's Set type (SPEC.md §2.2): unordered, unique, mutable.
// Backed by a Go map[HashKey]Object the same way Map is, and a pointer
// type for the same reference-semantics reason as List — required for
// the mutate-in-place builtins (sprinkle/scrape, SPEC.md §7).
type Set struct {
	Elements map[HashKey]Object
}

func NewSet() *Set {
	return &Set{Elements: make(map[HashKey]Object)}
}

func (s *Set) Type() ObjectType { return SET_OBJ }

// Add inserts item into the set (a no-op if it's already present). ok
// is false if item isn't Hashable, in which case nothing changes.
func (s *Set) Add(item Object) (ok bool) {
	h, ok := item.(Hashable)
	if !ok {
		return false
	}
	s.Elements[h.HashKey()] = item
	return true
}

// Remove deletes item from the set if present; removing an absent or
// non-Hashable item is a no-op, matching SPEC.md §7's scrape().
func (s *Set) Remove(item Object) {
	h, ok := item.(Hashable)
	if !ok {
		return
	}
	delete(s.Elements, h.HashKey())
}

// Has reports whether item is in the set. Non-Hashable items are
// simply never members.
func (s *Set) Has(item Object) bool {
	h, ok := item.(Hashable)
	if !ok {
		return false
	}
	_, exists := s.Elements[h.HashKey()]
	return exists
}

func (s *Set) Len() int { return len(s.Elements) }

func (s *Set) Inspect() string {
	var out strings.Builder
	out.WriteString("toppings{")
	first := true
	for _, item := range s.Elements {
		if !first {
			out.WriteString(", ")
		}
		first = false
		out.WriteString(item.Inspect())
	}
	out.WriteByte('}')
	return out.String()
}
