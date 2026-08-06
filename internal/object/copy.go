package object

// DeepCopy returns an independent copy of obj: for the four mutable
// container types (List, Map, Set, Grid), every level of nesting is
// duplicated, so mutating the copy (push, setAt, index assignment,
// sprinkle/scrape, ...) can never be observed through the original,
// or the other way around. Every other type comes back unchanged:
// Integer/Float/String/Boolean/Null are immutable value-like objects
// with nothing to copy, Function/Builtin/Error aren't SPEC.md's notion
// of a mutable value at all, and Tuple's own Hashable-element
// requirement (SPEC.md §2.3 — enforced wherever a Tuple is built,
// hashkey.go) already guarantees its contents can never include a
// mutable container in the first place (List/Map/Set/Grid don't
// implement Hashable), so a Tuple is already, by construction, as
// independent as a copy of it could ever be.
//
// Set and Map don't need their own keys/elements copied for the same
// reason a Tuple doesn't: both require Hashable members, which rules
// out anything mutable living there. Only List/Map's-values/Grid's-
// cells can hold arbitrary Objects (including another List/Map/Set/
// Grid), which is where the actual recursion happens.
func DeepCopy(obj Object) Object {
	return deepCopy(obj, map[Object]Object{})
}

// deepCopy does the real work. seen carries original-container ->
// its-already-built-copy across the whole call: a container is
// registered the moment it's allocated, before its contents are
// filled in, so a self-referential structure (e.g. a List that gets
// pushed onto itself) reuses that in-progress copy instead of
// recursing forever, and two independent references to the same
// original container end up pointing at the same new copy too,
// preserving whatever internal sharing the original had.
func deepCopy(obj Object, seen map[Object]Object) Object {
	if existing, ok := seen[obj]; ok {
		return existing
	}

	switch v := obj.(type) {
	case *List:
		out := &List{Elements: make([]Object, len(v.Elements))}
		seen[obj] = out
		for i, e := range v.Elements {
			out.Elements[i] = deepCopy(e, seen)
		}
		return out
	case *Map:
		out := NewMap()
		seen[obj] = out
		for _, pair := range v.Pairs {
			out.Set(pair.Key, deepCopy(pair.Value, seen))
		}
		return out
	case *Set:
		out := NewSet()
		seen[obj] = out
		for _, item := range v.Elements {
			out.Add(item)
		}
		return out
	case *Grid:
		out := &Grid{Rows: make([][]Object, len(v.Rows)), RowOffset: v.RowOffset, ColOffset: v.ColOffset}
		seen[obj] = out
		for i, row := range v.Rows {
			newRow := make([]Object, len(row))
			for j, cell := range row {
				newRow[j] = deepCopy(cell, seen)
			}
			out.Rows[i] = newRow
		}
		return out
	default:
		return obj
	}
}
