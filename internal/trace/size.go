package trace

import "github.com/Sintfoap/cRust/internal/object"

// SizeOf reports how much data v holds, and whether that's a
// meaningful question at all — the same "not everything has a size"
// split cRust's other reflective builtins draw (e.g. slices(x)
// errors on a scalar rather than inventing an answer). A String
// reports its rune count (matching slices(s), not len(s)'s byte
// count); a List/Tuple/Set/Map reports its element/pair count. A
// scalar (Integer, Float, Boolean, nobox) reports nothing — "1" isn't
// a size — and neither does a Function or Builtin, which don't hold
// data so much as behavior.
func SizeOf(v object.Object) (int, bool) {
	switch x := v.(type) {
	case *object.String:
		return len([]rune(x.Value)), true
	case *object.List:
		return len(x.Elements), true
	case *object.Tuple:
		return len(x.Elements), true
	case *object.Set:
		return x.Len(), true
	case *object.Map:
		return len(x.Pairs), true
	default:
		return 0, false
	}
}
