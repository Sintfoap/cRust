package interpreter

import (
	"fmt"
	"strings"

	"github.com/Sintfoap/cRust/internal/ast"
	"github.com/Sintfoap/cRust/internal/object"
)

// evalProgram evaluates every top-level statement in order. Unlike
// evalBlockStatement, it unwraps a bubbled-up ReturnValue immediately
// (a bare top-level `serve` isn't restricted to recipe bodies by
// SPEC.md, so this treats it as an early, whole-program exit rather
// than an error) and halts on the first Error, matching TODO.md's
// "runtime error handling" — one bad statement stops the run rather
// than continuing past it.
func (i *Interpreter) evalProgram(program *ast.Program, env *object.Environment) object.Object {
	var result object.Object = object.NULL

	for _, stmt := range program.Statements {
		result = i.evalTracedStatement(stmt, env)

		switch result := result.(type) {
		case *object.ReturnValue:
			return result.Value
		case *object.Error:
			return result
		}
	}

	return result
}

// evalBlockStatement runs a block's statements in the given env — the
// same Environment as the caller for order/knead/bake bodies (SPEC.md
// §3: those don't introduce a scope), or a freshly enclosed one for a
// recipe's body (see applyFunction). The moment a statement evaluates
// to a control-flow signal (Error, ReturnValue, BreakSignal,
// ContinueSignal), evaluation stops and that signal bubbles up
// unchanged — it's the caller's job to decide whether to catch it
// (a loop catches Break/Continue, a function call catches ReturnValue)
// or keep bubbling it (everything else, including plain nested
// blocks). Completing normally, with no signal hit, evaluates to NULL
// — cRust has no implicit last-statement-as-value, unlike Rust/Ruby.
func (i *Interpreter) evalBlockStatement(block *ast.BlockStatement, env *object.Environment) object.Object {
	for _, stmt := range block.Statements {
		result := i.evalTracedStatement(stmt, env)
		if result != nil {
			switch result.Type() {
			case object.RETURN_VALUE_OBJ, object.BREAK_OBJ, object.CONTINUE_OBJ, object.ERROR_OBJ:
				return result
			}
		}
	}
	return object.NULL
}

// evalIfStatement handles `order (cond) {...} combo (cond) {...}...
// [special {...}]` (SPEC.md §8, orderStmt) — the first matching
// condition's block runs (in the enclosing scope, per SPEC.md §3),
// `special`'s block runs if nothing else matched, or nothing runs at
// all and this evaluates to NULL.
func (i *Interpreter) evalIfStatement(is *ast.IfStatement, env *object.Environment) object.Object {
	cond := i.Eval(is.Condition, env)
	if isError(cond) {
		return cond
	}
	if object.IsTruthy(cond) {
		return i.evalBlockStatement(is.Consequence, env)
	}

	for _, combo := range is.Combos {
		cond := i.Eval(combo.Condition, env)
		if isError(cond) {
			return cond
		}
		if object.IsTruthy(cond) {
			return i.evalBlockStatement(combo.Body, env)
		}
	}

	if is.Alternative != nil {
		return i.evalBlockStatement(is.Alternative, env)
	}

	return object.NULL
}

// evalCountedLoop handles `knead (init; cond; post) {...}` (SPEC.md
// §8, countedHeader). `burnt` stops the loop immediately, skipping
// Post entirely (matching every C-family break); `flip` still runs
// Post before the next Cond check (matching every C-family continue).
func (i *Interpreter) evalCountedLoop(cl *ast.CountedLoop, env *object.Environment) object.Object {
	if cl.Init != nil {
		if result := i.Eval(cl.Init, env); isError(result) {
			return result
		}
	}

	for lap := 1; ; lap++ {
		if cl.Cond != nil {
			cond := i.Eval(cl.Cond, env)
			if isError(cond) {
				return cond
			}
			if !object.IsTruthy(cond) {
				break
			}
		}

		var label string
		if i.Trace != nil {
			label = fmt.Sprintf("knead (...) lap %d", lap)
		}
		result := i.evalFramed(label, cl.Body, env)
		if result != nil {
			switch result.Type() {
			case object.ERROR_OBJ, object.RETURN_VALUE_OBJ:
				return result
			case object.BREAK_OBJ:
				return object.NULL
			}
		}

		if cl.Post != nil {
			if result := i.Eval(cl.Post, env); isError(result) {
				return result
			}
		}
	}

	return object.NULL
}

// evalForEachLoop handles `knead item in collection {...}` (SPEC.md
// §8, forEachHeader). Dispatches on the collection's runtime type:
// List/Set iterate elements, Map iterates keys. `item` is bound via
// the ordinary Environment.Set (no new scope — same as CountedLoop's
// Init), so it's visible after the loop too, same as Python.
func (i *Interpreter) evalForEachLoop(fel *ast.ForEachLoop, env *object.Environment) object.Object {
	collection := i.Eval(fel.Collection, env)
	if isError(collection) {
		return collection
	}

	var items []object.Object
	switch c := collection.(type) {
	case *object.List:
		items = c.Elements
	case *object.Tuple:
		items = c.Elements
	case *object.Set:
		items = make([]object.Object, 0, c.Len())
		for _, item := range c.Elements {
			items = append(items, item)
		}
	case *object.Map:
		items = make([]object.Object, 0, len(c.Pairs))
		for _, pair := range c.Pairs {
			items = append(items, pair.Key)
		}
	default:
		return newError(fel.Token, "cannot iterate over %s", collection.Type())
	}

	for idx, item := range items {
		env.Set(fel.Identifier.Value, item)

		var label string
		if i.Trace != nil {
			label = fmt.Sprintf("knead %s in ... lap %d", fel.Identifier.Value, idx+1)
		}
		result := i.evalFramed(label, fel.Body, env)
		if result != nil {
			switch result.Type() {
			case object.ERROR_OBJ, object.RETURN_VALUE_OBJ:
				return result
			case object.BREAK_OBJ:
				return object.NULL
			}
		}
	}

	return object.NULL
}

// evalBakeStatement handles `bake (cond) {...}` (SPEC.md §8,
// bakeStmt) — an ordinary conditional loop, burnt/flip handled the
// same as the other two loop forms.
func (i *Interpreter) evalBakeStatement(bs *ast.BakeStatement, env *object.Environment) object.Object {
	for lap := 1; ; lap++ {
		cond := i.Eval(bs.Condition, env)
		if isError(cond) {
			return cond
		}
		if !object.IsTruthy(cond) {
			break
		}

		var label string
		if i.Trace != nil {
			label = fmt.Sprintf("bake (...) lap %d", lap)
		}
		result := i.evalFramed(label, bs.Body, env)
		if result != nil {
			switch result.Type() {
			case object.ERROR_OBJ, object.RETURN_VALUE_OBJ:
				return result
			case object.BREAK_OBJ:
				return object.NULL
			}
		}
	}

	return object.NULL
}

// evalReturnStatement handles `serve [expression]` (SPEC.md §8,
// serveStmt) — a bare `serve` wraps NULL (nobox), matching what a
// recipe that falls off the end without any `serve` at all evaluates
// to (see evalBlockStatement).
func (i *Interpreter) evalReturnStatement(rs *ast.ReturnStatement, env *object.Environment) object.Object {
	if rs.ReturnValue == nil {
		return &object.ReturnValue{Value: object.NULL}
	}
	val := i.Eval(rs.ReturnValue, env)
	if isError(val) {
		return val
	}
	return &object.ReturnValue{Value: val}
}

// evalAssignStatement handles `target OP value` (SPEC.md §3, §8
// assignStmt) for both lvalue shapes (bare identifier, index
// expression) and both assignment forms (plain `=`, compound
// `+=`/`-=`/`*=`/`/=`/`%=`).
func (i *Interpreter) evalAssignStatement(as *ast.AssignStatement, env *object.Environment) object.Object {
	switch target := as.Target.(type) {
	case *ast.Identifier:
		return i.evalIdentifierAssign(as, target, env)
	case *ast.IndexExpression:
		return i.evalIndexAssign(as, target, env)
	default:
		return newError(as.Token, "invalid assignment target")
	}
}

func (i *Interpreter) evalIdentifierAssign(as *ast.AssignStatement, target *ast.Identifier, env *object.Environment) object.Object {
	value := i.Eval(as.Value, env)
	if isError(value) {
		return value
	}

	if as.Operator != "=" {
		current, ok := env.Get(target.Value)
		if !ok {
			return newError(target.Token, "undefined variable: %s", target.Value)
		}
		value = i.evalInfixOperator(as.Token, compoundOp(as.Operator), current, value)
		if isError(value) {
			return value
		}
	}

	env.Set(target.Value, value)
	return value
}

// evalIndexAssign evaluates the target's base and index expressions
// exactly once — critical for correctness, not just efficiency: if it
// re-evaluated them for a compound op's read-then-write, a base/index
// expression with a side effect (e.g. a call) would run twice.
func (i *Interpreter) evalIndexAssign(as *ast.AssignStatement, target *ast.IndexExpression, env *object.Environment) object.Object {
	base := i.Eval(target.Left, env)
	if isError(base) {
		return base
	}
	index := i.Eval(target.Index, env)
	if isError(index) {
		return index
	}

	value := i.Eval(as.Value, env)
	if isError(value) {
		return value
	}

	if as.Operator != "=" {
		current := readIndex(target.Token, base, index)
		if isError(current) {
			return current
		}
		value = i.evalInfixOperator(as.Token, compoundOp(as.Operator), current, value)
		if isError(value) {
			return value
		}
	}

	return writeIndex(target.Token, base, index, value)
}

// compoundOp strips the trailing '=' from a compound assignment
// operator: "+=" -> "+", "%=" -> "%".
func compoundOp(op string) string {
	return strings.TrimSuffix(op, "=")
}

// evalUnpackAssignStatement handles `id, id, ... = value` (SPEC.md
// §3.1). Dispatches on Value's *runtime* type once evaluated — not
// anything visible in the AST — since Value can be any expression (a
// ternary choosing between two Tuples, a function call returning one,
// a plain variable, ...), not just a literal:
//   - a Tuple (SPEC.md §2) unpacks with exact arity — every target,
//     including the last, gets its own bare value; a count mismatch is
//     an error rather than silently padding/truncating.
//   - a List unpacks with the classic rule: first N-1 targets take one
//     element each, the last always takes a List of everything left
//     over (even if that's empty) — never a bare scalar.
//
// Either way, Value is evaluated exactly once before any target is
// assigned, so `a, b = (b, a)` swaps correctly instead of clobbering
// `b` before it's read.
func (i *Interpreter) evalUnpackAssignStatement(uas *ast.UnpackAssignStatement, env *object.Environment) object.Object {
	value := i.Eval(uas.Value, env)
	if isError(value) {
		return value
	}

	if tup, ok := value.(*object.Tuple); ok {
		if len(tup.Elements) != len(uas.Targets) {
			return newError(uas.Token, "tuple has %d value(s), need exactly %d", len(tup.Elements), len(uas.Targets))
		}
		for idx, target := range uas.Targets {
			env.Set(target.Value, tup.Elements[idx])
		}
		return object.NULL
	}

	list, ok := value.(*object.List)
	if !ok {
		return newError(uas.Token, "cannot unpack %s, expected a List or Tuple", value.Type())
	}

	n := len(uas.Targets)
	if len(list.Elements) < n-1 {
		return newError(uas.Token, "not enough values to unpack: need at least %d, got %d", n-1, len(list.Elements))
	}

	for idx := 0; idx < n-1; idx++ {
		env.Set(uas.Targets[idx].Value, list.Elements[idx])
	}

	rest := append([]object.Object{}, list.Elements[n-1:]...)
	env.Set(uas.Targets[n-1].Value, object.NewList(rest))

	return object.NULL
}

// evalIncDecStatement handles `target++`/`++target`/`target--`/`--target`
// (SPEC.md §5.2) — sugar for `target = target +/- 1`, only defined for
// Integer/Float targets.
func (i *Interpreter) evalIncDecStatement(ids *ast.IncDecStatement, env *object.Environment) object.Object {
	delta := int64(1)
	if ids.Operator == "--" {
		delta = -1
	}

	switch target := ids.Target.(type) {
	case *ast.Identifier:
		current, ok := env.Get(target.Value)
		if !ok {
			return newError(target.Token, "undefined variable: %s", target.Value)
		}
		next, errObj := addDelta(ids.Token, current, delta)
		if errObj != nil {
			return errObj
		}
		env.Set(target.Value, next)
		return next

	case *ast.IndexExpression:
		base := i.Eval(target.Left, env)
		if isError(base) {
			return base
		}
		index := i.Eval(target.Index, env)
		if isError(index) {
			return index
		}
		current := readIndex(target.Token, base, index)
		if isError(current) {
			return current
		}
		next, errObj := addDelta(ids.Token, current, delta)
		if errObj != nil {
			return errObj
		}
		return writeIndex(target.Token, base, index, next)

	default:
		return newError(ids.Token, "invalid increment/decrement target")
	}
}
