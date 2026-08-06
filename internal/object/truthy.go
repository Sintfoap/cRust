package object

// IsTruthy is SPEC.md §6's rule: only `thin` (FALSE) and `nobox`
// (NULL) are falsy, everything else — including `0`, `0.0`, `""`,
// an empty List, and an empty Set — is truthy. Applied uniformly by
// order/bake/ternary conditions, `hold`, with/or's short-circuiting,
// and any builtin (e.g. find()) that needs the same notion of "does
// this count as true" a predicate function's result is judged by.
// Lives here, alongside Equal, for the same reason: both
// internal/interpreter and internal/builtins need the identical rule,
// and internal/builtins can't import internal/interpreter to reach a
// copy kept there.
func IsTruthy(obj Object) bool {
	switch obj {
	case NULL, FALSE:
		return false
	default:
		return true
	}
}
