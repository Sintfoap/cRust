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

// Call invokes a cRust-callable Object (a *object.Function or
// *object.Builtin) with args and returns its result — exactly
// internal/interpreter's own applyFunction logic, injected here rather
// than imported directly, since internal/interpreter already imports
// internal/builtins and Go doesn't allow the reverse. `map` is the only
// builtin that needs this today.
type Call func(fn object.Object, args []object.Object) object.Object

// New returns a fresh builtin table with `deliver` writing to output,
// `unbox` (with no argument) reading from stdin, and `map` invoking
// user-supplied functions via call. A constructor rather than a
// package-level table, so callers (tests especially) can capture
// output/supply input deterministically instead of sharing mutable
// global state.
func New(output io.Writer, stdin io.Reader, call Call) map[string]*object.Builtin {
	return map[string]*object.Builtin{
		"deliver":    {Fn: deliverFn(output)},
		"slices":     {Fn: slicesFn},
		"sauce":      {Fn: sauceFn},
		"chars":      {Fn: charsFn},
		"ints":       {Fn: intsFn},
		"push":       {Fn: pushFn},
		"map":        {Fn: mapFn(call)},
		"min":        {Fn: minMaxFn("min", func(cmp int) bool { return cmp < 0 })},
		"max":        {Fn: minMaxFn("max", func(cmp int) bool { return cmp > 0 })},
		"combos":     {Fn: combosFn},
		"enumerate":  {Fn: enumerateFn},
		"grid":       {Fn: gridFn},
		"newGrid":    {Fn: newGridFn},
		"at":         {Fn: atFn},
		"setAt":      {Fn: setAtFn},
		"gridBounds": {Fn: gridBoundsFn},
		"neighbors4": {Fn: neighborsFn("neighbors4", orthogonalOffsets)},
		"neighbors8": {Fn: neighborsFn("neighbors8", allOffsets)},
		"idiv":       {Fn: idivFn},
		"gather":     {Fn: gatherFn},
		"sprinkle":   {Fn: sprinkleFn},
		"scrape":     {Fn: scrapeFn},
		"topped":     {Fn: toppedFn},
		"contains":   {Fn: containsFn},
		"combine":    {Fn: combineFn},
		"shared":     {Fn: sharedFn},
		"strip":      {Fn: stripFn},
		"unbox":      {Fn: unboxFn(stdin)},
		"lines":      {Fn: linesFn},
		"join":       {Fn: joinFn},
		"split":      {Fn: splitFn},
		"trim":       {Fn: trimFn},
		"str":        {Fn: strFn},
		"int":        {Fn: intFn},
		"float":      {Fn: floatFn},
		"bool":       {Fn: boolFn},
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

// pushFn is `push(list, item)` (SPEC.md §7) — appends item to list in
// place, the List counterpart to `sprinkle`'s in-place Set insert.
// Unlike `+` (List/List concatenation, evalArithmetic — always
// produces a new List), push mutates the List that's actually passed
// in, so every other reference to it sees the appended item too.
func pushFn(args ...object.Object) object.Object {
	if len(args) != 2 {
		return wrongArgCount("push", "2", len(args))
	}
	list, ok := args[0].(*object.List)
	if !ok {
		return wrongArgType("push", 0, "a List", args[0])
	}
	list.Elements = append(list.Elements, args[1])
	return object.NULL
}

// mapFn is `map(iterable, fn)` (SPEC.md §7) — applies fn to every
// element of a List or Tuple, in order, and collects the results into
// a new List. fn can be a user-defined recipe or another builtin;
// invoking it goes through the injected call callback (see the Call
// type doc comment) rather than anything in this package, since
// calling a *object.Function needs internal/interpreter's environment
// machinery. A single-function design deliberately — chaining more
// than one transform per element is already possible by passing a
// lambda that does both (`map(xs, recipe(x) { serve g(f(x)) })`), so
// map itself doesn't need to accept a List of functions to pipeline;
// that would just be a second, redundant way to spell the same thing.
// Stops and returns immediately on the first element fn errors on,
// same short-circuiting every other builtin here already gives you
// for free by just returning whatever call() hands back.
func mapFn(call Call) object.BuiltinFunction {
	return func(args ...object.Object) object.Object {
		if len(args) != 2 {
			return wrongArgCount("map", "2", len(args))
		}
		var elements []object.Object
		switch v := args[0].(type) {
		case *object.List:
			elements = v.Elements
		case *object.Tuple:
			elements = v.Elements
		default:
			return wrongArgType("map", 0, "a List or Tuple", args[0])
		}

		out := make([]object.Object, len(elements))
		for i, elem := range elements {
			result := call(args[1], []object.Object{elem})
			if result.Type() == object.ERROR_OBJ {
				return result
			}
			out[i] = result
		}
		return object.NewList(out)
	}
}

// numericValue reports v's value as a float64 if v is an Integer or
// Float, mirroring internal/interpreter's own helper of the same name
// (interpreter.go) so min/max order numbers exactly the way `<`/`>` do
// (SPEC.md §6: Integer/Float freely mixed as one "number" category).
// Duplicated by hand rather than imported — internal/builtins can't
// import internal/interpreter (see the Call type's doc comment).
func numericValue(obj object.Object) (float64, bool) {
	switch v := obj.(type) {
	case *object.Integer:
		return float64(v.Value), true
	case *object.Float:
		return v.Value, true
	default:
		return 0, false
	}
}

// compareTwo orders a and b the same way `<`/`>` do (SPEC.md §6):
// number-vs-number (Integer/Float mixed freely) or string-vs-string.
// Returns a negative/zero/positive int (a<b / a==b / a>b), or an Error
// for any other pairing, including number-vs-string.
func compareTwo(name string, a, b object.Object) (int, *object.Error) {
	if af, ok := numericValue(a); ok {
		if bf, ok := numericValue(b); ok {
			switch {
			case af < bf:
				return -1, nil
			case af > bf:
				return 1, nil
			default:
				return 0, nil
			}
		}
	}
	if as, ok := a.(*object.String); ok {
		if bs, ok := b.(*object.String); ok {
			return strings.Compare(as.Value, bs.Value), nil
		}
	}
	return 0, newError("%s: cannot compare %s and %s", name, a.Type(), b.Type())
}

// minMaxFn builds `min(...)` / `max(...)` (SPEC.md §7). Accepts either
// 2+ direct arguments (min(a, b, c)) or a single List/Tuple (min(xs)) —
// the same two-shape convention map/ints already established for
// "operate on either an iterable or its unpacked elements". want(cmp)
// decides which side wins a comparison: cmp is elem-compared-to-best,
// so `cmp < 0` (elem is smaller) picks min, `cmp > 0` picks max. Returns
// the winning element itself, not a converted copy, so `max(1, 2.5)`
// gives back the actual Float 2.5, not a widened Integer.
func minMaxFn(name string, want func(cmp int) bool) object.BuiltinFunction {
	return func(args ...object.Object) object.Object {
		var elements []object.Object
		switch {
		case len(args) == 1:
			switch v := args[0].(type) {
			case *object.List:
				elements = v.Elements
			case *object.Tuple:
				elements = v.Elements
			default:
				return wrongArgType(name, 0, "a List, Tuple, or 2+ arguments", args[0])
			}
			if len(elements) == 0 {
				return newError("%s: %s is empty", name, args[0].Type())
			}
		case len(args) >= 2:
			elements = args
		default:
			return wrongArgCount(name, "a List/Tuple, or 2+ values", len(args))
		}

		best := elements[0]
		for _, elem := range elements[1:] {
			cmp, errObj := compareTwo(name, elem, best)
			if errObj != nil {
				return errObj
			}
			if want(cmp) {
				best = elem
			}
		}
		return best
	}
}

// combosFn is `combos(list, n)` (SPEC.md §7) — every n-element
// combination of list's elements (List or Tuple), each returned as a
// Tuple, in lexicographic order of position. Combinations, not
// permutations: within one group, order doesn't matter and no element
// is picked twice, matching the standard "n choose k" idea (Python's
// itertools.combinations is the same shape) -- e.g. combos(xs, 2) is
// every distinct pair, combos(xs, 3) every distinct triple, and so on
// for any n. n > slices(list) isn't an error, just zero combinations —
// there aren't any, the same way asking for more items than exist
// isn't a special case worth its own error. n < 0 is a genuine error;
// there's no such thing as a negative-size combination. Every source
// element must be Hashable, the same requirement evalTupleLiteral
// enforces for an ordinary (a, b) literal (object/tuple.go's HashKey
// never has to handle a non-Hashable element) — checked once up front
// against the source elements rather than per generated combination,
// since the same elements are reused across all of them.
func combosFn(args ...object.Object) object.Object {
	if len(args) != 2 {
		return wrongArgCount("combos", "2", len(args))
	}
	var elements []object.Object
	switch v := args[0].(type) {
	case *object.List:
		elements = v.Elements
	case *object.Tuple:
		elements = v.Elements
	default:
		return wrongArgType("combos", 0, "a List or Tuple", args[0])
	}
	nObj, ok := args[1].(*object.Integer)
	if !ok {
		return wrongArgType("combos", 1, "an Integer", args[1])
	}
	n := int(nObj.Value)
	if n < 0 {
		return newError("combos: n must be non-negative, got %d", n)
	}
	for _, elem := range elements {
		if _, ok := elem.(object.Hashable); !ok {
			return newError("combos: unhashable type %s cannot be a Tuple element", elem.Type())
		}
	}
	if n > len(elements) {
		return object.NewList(nil)
	}

	var out []object.Object
	indices := make([]int, n)
	for i := range indices {
		indices[i] = i
	}
	for {
		group := make([]object.Object, n)
		for i, idx := range indices {
			group[i] = elements[idx]
		}
		out = append(out, object.NewTuple(group))

		i := n - 1
		for i >= 0 && indices[i] == len(elements)-n+i {
			i--
		}
		if i < 0 {
			break
		}
		indices[i]++
		for j := i + 1; j < n; j++ {
			indices[j] = indices[j-1] + 1
		}
	}
	return object.NewList(out)
}

// enumerateFn is `enumerate(collection)` (SPEC.md §7) — pairs each
// element of a List or Tuple with its 0-based position, as a
// (index, value) Tuple, so `knead pair in enumerate(items) { i, x =
// pair; ... }` gets both using the tuple-unpack assignment sugar
// (statements.go's evalUnpackAssign) already built for exactly this
// shape.
//
// The pair is a Tuple, not a two-element List, on purpose: List-unpack
// follows the classic "last target catches everything left over as its
// own List" rule (so `i, x = [0, "a"]` binds `x` to `["a"]`, not `"a"`
// bare) — right for a variable-length remainder, wrong for a fixed
// index/value pair, where `x` should always be the bare value. A Tuple
// unpacks with exact arity instead, giving `x` the bare element every
// time. That does mean every element must be Hashable (SPEC.md §2.3,
// checked up front the same way combos() above checks it, and for the
// same reason: a Tuple has to be safe to use as a Set element or Map
// key wherever one ends up), so enumerate() can't pair positions with
// an unhashable value like a List/Map/Grid row -- reasonable, since
// cRust already has an ordinary counted loop (`knead i in
// 0.<slices(xs)`) for indexing into exactly that kind of collection.
func enumerateFn(args ...object.Object) object.Object {
	if len(args) != 1 {
		return wrongArgCount("enumerate", "1", len(args))
	}
	var elements []object.Object
	switch v := args[0].(type) {
	case *object.List:
		elements = v.Elements
	case *object.Tuple:
		elements = v.Elements
	default:
		return wrongArgType("enumerate", 0, "a List or Tuple", args[0])
	}
	for _, elem := range elements {
		if _, ok := elem.(object.Hashable); !ok {
			return newError("enumerate: unhashable type %s cannot be a Tuple element", elem.Type())
		}
	}
	out := make([]object.Object, len(elements))
	for i, e := range elements {
		out[i] = object.NewTuple([]object.Object{&object.Integer{Value: int64(i)}, e})
	}
	return object.NewList(out)
}

// gridFn is `grid(s)` (SPEC.md §7) — parses a String into a row-major
// grid: a List of rows, each row itself a List of one-character
// Strings. The 2D counterpart to `lines` (String -> List of line
// Strings) and `chars` (one line -> List of one-character Strings)
// combined into a single call — `grid(unbox(path))` turns a raw
// AoC-style grid-puzzle input straight into something `at`/`setAt`/
// `neighbors4`/`neighbors8` can work with. Same line-splitting rule
// `lines` already uses (handles `\n` and `\r\n`, no trailing blank row
// for a string that ends in a newline).
func gridFn(args ...object.Object) object.Object {
	if len(args) != 1 {
		return wrongArgCount("grid", "1", len(args))
	}
	s, ok := args[0].(*object.String)
	if !ok {
		return wrongArgType("grid", 0, "a String", args[0])
	}
	var rows [][]object.Object
	scanner := bufio.NewScanner(strings.NewReader(s.Value))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		runes := []rune(scanner.Text())
		row := make([]object.Object, len(runes))
		for i, r := range runes {
			row[i] = &object.String{Value: string(r)}
		}
		rows = append(rows, row)
	}
	return object.NewGrid(rows, 0, 0)
}

// newGridFn is `newGrid()` (SPEC.md §7) — an empty Grid, the starting
// point for building one up entirely through `setAt` (a simulation
// with no fixed-size input to parse in the first place, e.g. Conway's
// Game of Life starting from a handful of live cells) rather than
// parsing one from text via `grid(s)`.
func newGridFn(args ...object.Object) object.Object {
	if len(args) != 0 {
		return wrongArgCount("newGrid", "0", len(args))
	}
	return &object.Grid{}
}

// gridPos validates pos as a (row, col) Tuple of two Integers — the
// coordinate representation every grid builtin here shares, chosen
// specifically because a Tuple is hashable (SPEC.md §2.3), so a
// position can go straight into a Set (visited cells) or a Map key
// (distances, costs) with no extra packing. index is the argument
// position pos was passed at, purely for wrongArgType's error message.
func gridPos(name string, index int, pos object.Object) (row, col int64, errObj *object.Error) {
	tup, ok := pos.(*object.Tuple)
	if !ok || len(tup.Elements) != 2 {
		return 0, 0, wrongArgType(name, index, "a (row, col) Tuple", pos)
	}
	r, ok := tup.Elements[0].(*object.Integer)
	if !ok {
		return 0, 0, newError("%s: row must be an Integer, got %s", name, tup.Elements[0].Type())
	}
	c, ok := tup.Elements[1].(*object.Integer)
	if !ok {
		return 0, 0, newError("%s: col must be an Integer, got %s", name, tup.Elements[1].Type())
	}
	return r.Value, c.Value, nil
}

// gridRowElements accepts either a List or Tuple as one grid row —
// `grid(s)`'s own output is always List-of-List, but nothing stops
// user code building a grid out of Tuple rows (e.g. after `map`),
// so `at` reads through either.
func gridRowElements(row object.Object) ([]object.Object, bool) {
	switch r := row.(type) {
	case *object.List:
		return r.Elements, true
	case *object.Tuple:
		return r.Elements, true
	default:
		return nil, false
	}
}

// atFn is `at(g, pos)` (SPEC.md §7) — bounds-checked read at (row,
// col). g is usually a Grid (`grid(s)`/`newGrid()`'s own output
// shape), but a plain List of row Lists/Tuples still works too — `at`
// predates Grid and this keeps hand-built nested-List grids (e.g. from
// `map`) usable without forcing a conversion. Out-of-range reads as
// nobox rather than an error, deliberately different from plain
// `g[row][col]` indexing (which errors via readIndex) — the same
// reasoning readIndex's own doc comment already gives for a missing
// Map key reading as nobox instead of erroring: grid code constantly
// needs to ask "is there a cell here" for a candidate neighbor near an
// edge, and nobox lets that be a plain equality/`?:` check instead of
// a hand-written bounds check before every single lookup.
func atFn(args ...object.Object) object.Object {
	if len(args) != 2 {
		return wrongArgCount("at", "2", len(args))
	}
	row, col, errObj := gridPos("at", 1, args[1])
	if errObj != nil {
		return errObj
	}

	if g, ok := args[0].(*object.Grid); ok {
		if v, ok := g.Get(int(row), int(col)); ok {
			return v
		}
		return object.NULL
	}

	g, ok := args[0].(*object.List)
	if !ok {
		return wrongArgType("at", 0, "a Grid or List", args[0])
	}
	if row < 0 || row >= int64(len(g.Elements)) {
		return object.NULL
	}
	rowElements, ok := gridRowElements(g.Elements[row])
	if !ok {
		return newError("at: row %d is %s, not a List or Tuple", row, g.Elements[row].Type())
	}
	if col < 0 || col >= int64(len(rowElements)) {
		return object.NULL
	}
	return rowElements[col]
}

// setAtFn is `setAt(g, pos, value)` (SPEC.md §7) — write into a Grid
// at (row, col), growing g in whichever direction(s) that coordinate
// falls outside its current bounds (including negative — a Grid's
// bounds can shift, see `object.Grid.Set`), `at`'s mutating
// counterpart. g must be a real Grid (`grid(s)`/`newGrid()`), not a
// plain List: growing "in place" needs somewhere to remember the
// shifted origin between calls, which a plain List has no room for
// (see object/grid.go's doc comment) — a plain nested List still works
// with `at` (read-only, no growing needed), just not `setAt`.
func setAtFn(args ...object.Object) object.Object {
	if len(args) != 3 {
		return wrongArgCount("setAt", "3", len(args))
	}
	g, ok := args[0].(*object.Grid)
	if !ok {
		return wrongArgType("setAt", 0, "a Grid (grid(s) or newGrid())", args[0])
	}
	row, col, errObj := gridPos("setAt", 1, args[1])
	if errObj != nil {
		return errObj
	}
	g.Set(int(row), int(col), args[2])
	return object.NULL
}

// gridBoundsFn is `gridBounds(g)` (SPEC.md §7) — g's current logical
// bounding box as a `(minRow, minCol, maxRow, maxCol)` Tuple, or nobox
// for an empty Grid (nothing's been written yet, so there's no box to
// report — the same "nothing here" answer `at` gives for a single
// missing cell). The only way to learn where a Grid's bounds actually
// are after any number of expanding `setAt` calls: a Grid deliberately
// isn't directly indexable/iterable the way a List is (see
// ARCHITECTURE.md), so `at`/`setAt`/`gridBounds` are the whole
// interface.
func gridBoundsFn(args ...object.Object) object.Object {
	if len(args) != 1 {
		return wrongArgCount("gridBounds", "1", len(args))
	}
	g, ok := args[0].(*object.Grid)
	if !ok {
		return wrongArgType("gridBounds", 0, "a Grid", args[0])
	}
	minRow, minCol, maxRow, maxCol, ok := g.Bounds()
	if !ok {
		return object.NULL
	}
	return object.NewTuple([]object.Object{
		object.NewInteger(int64(minRow)),
		object.NewInteger(int64(minCol)),
		object.NewInteger(int64(maxRow)),
		object.NewInteger(int64(maxCol)),
	})
}

// orthogonalOffsets/allOffsets are neighbors4/neighbors8's (dRow, dCol)
// offsets, each listed in row-major order over the 3x3 neighborhood
// (top-to-bottom, left-to-right) with (0, 0) — pos itself — skipped;
// neighbors4 is exactly allOffsets' four non-diagonal entries in that
// same relative order, so the two functions agree on "which direction
// comes first" wherever they overlap.
var orthogonalOffsets = [][2]int64{{-1, 0}, {0, -1}, {0, 1}, {1, 0}}
var allOffsets = [][2]int64{
	{-1, -1}, {-1, 0}, {-1, 1},
	{0, -1}, {0, 1},
	{1, -1}, {1, 0}, {1, 1},
}

// neighborsFn builds `neighbors4(pos)` / `neighbors8(pos)` (SPEC.md
// §7): given a (row, col) Tuple, returns the List of its 4 orthogonal
// or 8 orthogonal+diagonal neighbor positions, each as a Tuple, with no
// bounds checking against any particular grid — pos can be any (row,
// col), including ones that go negative or off some grid's edge, since
// this is pure coordinate arithmetic. Pair with `at` (which reads
// out-of-range as nobox rather than erroring) to filter to only the
// neighbors that actually exist on a given grid.
func neighborsFn(name string, offsets [][2]int64) object.BuiltinFunction {
	return func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return wrongArgCount(name, "1", len(args))
		}
		row, col, errObj := gridPos(name, 0, args[0])
		if errObj != nil {
			return errObj
		}
		out := make([]object.Object, len(offsets))
		for i, off := range offsets {
			out[i] = object.NewTuple([]object.Object{
				object.NewInteger(row + off[0]),
				object.NewInteger(col + off[1]),
			})
		}
		return object.NewList(out)
	}
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

// intsFn is `ints(s)` / `ints(list)` (SPEC.md §7). `ints(s)` splits a
// String into a List of single-digit Integers, one per character: the
// numeric-grid counterpart to `chars`, which does the same split but
// keeps each character as a one-rune String instead of parsing it.
// Every rune must be a decimal digit ('0'-'9') — a non-digit character
// is a runtime error, not silently skipped or mapped to some other
// value. `ints(list)` is a different, complementary shape: each
// element of list must be a String holding a (possibly multi-digit)
// base-10 integer — parsed the same way `int(x)` parses a String, not
// digit-by-digit — so `ints(split(line))` turns a line of
// whitespace-separated numbers straight into a List of Integers.
func intsFn(args ...object.Object) object.Object {
	if len(args) != 1 {
		return wrongArgCount("ints", "1", len(args))
	}
	switch v := args[0].(type) {
	case *object.String:
		runes := []rune(v.Value)
		out := make([]object.Object, len(runes))
		for i, r := range runes {
			if r < '0' || r > '9' {
				return newError("ints: %q is not a digit", string(r))
			}
			out[i] = object.NewInteger(int64(r - '0'))
		}
		return object.NewList(out)

	case *object.List:
		out := make([]object.Object, len(v.Elements))
		for i, elem := range v.Elements {
			s, ok := elem.(*object.String)
			if !ok {
				return newError("ints: element %d is %s, not a String", i, elem.Type())
			}
			n, err := strconv.ParseInt(strings.TrimSpace(s.Value), 10, 64)
			if err != nil {
				return newError("ints: cannot parse %q as an Integer", s.Value)
			}
			out[i] = object.NewInteger(n)
		}
		return object.NewList(out)

	default:
		return wrongArgType("ints", 0, "a String or List", args[0])
	}
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

// toppedFn is `topped(set, item)` (SPEC.md §7) — Set membership test.
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

// containsFn is `contains(collection, item)` (SPEC.md §7) — the
// general membership test `topped` doesn't cover on its own: a List or
// Tuple (linear scan, comparing with object.Equal — the exact same
// notion of "equal" `==` uses, so contains(xs, y)` agrees with
// `xs[i] == y` for whichever i), a Set (topped's own O(1) Hashable
// lookup, reused rather than re-implemented), or a Map (membership by
// *key*, the same "is this key present" question `sauce`/nobox
// indexing already answers less directly).
func containsFn(args ...object.Object) object.Object {
	if len(args) != 2 {
		return wrongArgCount("contains", "2", len(args))
	}
	switch v := args[0].(type) {
	case *object.List:
		return object.NativeBoolToBooleanObject(elementsContain(v.Elements, args[1]))
	case *object.Tuple:
		return object.NativeBoolToBooleanObject(elementsContain(v.Elements, args[1]))
	case *object.Set:
		return object.NativeBoolToBooleanObject(v.Has(args[1]))
	case *object.Map:
		_, ok := v.Get(args[1])
		return object.NativeBoolToBooleanObject(ok)
	default:
		return wrongArgType("contains", 0, "a List, Tuple, Set, or Map", args[0])
	}
}

// elementsContain reports whether item equals (object.Equal) any
// element of elements — the linear-scan half of contains(), shared by
// its List and Tuple cases since both compare contents the same way.
func elementsContain(elements []object.Object, item object.Object) bool {
	for _, e := range elements {
		if object.Equal(e, item) {
			return true
		}
	}
	return false
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

// joinFn is `join(list, sep)` (SPEC.md §7) — the natural counterpart
// to `split`: joins a List of Strings with sep between each. Every
// element must already be a String — this doesn't call str() on
// non-String elements, matching SPEC.md §6's "type conversion is
// always explicit" rule; convert with str() first if that's what's
// wanted (e.g. joining a List of Integers).
func joinFn(args ...object.Object) object.Object {
	if len(args) != 2 {
		return wrongArgCount("join", "2", len(args))
	}
	list, ok := args[0].(*object.List)
	if !ok {
		return wrongArgType("join", 0, "a List", args[0])
	}
	sep, ok := args[1].(*object.String)
	if !ok {
		return wrongArgType("join", 1, "a String", args[1])
	}
	parts := make([]string, len(list.Elements))
	for idx, elem := range list.Elements {
		s, ok := elem.(*object.String)
		if !ok {
			return newError("join: element %d is %s, not a String (use str() to convert first)", idx, elem.Type())
		}
		parts[idx] = s.Value
	}
	return &object.String{Value: strings.Join(parts, sep.Value)}
}

// splitFn is `split(s)` / `split(s, delim)` (SPEC.md §7). With one
// argument, splits on runs of whitespace — leading/trailing/repeated
// whitespace produces no empty entries, the same "just give me the
// words" behavior most languages' no-argument split has, handy for
// AoC input lines with irregular spacing. With a delim argument,
// splits on that literal string instead, preserving empty entries
// between consecutive delimiters ("a,,b" on "," is ["a", "", "b"], not
// ["a", "b"]) — a real CSV-style split, not whitespace-collapsing. An
// empty delim is a runtime error: the rune-by-rune behavior that
// would otherwise imply already has a name, chars(s).
func splitFn(args ...object.Object) object.Object {
	switch len(args) {
	case 1:
		s, ok := args[0].(*object.String)
		if !ok {
			return wrongArgType("split", 0, "a String", args[0])
		}
		fields := strings.Fields(s.Value)
		out := make([]object.Object, len(fields))
		for i, f := range fields {
			out[i] = &object.String{Value: f}
		}
		return object.NewList(out)
	case 2:
		s, ok := args[0].(*object.String)
		if !ok {
			return wrongArgType("split", 0, "a String", args[0])
		}
		delim, ok := args[1].(*object.String)
		if !ok {
			return wrongArgType("split", 1, "a String", args[1])
		}
		if delim.Value == "" {
			return newError("split: delimiter cannot be empty (use chars(s) to split into individual characters)")
		}
		parts := strings.Split(s.Value, delim.Value)
		out := make([]object.Object, len(parts))
		for i, p := range parts {
			out[i] = &object.String{Value: p}
		}
		return object.NewList(out)
	default:
		return wrongArgCount("split", "1 or 2", len(args))
	}
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
