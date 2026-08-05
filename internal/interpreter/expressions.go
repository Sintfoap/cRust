package interpreter

import (
	"github.com/Sintfoap/cRust/internal/ast"
	"github.com/Sintfoap/cRust/internal/object"
	"github.com/Sintfoap/cRust/internal/token"
)

func (i *Interpreter) evalIdentifier(node *ast.Identifier, env *object.Environment) object.Object {
	if val, ok := env.Get(node.Value); ok {
		return val
	}
	if builtin, ok := i.Builtins[node.Value]; ok {
		return builtin
	}
	return newError(node.Token, "identifier not found: %s", node.Value)
}

// evalPrefixExpression handles unary `-x` and `hold x` (SPEC.md §5,
// unary). `hold` applies the same truthiness rule (§6) as `order`/
// `bake`/ternary conditions, to any operand type — not just Boolean.
func (i *Interpreter) evalPrefixExpression(pe *ast.PrefixExpression, env *object.Environment) object.Object {
	right := i.Eval(pe.Right, env)
	if isError(right) {
		return right
	}

	switch pe.Operator {
	case "-":
		switch r := right.(type) {
		case *object.Integer:
			return object.NewInteger(-r.Value)
		case *object.Float:
			return &object.Float{Value: -r.Value}
		default:
			return newError(pe.Token, "unary -: expected a number, got %s", right.Type())
		}
	case "hold":
		return object.NativeBoolToBooleanObject(!isTruthy(right))
	default:
		return newError(pe.Token, "unknown prefix operator: %s", pe.Operator)
	}
}

// evalInfixExpression handles every InfixExpression operator.
// with/or need to see the un-evaluated Right so they can short-circuit
// (SPEC.md §5's with/or are keyword logical operators, not eager
// value-producing ones), so they're peeled off before Right is
// evaluated at all; everything else evaluates both sides first.
func (i *Interpreter) evalInfixExpression(ie *ast.InfixExpression, env *object.Environment) object.Object {
	if ie.Operator == "with" || ie.Operator == "or" {
		return i.evalLogicalExpression(ie, env)
	}

	left := i.Eval(ie.Left, env)
	if isError(left) {
		return left
	}
	right := i.Eval(ie.Right, env)
	if isError(right) {
		return right
	}

	return i.evalInfixOperator(ie.Token, ie.Operator, left, right)
}

// evalLogicalExpression is with/or's short-circuit evaluation. Always
// produces a strict Boolean (never a leaked operand value the way
// Python's `and`/`or` do) — cRust already has a dedicated operator for
// "give me back the actual value, falling through on nobox" (Elvis,
// §5.4), so with/or stay in the plainer "give me a Boolean" lane,
// consistent with the language using explicit stuffed/thin rather than
// truthy-value passthrough anywhere else.
func (i *Interpreter) evalLogicalExpression(ie *ast.InfixExpression, env *object.Environment) object.Object {
	left := i.Eval(ie.Left, env)
	if isError(left) {
		return left
	}

	if ie.Operator == "with" && !isTruthy(left) {
		return object.FALSE
	}
	if ie.Operator == "or" && isTruthy(left) {
		return object.TRUE
	}

	right := i.Eval(ie.Right, env)
	if isError(right) {
		return right
	}
	return object.NativeBoolToBooleanObject(isTruthy(right))
}

// evalInfixOperator applies operator to two already-evaluated operands
// — the shared arithmetic/comparison core used both by ordinary infix
// expressions and by compound assignment (`x += 1`), so there's one
// place that owns SPEC.md §6's coercion rules.
func (i *Interpreter) evalInfixOperator(tok token.Token, operator string, left, right object.Object) object.Object {
	switch operator {
	case "+", "-", "*", "/", "%":
		return evalArithmetic(tok, operator, left, right)
	case "==":
		return object.NativeBoolToBooleanObject(object.Equal(left, right))
	case "!=":
		return object.NativeBoolToBooleanObject(!object.Equal(left, right))
	case "<", ">", "<=", ">=":
		return evalComparison(tok, operator, left, right)
	default:
		return newError(tok, "unknown operator: %s", operator)
	}
}

// evalArithmetic implements SPEC.md §6's arithmetic/coercion rules:
// `+ - *` stay Integer for two Integers, widen to Float if either
// operand is a Float; `/` always true-divides to a Float regardless of
// operand types; `%` requires two Integers; `+` between two Strings,
// two Lists, or two Tuples concatenates (always producing a new
// value — even for List, which `push` mutates in place instead);
// between mismatched types among String/List/Tuple, `+` is a type
// error rather than an implicit conversion.
func evalArithmetic(tok token.Token, operator string, left, right object.Object) object.Object {
	if operator == "+" {
		ls, lIsStr := left.(*object.String)
		rs, rIsStr := right.(*object.String)
		if lIsStr && rIsStr {
			return &object.String{Value: ls.Value + rs.Value}
		}
		if lIsStr || rIsStr {
			return newError(tok, "type error: cannot add %s and %s", left.Type(), right.Type())
		}

		ll, lIsList := left.(*object.List)
		rl, rIsList := right.(*object.List)
		if lIsList && rIsList {
			combined := make([]object.Object, 0, len(ll.Elements)+len(rl.Elements))
			combined = append(combined, ll.Elements...)
			combined = append(combined, rl.Elements...)
			return object.NewList(combined)
		}
		if lIsList || rIsList {
			return newError(tok, "type error: cannot add %s and %s", left.Type(), right.Type())
		}

		lt, lIsTuple := left.(*object.Tuple)
		rt, rIsTuple := right.(*object.Tuple)
		if lIsTuple && rIsTuple {
			combined := make([]object.Object, 0, len(lt.Elements)+len(rt.Elements))
			combined = append(combined, lt.Elements...)
			combined = append(combined, rt.Elements...)
			return object.NewTuple(combined)
		}
		if lIsTuple || rIsTuple {
			return newError(tok, "type error: cannot add %s and %s", left.Type(), right.Type())
		}
	}

	li, lIsInt := left.(*object.Integer)
	ri, rIsInt := right.(*object.Integer)

	if operator == "%" {
		if !lIsInt || !rIsInt {
			return newError(tok, "%% requires two Integers, got %s and %s", left.Type(), right.Type())
		}
		if ri.Value == 0 {
			return newError(tok, "division by zero")
		}
		return object.NewInteger(li.Value % ri.Value)
	}

	if lIsInt && rIsInt && operator != "/" {
		switch operator {
		case "+":
			return object.NewInteger(li.Value + ri.Value)
		case "-":
			return object.NewInteger(li.Value - ri.Value)
		case "*":
			return object.NewInteger(li.Value * ri.Value)
		}
	}

	lf, lIsNum := numericValue(left)
	rf, rIsNum := numericValue(right)
	if !lIsNum || !rIsNum {
		return newError(tok, "unsupported operand types for %s: %s and %s", operator, left.Type(), right.Type())
	}

	switch operator {
	case "+":
		return &object.Float{Value: lf + rf}
	case "-":
		return &object.Float{Value: lf - rf}
	case "*":
		return &object.Float{Value: lf * rf}
	case "/":
		if rf == 0 {
			return newError(tok, "division by zero")
		}
		return &object.Float{Value: lf / rf}
	}

	return newError(tok, "unknown operator: %s", operator)
}

// evalComparison implements SPEC.md §6's ordering rule: `< > <= >=`
// only accept number-vs-number (Integer/Float freely mixed) or
// string-vs-string; anything else is a runtime Error, unlike `==`'s
// "always false across types" (evaluated separately via object.Equal).
func evalComparison(tok token.Token, operator string, left, right object.Object) object.Object {
	if lf, ok := numericValue(left); ok {
		if rf, ok := numericValue(right); ok {
			return object.NativeBoolToBooleanObject(compareOrdered(operator, lf, rf))
		}
	}
	if ls, ok := left.(*object.String); ok {
		if rs, ok := right.(*object.String); ok {
			return object.NativeBoolToBooleanObject(compareOrdered(operator, ls.Value, rs.Value))
		}
	}
	return newError(tok, "cannot compare %s and %s with %s", left.Type(), right.Type(), operator)
}

func compareOrdered[T float64 | string](operator string, a, b T) bool {
	switch operator {
	case "<":
		return a < b
	case ">":
		return a > b
	case "<=":
		return a <= b
	case ">=":
		return a >= b
	default:
		return false
	}
}

// evalRangeExpression handles `start..end` / `start.<end` (SPEC.md
// §5.1) — both bounds must be Integer, the result is an
// immediately-materialized List (never a lazy sequence), and start >
// the (inclusive-adjusted) end produces an empty List rather than an
// implicit reversal.
func (i *Interpreter) evalRangeExpression(re *ast.RangeExpression, env *object.Environment) object.Object {
	start := i.Eval(re.Start, env)
	if isError(start) {
		return start
	}
	end := i.Eval(re.End, env)
	if isError(end) {
		return end
	}

	si, ok := start.(*object.Integer)
	if !ok {
		return newError(re.Token, "range bounds must be Integers, got %s", start.Type())
	}
	ei, ok := end.(*object.Integer)
	if !ok {
		return newError(re.Token, "range bounds must be Integers, got %s", end.Type())
	}

	upper := ei.Value
	if !re.Inclusive {
		upper--
	}
	if si.Value > upper {
		return object.NewList(nil)
	}

	elements := make([]object.Object, 0, upper-si.Value+1)
	for v := si.Value; v <= upper; v++ {
		elements = append(elements, object.NewInteger(v))
	}
	return object.NewList(elements)
}

// evalTernaryExpression handles `cond (| then |) else` (SPEC.md §5.3)
// — exactly one of Then/Else is ever evaluated, never both.
func (i *Interpreter) evalTernaryExpression(te *ast.TernaryExpression, env *object.Environment) object.Object {
	cond := i.Eval(te.Cond, env)
	if isError(cond) {
		return cond
	}
	if isTruthy(cond) {
		return i.Eval(te.Then, env)
	}
	return i.Eval(te.Else, env)
}

// evalElvisExpression handles `a ?: b` (SPEC.md §5.4) — tests
// specifically for nobox, not general falsiness, so `thin ?: x` stays
// `thin` and `0 ?: x` stays `0`.
func (i *Interpreter) evalElvisExpression(ee *ast.ElvisExpression, env *object.Environment) object.Object {
	left := i.Eval(ee.Left, env)
	if isError(left) {
		return left
	}
	if left != object.NULL {
		return left
	}
	return i.Eval(ee.Right, env)
}

// evalFunctionLiteral builds a closure capturing env (SPEC.md §3) and,
// for a *named* literal, binds it into env under that name as a side
// effect — this is what makes `recipe add(a, b) {...}` at statement
// level act like a declaration, with no separate AST node or Eval case
// needed for "recipe declaration" versus "anonymous function literal
// used as a value" (see ast.FunctionLiteral's doc comment).
func (i *Interpreter) evalFunctionLiteral(fl *ast.FunctionLiteral, env *object.Environment) object.Object {
	fn := &object.Function{Parameters: fl.Parameters, Body: fl.Body, Env: env}
	if fl.Name != nil {
		env.Set(fl.Name.Value, fn)
	}
	return fn
}

// evalCallExpression handles `function(arg, ...)` (SPEC.md §8, call).
func (i *Interpreter) evalCallExpression(ce *ast.CallExpression, env *object.Environment) object.Object {
	fn := i.Eval(ce.Function, env)
	if isError(fn) {
		return fn
	}

	args, errObj := i.evalExpressions(ce.Arguments, env)
	if errObj != nil {
		return errObj
	}

	var label string
	if i.Trace != nil {
		label = ce.Function.String() + "(...)"
	}
	return i.applyFunction(ce.Token, label, fn, args)
}

// evalExpressions evaluates exps left to right, stopping at the first
// Error.
func (i *Interpreter) evalExpressions(exps []ast.Expression, env *object.Environment) ([]object.Object, *object.Error) {
	result := make([]object.Object, 0, len(exps))
	for _, e := range exps {
		evaluated := i.Eval(e, env)
		if errObj, ok := evaluated.(*object.Error); ok {
			return nil, errObj
		}
		result = append(result, evaluated)
	}
	return result, nil
}

// applyFunction calls fn (a user *object.Function or a
// *object.Builtin) with args. A user Function's call extends its
// *captured* Env, not the caller's — that's what makes closures work
// (SPEC.md §3) — and unwraps the body's result the same way every
// block-completion path does. Parameters bind via Declare, not Set:
// each call's extEnv is fresh (object.NewEnclosedEnvironment), and
// Declare is what actually keeps it isolated — using Set here would
// walk out to fn.Env looking for an existing same-named binding (e.g.
// a global with the same name as a parameter) and silently overwrite
// it instead of shadowing it, since Set can't tell "this is a brand
// new parameter slot" apart from "this is an ordinary reassignment."
// A Builtin error's position is unset until here, since
// internal/builtins knows nothing about source positions; this is the
// one place that patches it in. label is purely for the debugger's
// frame display (see evalFramed) — the callee's own source text
// ("sum(...)") when known from a real call site, or a generic
// fallback from callers with no call-site expression to read (i.Call,
// used by e.g. the map() builtin).
func (i *Interpreter) applyFunction(tok token.Token, label string, fn object.Object, args []object.Object) object.Object {
	switch fn := fn.(type) {
	case *object.Function:
		if len(args) != len(fn.Parameters) {
			return newError(tok, "wrong number of arguments: want %d, got %d", len(fn.Parameters), len(args))
		}
		extEnv := object.NewEnclosedEnvironment(fn.Env)
		for idx, param := range fn.Parameters {
			extEnv.Declare(param.Value, args[idx])
		}
		result := i.evalFramed(label, fn.Body, extEnv)
		return unwrapReturnValue(result)

	case *object.Builtin:
		result := fn.Fn(args...)
		if errObj, ok := result.(*object.Error); ok && errObj.Line == 0 && errObj.Col == 0 {
			errObj.Line, errObj.Col = tok.Line, tok.Col
		}
		return result

	default:
		return newError(tok, "not a recipe: %s", fn.Type())
	}
}

// evalIndexExpression handles `left[index]` (SPEC.md §8, index) reads.
func (i *Interpreter) evalIndexExpression(ie *ast.IndexExpression, env *object.Environment) object.Object {
	left := i.Eval(ie.Left, env)
	if isError(left) {
		return left
	}
	index := i.Eval(ie.Index, env)
	if isError(index) {
		return index
	}
	return readIndex(ie.Token, left, index)
}

// readIndex is the shared index-read core for both plain
// `left[index]` expressions and a compound assignment's "current
// value" step (`list[i] += 1`) — the latter needs base/index
// pre-evaluated exactly once, hence taking them as already-Eval'd
// Objects rather than ast.Expressions.
//
// A missing Map key reads as nobox rather than erroring — SPEC.md's
// own example (`cache[n] = cache[n] ?: computeFib(n)`, a memoization
// pattern) only works if that's true, since ?: needs a nobox to fall
// through on rather than an Error to propagate.
func readIndex(tok token.Token, base, index object.Object) object.Object {
	switch b := base.(type) {
	case *object.List:
		idx, ok := index.(*object.Integer)
		if !ok {
			return newError(tok, "List index must be an Integer, got %s", index.Type())
		}
		if idx.Value < 0 || idx.Value >= int64(len(b.Elements)) {
			return newError(tok, "index out of range: %d", idx.Value)
		}
		return b.Elements[idx.Value]

	case *object.Map:
		if !isValidMapKey(index) {
			return newError(tok, "map keys must be Hashable (String, Integer, Float, Boolean, or Tuple), got %s", index.Type())
		}
		val, ok := b.Get(index)
		if !ok {
			return object.NULL
		}
		return val

	case *object.String:
		idx, ok := index.(*object.Integer)
		if !ok {
			return newError(tok, "String index must be an Integer, got %s", index.Type())
		}
		runes := []rune(b.Value)
		if idx.Value < 0 || idx.Value >= int64(len(runes)) {
			return newError(tok, "index out of range: %d", idx.Value)
		}
		return &object.String{Value: string(runes[idx.Value])}

	case *object.Tuple:
		idx, ok := index.(*object.Integer)
		if !ok {
			return newError(tok, "Tuple index must be an Integer, got %s", index.Type())
		}
		if idx.Value < 0 || idx.Value >= int64(len(b.Elements)) {
			return newError(tok, "index out of range: %d", idx.Value)
		}
		return b.Elements[idx.Value]

	case *object.Set:
		return newError(tok, "Set is not indexable")

	default:
		return newError(tok, "type %s is not indexable", base.Type())
	}
}

// writeIndex is readIndex's write-side counterpart, used by index
// assignment (`list[0] = 1`) and index inc/dec (`list[0]++`). Strings
// are immutable (no case here does anything but error), matching every
// other cRust value's copy/reference semantics being fixed at
// construction — SPEC.md never proposes in-place character mutation.
func writeIndex(tok token.Token, base, index, value object.Object) object.Object {
	switch b := base.(type) {
	case *object.List:
		idx, ok := index.(*object.Integer)
		if !ok {
			return newError(tok, "List index must be an Integer, got %s", index.Type())
		}
		if idx.Value < 0 || idx.Value >= int64(len(b.Elements)) {
			return newError(tok, "index out of range: %d", idx.Value)
		}
		b.Elements[idx.Value] = value
		return value

	case *object.Map:
		if !isValidMapKey(index) {
			return newError(tok, "map keys must be Hashable (String, Integer, Float, Boolean, or Tuple), got %s", index.Type())
		}
		b.Set(index, value)
		return value

	case *object.Set:
		return newError(tok, "Set is not indexable")

	case *object.Tuple:
		return newError(tok, "Tuple is immutable, does not support index assignment")

	case *object.String:
		return newError(tok, "String is immutable, does not support index assignment")

	default:
		return newError(tok, "type %s is not indexable", base.Type())
	}
}

// isValidMapKey is true for any Hashable Object — matching Set's own
// "any Hashable element" rule (evalSetLiteral) rather than a narrower
// String/Integer-only allowlist, so a Map and a Set backed by the same
// object.Hashable interface actually agree on what can go in either
// one. Notably this includes Tuple: SPEC.md §2.3 introduced Tuple
// specifically so fixed-size groups like grid coordinates could be
// hashed "the way a List never safely could" and explicitly promises
// it "can be a Map key or Set element" — a promise this function used
// to only keep for Set, since it still special-cased String/Integer
// alone (a stale rule from before Tuple existed).
func isValidMapKey(obj object.Object) bool {
	_, ok := obj.(object.Hashable)
	return ok
}

// addDelta is IncDecStatement's `+1`/`-1` step (SPEC.md §5.2), shared
// between the identifier and index-expression target shapes.
func addDelta(tok token.Token, current object.Object, delta int64) (object.Object, *object.Error) {
	switch c := current.(type) {
	case *object.Integer:
		return object.NewInteger(c.Value + delta), nil
	case *object.Float:
		return &object.Float{Value: c.Value + float64(delta)}, nil
	default:
		return nil, newError(tok, "++/--: expected Integer or Float, got %s", current.Type())
	}
}

func (i *Interpreter) evalListLiteral(ll *ast.ListLiteral, env *object.Environment) object.Object {
	elements, errObj := i.evalExpressions(ll.Elements, env)
	if errObj != nil {
		return errObj
	}
	return object.NewList(elements)
}

// evalMapLiteral enforces SPEC.md §2's narrower "String or Integer
// only" Map-key rule — stricter than object.Hashable, which is why
// this check lives here rather than in internal/object (see
// hashkey.go's doc comment).
func (i *Interpreter) evalMapLiteral(ml *ast.MapLiteral, env *object.Environment) object.Object {
	m := object.NewMap()
	for _, pair := range ml.Pairs {
		key := i.Eval(pair.Key, env)
		if isError(key) {
			return key
		}
		if !isValidMapKey(key) {
			return newError(ml.Token, "map keys must be Hashable (String, Integer, Float, Boolean, or Tuple), got %s", key.Type())
		}
		value := i.Eval(pair.Value, env)
		if isError(value) {
			return value
		}
		m.Set(key, value)
	}
	return m
}

func (i *Interpreter) evalSetLiteral(sl *ast.SetLiteral, env *object.Environment) object.Object {
	elements, errObj := i.evalExpressions(sl.Elements, env)
	if errObj != nil {
		return errObj
	}
	s := object.NewSet()
	for _, elem := range elements {
		if !s.Add(elem) {
			return newError(sl.Token, "unhashable type: %s cannot be a Set element", elem.Type())
		}
	}
	return s
}

// evalTupleLiteral requires every element to be Hashable (SPEC.md §2)
// — checked once here, at construction, specifically so
// object.Tuple's own HashKey never has to handle a non-Hashable
// element itself (see its doc comment). This is the same "narrower
// than Object itself" restriction evalMapLiteral already enforces for
// map keys, applied to every element instead of just keys.
func (i *Interpreter) evalTupleLiteral(tl *ast.TupleLiteral, env *object.Environment) object.Object {
	elements, errObj := i.evalExpressions(tl.Elements, env)
	if errObj != nil {
		return errObj
	}
	for _, elem := range elements {
		if _, ok := elem.(object.Hashable); !ok {
			return newError(tl.Token, "unhashable type: %s cannot be a Tuple element", elem.Type())
		}
	}
	return object.NewTuple(elements)
}
