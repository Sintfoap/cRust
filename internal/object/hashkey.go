package object

// HashKey identifies a hashable Object for use as a Map/Set key.
// Computed once per key operation via Hashable rather than relying on
// reflection-based hashing (e.g. keying a Go map directly on Object
// values, which for an interface holding a pointer would hash by
// identity, not value — exactly wrong for cRust's value-equality Map
// keys, SPEC.md §6).
type HashKey struct {
	Type  ObjectType
	Value uint64
}

// Hashable is implemented by every Object type that can be used as a
// Map or Set key/element: Integer, Float, String, Boolean. Lists,
// Maps, and Sets are not (they're mutable, so their hash would change
// out from under a map that used one as a key — SPEC.md never proposes
// this, and it's not implemented).
//
// SPEC.md §2 additionally restricts Map keys specifically to String or
// Integer; that's a narrower rule than "hashable" and belongs to the
// evaluator (Phase 4), which is the layer that knows it's evaluating a
// mapLiteral or map index-assignment rather than a Set operation.
type Hashable interface {
	HashKey() HashKey
}
