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

// Declare binds name directly in this Environment, never walking to an
// outer scope even if name is already bound there — the opposite of
// Set's "find and mutate" rule above. This is what a recipe call's
// parameter binding needs: each call gets its own fresh enclosed
// Environment (NewEnclosedEnvironment, applyFunction) specifically so
// arguments can't collide with anything in an outer scope, and using
// Set for that binding would silently defeat the whole point of that
// isolation — a global (or any enclosing) variable that happens to
// share a parameter's name would get overwritten by the call instead
// of correctly being shadowed by it. Everywhere else that binds a name
// (knead's loop variable, plain assignment, unpack targets) does so
// directly in an existing, non-fresh scope and is supposed to walk up
// via Set — see NewEnclosedEnvironment's doc comment for why only
// recipe calls get a new Environment at all.
func (e *Environment) Declare(name string, value Object) {
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

// Snapshot returns every name currently visible from e — every scope in
// the chain, innermost shadowing outer exactly the way Get already
// resolves a lookup — flattened into one map. Built for the debugger's
// step-by-step recording (internal/debugger, internal/trace): a
// Tracer.Step call needs to capture "what were the variables at this
// exact moment," and e's own store map keeps mutating as the run
// continues past that point, so a live *Environment reference taken at
// trace time would silently show *later* values by the time anyone
// actually looks at it. Snapshot copies each scope's bindings (the map
// header only — an Object itself is never deep-copied here) so a
// caller holding the result sees this instant's bindings even after e
// itself has moved on. That does mean a later in-place mutation of a
// *shared* mutable value (a List/Map/Set/Grid still reachable through
// push/setAt/sprinkle/scrape/wrapReplace/index-assignment) is still
// visible through an old snapshot — Snapshot freezes *which value each
// name pointed to*, not a deep recursive copy of every value's own
// contents, the same "shallow enough to be cheap on every single
// traced statement" tradeoff object.DeepCopy's own callers (copy(),
// SPEC.md §7) don't have to make, since copy() only ever runs once per
// call, not once per statement.
func (e *Environment) Snapshot() map[string]Object {
	var chain []*Environment
	for env := e; env != nil; env = env.outer {
		chain = append(chain, env)
	}
	out := make(map[string]Object)
	for i := len(chain) - 1; i >= 0; i-- {
		for name, value := range chain[i].store {
			out[name] = value
		}
	}
	return out
}
