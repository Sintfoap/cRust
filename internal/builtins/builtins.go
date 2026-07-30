// Package builtins implements cRust's standard library (SPEC.md §7) —
// predeclared global functions, not keywords, so user code is free to
// shadow any of them (SPEC.md §4). Building this now, ahead of its own
// Phase 5, mirrors how internal/object and the CLI's debug commands
// were built ahead of their phases: without at least `deliver`, Phase
// 4's interpreter would have no way to produce visible output at all.
package builtins

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/Sintfoap/cRust/internal/object"
)

// New returns a fresh builtin table with `deliver` writing to output.
// A constructor rather than a package-level table, so callers (tests
// especially) can capture output deterministically instead of sharing
// mutable global state.
func New(output io.Writer) map[string]*object.Builtin {
	return map[string]*object.Builtin{
		"deliver":  {Fn: deliverFn(output)},
		"slices":   {Fn: slicesFn},
		"sauce":    {Fn: sauceFn},
		"chars":    {Fn: charsFn},
		"idiv":     {Fn: idivFn},
		"gather":   {Fn: gatherFn},
		"sprinkle": {Fn: sprinkleFn},
		"scrape":   {Fn: scrapeFn},
		"topped":   {Fn: toppedFn},
		"combine":  {Fn: combineFn},
		"shared":   {Fn: sharedFn},
		"strip":    {Fn: stripFn},
	}
}

func newError(format string, args ...any) *object.Error {
	return &object.Error{Message: fmt.Sprintf(format, args...)}
}

func wrongArgCount(name string, want string, got int) *object.Error {
	return newError("%s: expected %s argument(s), got %d", name, want, got)
}

func wrongArgType(name string, index int, want string, got object.Object) *object.Error {
	return newError("%s: argument %d must be %s, got %s", name, index+1, want, got.Type())
}

// deliverFn is `deliver(values...)` (SPEC.md §7) — prints every
// argument's Inspect() form, space-separated, followed by a newline.
func deliverFn(output io.Writer) object.BuiltinFunction {
	return func(args ...object.Object) object.Object {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = a.Inspect()
		}
		fmt.Fprintln(output, strings.Join(parts, " "))
		return object.NULL
	}
}

// slicesFn is `slices(x)` (SPEC.md §7) — length of a String (rune
// count, not byte count — SPEC.md §2.1 strings are UTF-8), List, Map,
// or Set.
func slicesFn(args ...object.Object) object.Object {
	if len(args) != 1 {
		return wrongArgCount("slices", "1", len(args))
	}
	switch x := args[0].(type) {
	case *object.String:
		return object.NewInteger(int64(utf8.RuneCountInString(x.Value)))
	case *object.List:
		return object.NewInteger(int64(len(x.Elements)))
	case *object.Map:
		return object.NewInteger(int64(len(x.Pairs)))
	case *object.Set:
		return object.NewInteger(int64(x.Len()))
	default:
		return newError("slices: argument must be a String, List, Map, or Set, got %s", x.Type())
	}
}

// sauceFn is `sauce(value, fallback)` (SPEC.md §7) — value unless it's
// nobox, in which case fallback. Identical logic to the `?:` operator
// (SPEC.md §5.4), as a plain function instead of an operator.
func sauceFn(args ...object.Object) object.Object {
	if len(args) != 2 {
		return wrongArgCount("sauce", "2", len(args))
	}
	if args[0] != object.NULL {
		return args[0]
	}
	return args[1]
}

// charsFn is `chars(s)` (SPEC.md §7) — splits a String into a List of
// one-character (one-rune) Strings.
func charsFn(args ...object.Object) object.Object {
	if len(args) != 1 {
		return wrongArgCount("chars", "1", len(args))
	}
	s, ok := args[0].(*object.String)
	if !ok {
		return wrongArgType("chars", 0, "a String", args[0])
	}
	runes := []rune(s.Value)
	out := make([]object.Object, len(runes))
	for i, r := range runes {
		out[i] = &object.String{Value: string(r)}
	}
	return object.NewList(out)
}

// idivFn is `idiv(a, b)` (SPEC.md §6) — integer (floor) division,
// since `/` always true-divides to a Float.
func idivFn(args ...object.Object) object.Object {
	if len(args) != 2 {
		return wrongArgCount("idiv", "2", len(args))
	}
	a, ok := args[0].(*object.Integer)
	if !ok {
		return wrongArgType("idiv", 0, "an Integer", args[0])
	}
	b, ok := args[1].(*object.Integer)
	if !ok {
		return wrongArgType("idiv", 1, "an Integer", args[1])
	}
	if b.Value == 0 {
		return newError("idiv: division by zero")
	}
	return object.NewInteger(a.Value / b.Value)
}

// gatherFn is `gather(list)` (SPEC.md §7) — collects a List into a
// Set, dropping duplicates.
func gatherFn(args ...object.Object) object.Object {
	if len(args) != 1 {
		return wrongArgCount("gather", "1", len(args))
	}
	list, ok := args[0].(*object.List)
	if !ok {
		return wrongArgType("gather", 0, "a List", args[0])
	}
	set := object.NewSet()
	for _, elem := range list.Elements {
		if !set.Add(elem) {
			return newError("gather: unhashable element of type %s cannot go in a Set", elem.Type())
		}
	}
	return set
}

// sprinkleFn is `sprinkle(set, item)` (SPEC.md §7) — adds item to set
// in place.
func sprinkleFn(args ...object.Object) object.Object {
	if len(args) != 2 {
		return wrongArgCount("sprinkle", "2", len(args))
	}
	set, ok := args[0].(*object.Set)
	if !ok {
		return wrongArgType("sprinkle", 0, "a Set", args[0])
	}
	if !set.Add(args[1]) {
		return newError("sprinkle: unhashable type %s cannot go in a Set", args[1].Type())
	}
	return object.NULL
}

// scrapeFn is `scrape(set, item)` (SPEC.md §7) — removes item from set
// in place, no error if absent.
func scrapeFn(args ...object.Object) object.Object {
	if len(args) != 2 {
		return wrongArgCount("scrape", "2", len(args))
	}
	set, ok := args[0].(*object.Set)
	if !ok {
		return wrongArgType("scrape", 0, "a Set", args[0])
	}
	set.Remove(args[1])
	return object.NULL
}

// toppedFn is `topped(set, item)` (SPEC.md §7) — membership test.
func toppedFn(args ...object.Object) object.Object {
	if len(args) != 2 {
		return wrongArgCount("topped", "2", len(args))
	}
	set, ok := args[0].(*object.Set)
	if !ok {
		return wrongArgType("topped", 0, "a Set", args[0])
	}
	return object.NativeBoolToBooleanObject(set.Has(args[1]))
}

// twoSets validates both arguments of a binary Set builtin at once.
func twoSets(name string, args []object.Object) (a, b *object.Set, errObj *object.Error) {
	if len(args) != 2 {
		return nil, nil, wrongArgCount(name, "2", len(args))
	}
	a, ok := args[0].(*object.Set)
	if !ok {
		return nil, nil, wrongArgType(name, 0, "a Set", args[0])
	}
	b, ok = args[1].(*object.Set)
	if !ok {
		return nil, nil, wrongArgType(name, 1, "a Set", args[1])
	}
	return a, b, nil
}

// combineFn is `combine(a, b)` (SPEC.md §7) — union.
func combineFn(args ...object.Object) object.Object {
	a, b, errObj := twoSets("combine", args)
	if errObj != nil {
		return errObj
	}
	result := object.NewSet()
	for _, item := range a.Elements {
		result.Add(item)
	}
	for _, item := range b.Elements {
		result.Add(item)
	}
	return result
}

// sharedFn is `shared(a, b)` (SPEC.md §7) — intersection.
func sharedFn(args ...object.Object) object.Object {
	a, b, errObj := twoSets("shared", args)
	if errObj != nil {
		return errObj
	}
	result := object.NewSet()
	for _, item := range a.Elements {
		if b.Has(item) {
			result.Add(item)
		}
	}
	return result
}

// stripFn is `strip(a, b)` (SPEC.md §7) — difference: items in a not
// in b.
func stripFn(args ...object.Object) object.Object {
	a, b, errObj := twoSets("strip", args)
	if errObj != nil {
		return errObj
	}
	result := object.NewSet()
	for _, item := range a.Elements {
		if !b.Has(item) {
			result.Add(item)
		}
	}
	return result
}
