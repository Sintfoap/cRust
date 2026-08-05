package object

// Equal is SPEC.md §6's `==`/`!=` rule: value equality (Lists/Tuples/
// Maps/Sets/Grids compare contents, not identity; cross-category
// comparison is always false rather than an error; Integer/Float are
// one "number" category, so 1 == 1.0 is true). Lives here, not in
// internal/interpreter, so both the `==`/`!=` operator and any builtin
// that needs the exact same notion of "equal" (e.g. contains()) share
// one implementation instead of two that could quietly drift apart.
func Equal(a, b Object) bool {
	an, aIsNum := equalNumericValue(a)
	bn, bIsNum := equalNumericValue(b)
	if aIsNum && bIsNum {
		ai, aIsInt := a.(*Integer)
		bi, bIsInt := b.(*Integer)
		if aIsInt && bIsInt {
			return ai.Value == bi.Value
		}
		return an == bn
	}

	if a.Type() != b.Type() {
		return false
	}

	switch a := a.(type) {
	case *String:
		return a.Value == b.(*String).Value
	case *Boolean:
		return a.Value == b.(*Boolean).Value
	case *Null:
		return true
	case *List:
		return equalSlices(a.Elements, b.(*List).Elements)
	case *Tuple:
		return equalSlices(a.Elements, b.(*Tuple).Elements)
	case *Map:
		return equalMaps(a, b.(*Map))
	case *Set:
		return equalSets(a, b.(*Set))
	case *Grid:
		return equalGrids(a, b.(*Grid))
	case *Function:
		return a == b.(*Function)
	default:
		return false
	}
}

// equalNumericValue reports v's value as a float64 if v is an Integer
// or Float, mirroring internal/interpreter's own numericValue (kept
// separate rather than shared, since that one also serves `<`/`>`
// comparisons and has no reason to live in this package).
func equalNumericValue(obj Object) (float64, bool) {
	switch o := obj.(type) {
	case *Integer:
		return float64(o.Value), true
	case *Float:
		return o.Value, true
	default:
		return 0, false
	}
}

// equalGrids compares two Grids by their full logical layout: same
// offset (so the same coordinates mean the same cells — a shifted
// duplicate with identical *content* but a different origin is not
// the same grid) and the same rows underneath it.
func equalGrids(a, b *Grid) bool {
	if a.RowOffset != b.RowOffset || a.ColOffset != b.ColOffset {
		return false
	}
	if len(a.Rows) != len(b.Rows) {
		return false
	}
	for i := range a.Rows {
		if !equalSlices(a.Rows[i], b.Rows[i]) {
			return false
		}
	}
	return true
}

// equalSlices compares two element slices positionally — shared by
// List and Tuple equality (SPEC.md §6/§2: both compare contents, in
// order; the two types only differ in mutability, not in what "equal"
// means).
func equalSlices(a, b []Object) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !Equal(a[i], b[i]) {
			return false
		}
	}
	return true
}

func equalMaps(a, b *Map) bool {
	if len(a.Pairs) != len(b.Pairs) {
		return false
	}
	for _, pair := range a.Pairs {
		bv, ok := b.Get(pair.Key)
		if !ok || !Equal(pair.Value, bv) {
			return false
		}
	}
	return true
}

func equalSets(a, b *Set) bool {
	if a.Len() != b.Len() {
		return false
	}
	for _, item := range a.Elements {
		if !b.Has(item) {
			return false
		}
	}
	return true
}
