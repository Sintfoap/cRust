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
// Map or Set key/element: Integer, Float, String, Boolean, Tuple.
// Lists, Maps, and Sets are not (they're mutable, so their hash would
// change out from under a map that used one as a key) — Tuple is
// exactly List's immutable counterpart, added specifically so
// fixed-size groups (grid coordinates, most usefully) can be hashed
// the way a List never safely could.
//
// Map and Set agree on exactly this rule — "any Hashable value" — with
// no narrower allowlist for either one (see isValidMapKey,
// internal/interpreter/expressions.go): a Map that accepted a smaller
// set of key types than Set accepts as elements would be a surprising
// asymmetry given both are backed by this same interface.
type Hashable interface {
	HashKey() HashKey
}
