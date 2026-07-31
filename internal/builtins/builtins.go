// Package builtins implements cRust's standard library (SPEC.md §7) —
// predeclared global functions, not keywords, so user code is free to
// shadow any of them (SPEC.md §4). Building this now, ahead of its own
// Phase 5, mirrors how internal/object and the CLI's debug commands
// were built ahead of their phases: without at least `deliver`, Phase
// 4's interpreter would have no way to produce visible output at all.
package builtins

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Sintfoap/cRust/internal/object"
)

// New returns a fresh builtin table with `deliver` writing to output
// and `unbox` (with no argument) reading from stdin. A constructor
// rather than a package-level table, so callers (tests especially) can
// capture output/supply input deterministically instead of sharing
// mutable global state.
func New(output io.Writer, stdin io.Reader) map[string]*object.Builtin {
	return map[string]*object.Builtin{
		"deliver":  {Fn: deliverFn(output)},
		"slices":   {Fn: slicesFn},
		"sauce":    {Fn: sauceFn},
		"chars":    {Fn: charsFn},
		"ints":     {Fn: intsFn},
		"idiv":     {Fn: idivFn},
		"gather":   {Fn: gatherFn},
		"sprinkle": {Fn: sprinkleFn},
		"scrape":   {Fn: scrapeFn},
		"topped":   {Fn: toppedFn},
		"combine":  {Fn: combineFn},
		"shared":   {Fn: sharedFn},
		"strip":    {Fn: stripFn},
		"unbox":    {Fn: unboxFn(stdin)},
		"lines":    {Fn: linesFn},
		"trim":     {Fn: trimFn},
		"str":      {Fn: strFn},
		"int":      {Fn: intFn},
		"float":    {Fn: floatFn},
		"bool":     {Fn: boolFn},
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
	case *object.Tuple:
		return object.NewInteger(int64(len(x.Elements)))
	case *object.Map:
		return object.NewInteger(int64(len(x.Pairs)))
	case *object.Set:
		return object.NewInteger(int64(x.Len()))
	default:
		return newError("slices: argument must be a String, List, Tuple, Map, or Set, got %s", x.Type())
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

// intsFn is `ints(s)` (SPEC.md §7) — splits a String into a List of
// single-digit Integers, one per character: the numeric-grid
// counterpart to `chars`, which does the same split but keeps each
// character as a one-rune String instead of parsing it. Every rune
// must be a decimal digit ('0'-'9') — a non-digit character is a
// runtime error, not silently skipped or mapped to some other value.
func intsFn(args ...object.Object) object.Object {
	if len(args) != 1 {
		return wrongArgCount("ints", "1", len(args))
	}
	s, ok := args[0].(*object.String)
	if !ok {
		return wrongArgType("ints", 0, "a String", args[0])
	}
	runes := []rune(s.Value)
	out := make([]object.Object, len(runes))
	for i, r := range runes {
		if r < '0' || r > '9' {
			return newError("ints: %q is not a digit", string(r))
		}
		out[i] = object.NewInteger(int64(r - '0'))
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

// unboxFn is `unbox()` / `unbox(path)` (SPEC.md §7) — reads all of
// stdin as a String with no argument, or a whole file's contents with
// one String argument (the puzzle-input path). Either way the result
// is the raw content, trailing newline and all — pair with `lines` to
// split it, since whether there's a trailing blank entry depends on
// the exact input file, not something this should silently normalize
// away.
func unboxFn(stdin io.Reader) object.BuiltinFunction {
	return func(args ...object.Object) object.Object {
		switch len(args) {
		case 0:
			data, err := io.ReadAll(stdin)
			if err != nil {
				return newError("unbox: reading stdin: %s", err)
			}
			return &object.String{Value: string(data)}
		case 1:
			path, ok := args[0].(*object.String)
			if !ok {
				return wrongArgType("unbox", 0, "a String", args[0])
			}
			data, err := os.ReadFile(path.Value)
			if err != nil {
				return newError("unbox: %s", err)
			}
			return &object.String{Value: string(data)}
		default:
			return wrongArgCount("unbox", "0 or 1", len(args))
		}
	}
}

// linesFn is `lines(s)` (SPEC.md §7) — splits a String into a List of
// lines, one per `\n` (a trailing `\r` from CRLF input is stripped
// too), with no trailing empty entry for a string that ends in a
// newline — the same "how many actual lines are there" behavior most
// languages' readlines-equivalents give, via bufio.Scanner's own
// ScanLines split function rather than reimplementing that edge case
// by hand.
func linesFn(args ...object.Object) object.Object {
	if len(args) != 1 {
		return wrongArgCount("lines", "1", len(args))
	}
	s, ok := args[0].(*object.String)
	if !ok {
		return wrongArgType("lines", 0, "a String", args[0])
	}
	var out []object.Object
	scanner := bufio.NewScanner(strings.NewReader(s.Value))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		out = append(out, &object.String{Value: scanner.Text()})
	}
	return object.NewList(out)
}

// trimFn is `trim(s)` (SPEC.md §7) — removes leading/trailing
// whitespace from a String. Named distinctly from `strip` (Set
// difference, already taken — SPEC.md §7) specifically to avoid the
// two colliding; the most common reason to reach for this one is
// cleaning up a trailing newline off `unbox()`'s raw output before
// parsing it.
func trimFn(args ...object.Object) object.Object {
	if len(args) != 1 {
		return wrongArgCount("trim", "1", len(args))
	}
	s, ok := args[0].(*object.String)
	if !ok {
		return wrongArgType("trim", 0, "a String", args[0])
	}
	return &object.String{Value: strings.TrimSpace(s.Value)}
}

// strFn is `str(x)` (SPEC.md §7) — converts any value to its String
// form, the same text `deliver` would print for it (Inspect()),
// wrapped as a String value instead of written to output.
func strFn(args ...object.Object) object.Object {
	if len(args) != 1 {
		return wrongArgCount("str", "1", len(args))
	}
	if s, ok := args[0].(*object.String); ok {
		return s
	}
	return &object.String{Value: args[0].Inspect()}
}

// intFn is `int(x)` (SPEC.md §7) — converts a String (parsed as a
// base-10 integer; a decimal string like "3.5" is a runtime error, not
// silently truncated — call float(x) first if that's really what's
// wanted) or a Float (truncated toward zero, like a static cast) to an
// Integer. An Integer argument passes through unchanged.
func intFn(args ...object.Object) object.Object {
	if len(args) != 1 {
		return wrongArgCount("int", "1", len(args))
	}
	switch v := args[0].(type) {
	case *object.Integer:
		return v
	case *object.Float:
		return object.NewInteger(int64(v.Value))
	case *object.String:
		n, err := strconv.ParseInt(strings.TrimSpace(v.Value), 10, 64)
		if err != nil {
			return newError("int: cannot parse %q as an Integer", v.Value)
		}
		return object.NewInteger(n)
	default:
		return wrongArgType("int", 0, "a String, Integer, or Float", args[0])
	}
}

// floatFn is `float(x)` (SPEC.md §7) — converts a String (parsed) or
// an Integer (widened) to a Float. A Float argument passes through
// unchanged.
func floatFn(args ...object.Object) object.Object {
	if len(args) != 1 {
		return wrongArgCount("float", "1", len(args))
	}
	switch v := args[0].(type) {
	case *object.Float:
		return v
	case *object.Integer:
		return &object.Float{Value: float64(v.Value)}
	case *object.String:
		f, err := strconv.ParseFloat(strings.TrimSpace(v.Value), 64)
		if err != nil {
			return newError("float: cannot parse %q as a Float", v.Value)
		}
		return &object.Float{Value: f}
	default:
		return wrongArgType("float", 0, "a String, Integer, or Float", args[0])
	}
}

// boolFn is `bool(x)` (SPEC.md §7) — normalizes any value to a strict
// Boolean using cRust's own truthiness rule (SPEC.md §6: only `thin`
// and `nobox` are falsy — everything else, including 0 and "", is
// truthy), the same rule `order`/`bake`/ternary already apply,
// exposed here as a value instead of only a branch decision.
func boolFn(args ...object.Object) object.Object {
	if len(args) != 1 {
		return wrongArgCount("bool", "1", len(args))
	}
	switch v := args[0].(type) {
	case *object.Boolean:
		return v
	case *object.Null:
		return object.FALSE
	default:
		return object.TRUE
	}
}
