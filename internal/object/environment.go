package object

// Environment binds names to Objects and implements cRust's scoping
// rule (SPEC.md §3): there's no declaration keyword, so a single Set
// method has to do double duty as both "create" and "mutate."
type Environment struct {
	store map[string]Object
	outer *Environment
}

// NewEnvironment returns an empty top-level (global) Environment.
func NewEnvironment() *Environment {
	return &Environment{store: make(map[string]Object)}
}

// NewEnclosedEnvironment returns a new Environment scoped inside
// outer. Only `recipe` calls do this (SPEC.md §3) — `order`/`knead`/
// `bake` blocks evaluate directly in the enclosing scope, so the
// evaluator should *not* call this for them.
func NewEnclosedEnvironment(outer *Environment) *Environment {
	return &Environment{store: make(map[string]Object), outer: outer}
}

// Get looks up name, walking outward through enclosing scopes if it
// isn't found locally. ok is false if name is bound nowhere in the
// chain.
func (e *Environment) Get(name string) (value Object, ok bool) {
	value, ok = e.store[name]
	if !ok && e.outer != nil {
		return e.outer.Get(name)
	}
	return value, ok
}

// Set implements SPEC.md §3's assignment rule directly: if name is
// already bound anywhere in the scope chain, that binding is updated
// in place; otherwise a new binding is created in this Environment
// (the innermost enclosing recipe, or global). This one method is
// what makes "assignment with no declare keyword" work — there's no
// separate Define/Assign split, and no global/nonlocal keyword needed
// for a closure to mutate a variable it captured.
func (e *Environment) Set(name string, value Object) {
	if e.assignExisting(name, value) {
		return
	}
	e.store[name] = value
}

// assignExisting updates name's binding in place if it exists anywhere
// in the chain, reporting whether it found one.
func (e *Environment) assignExisting(name string, value Object) bool {
	if _, ok := e.store[name]; ok {
		e.store[name] = value
		return true
	}
	if e.outer != nil {
		return e.outer.assignExisting(name, value)
	}
	return false
}
