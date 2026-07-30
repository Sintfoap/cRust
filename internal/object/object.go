// Package object defines cRust's runtime value representation — what
// an Eval'd expression actually *is* at runtime — and the Environment
// that binds names to values. See ARCHITECTURE.md's Phase 4 section
// for the design this implements.
//
// This package is being built ahead of Phase 3/4 proper, covering only
// the pieces that don't depend on the AST (which doesn't exist yet):
// the primitive and collection Object types, plus Environment. Function
// (needs ast.BlockStatement/params) and the control-flow signal types
// (ReturnValue, Error, Break/Continue — needs a decided error-value
// convention) are deliberately not here yet; they land with Phase 4
// proper.
//
// Several types here exist specifically for performance reasons agreed
// on ahead of time rather than after profiling — see ARCHITECTURE.md's
// "Performance Strategy" notes:
//   - Boolean and Null are singletons (TRUE/FALSE/NULL), never
//     allocated per-use.
//   - Integer caches small values (see NewInteger) so common loop
//     counters and arithmetic don't allocate on every operation.
//   - List/Map/Set are reference types (methods on pointer receivers,
//     backed by a slice/map) so passing them around or assigning them
//     never deep-copies — required by SPEC.md's mutate-in-place
//     builtins (sprinkle/scrape/etc.), and a performance win besides.
//   - Map/Set keys go through a precomputed HashKey rather than
//     reflection-based hashing.
package object

// ObjectType identifies an Object's runtime type. Every Eval'd
// expression produces one of these.
type ObjectType string

const (
	INTEGER_OBJ ObjectType = "INTEGER"
	FLOAT_OBJ   ObjectType = "FLOAT"
	STRING_OBJ  ObjectType = "STRING"
	BOOLEAN_OBJ ObjectType = "BOOLEAN"
	NULL_OBJ    ObjectType = "NULL"
	LIST_OBJ    ObjectType = "LIST"
	MAP_OBJ     ObjectType = "MAP"
	SET_OBJ     ObjectType = "SET"
)

// Object is any cRust runtime value. Inspect() renders it the way
// `deliver` would print it, using cRust's own vocabulary (a Boolean's
// Inspect() is "stuffed"/"thin", not "true"/"false").
type Object interface {
	Type() ObjectType
	Inspect() string
}
