package builtins

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sintfoap/cRust/internal/object"
)

func call(t *testing.T, table map[string]*object.Builtin, name string, args ...object.Object) object.Object {
	t.Helper()
	b, ok := table[name]
	if !ok {
		t.Fatalf("no builtin registered under %q", name)
	}
	return b.Fn(args...)
}

// fakeCall stands in for internal/interpreter's real Call in tests
// that don't need actual user-defined *object.Function support (that
// needs environment/Eval machinery this package can't import — see
// the Call type doc comment). It can still invoke a *object.Builtin
// directly, which is enough to exercise map's own iterate-and-collect
// logic; anything else is reported as not callable, the same shape of
// error the real interpreter's applyFunction would give.
func fakeCall(fn object.Object, args []object.Object) object.Object {
	b, ok := fn.(*object.Builtin)
	if !ok {
		return newError("not a recipe: %s", fn.Type())
	}
	return b.Fn(args...)
}

func wantInteger(t *testing.T, got object.Object, want int64) {
	t.Helper()
	i, ok := got.(*object.Integer)
	if !ok {
		t.Fatalf("got %T (%v), want *object.Integer", got, got)
	}
	if i.Value != want {
		t.Errorf("Value = %d, want %d", i.Value, want)
	}
}

func wantError(t *testing.T, got object.Object) *object.Error {
	t.Helper()
	e, ok := got.(*object.Error)
	if !ok {
		t.Fatalf("got %T (%v), want *object.Error", got, got)
	}
	return e
}

func wantBoolean(t *testing.T, got object.Object, want bool) {
	t.Helper()
	b, ok := got.(*object.Boolean)
	if !ok {
		t.Fatalf("got %T (%v), want *object.Boolean", got, got)
	}
	if b.Value != want {
		t.Errorf("Value = %v, want %v", b.Value, want)
	}
}

func wantString(t *testing.T, got object.Object, want string) {
	t.Helper()
	s, ok := got.(*object.String)
	if !ok {
		t.Fatalf("got %T (%v), want *object.String", got, got)
	}
	if s.Value != want {
		t.Errorf("Value = %q, want %q", s.Value, want)
	}
}

func wantFloat(t *testing.T, got object.Object, want float64) {
	t.Helper()
	f, ok := got.(*object.Float)
	if !ok {
		t.Fatalf("got %T (%v), want *object.Float", got, got)
	}
	if f.Value != want {
		t.Errorf("Value = %v, want %v", f.Value, want)
	}
}

// wantTupleOfInts asserts got is a *object.Tuple holding exactly want,
// in order.
func wantTupleOfInts(t *testing.T, got object.Object, want ...int64) {
	t.Helper()
	tup, ok := got.(*object.Tuple)
	if !ok {
		t.Fatalf("got %T (%v), want *object.Tuple", got, got)
	}
	if len(tup.Elements) != len(want) {
		t.Fatalf("got %d elements, want %d", len(tup.Elements), len(want))
	}
	for i, w := range want {
		wantInteger(t, tup.Elements[i], w)
	}
}

func TestNewRegistersEveryBuiltin(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	want := []string{
		"deliver", "slices", "sauce", "chars", "ints", "push", "copy", "map", "find", "min", "max", "pizzasort", "combos", "enumerate", "join", "split", "idiv",
		"gather", "list", "tuple", "set", "freq", "sprinkle", "scrape", "topped", "contains", "combine", "shared", "strip",
		"unbox", "lines", "trim", "str", "int", "float", "bool",
		"grid", "newGrid", "at", "setAt", "gridBounds", "neighbors4", "neighbors8",
	}
	for _, name := range want {
		if _, ok := table[name]; !ok {
			t.Errorf("New() table missing builtin %q", name)
		}
	}
}

func TestDeliver(t *testing.T) {
	var buf bytes.Buffer
	table := New(&buf, strings.NewReader(""), fakeCall)

	result := call(t, table, "deliver", &object.String{Value: "hi"}, object.NewInteger(5))
	if result != object.NULL {
		t.Errorf("deliver(...) = %v, want NULL", result)
	}
	if got := buf.String(); got != "hi 5\n" {
		t.Errorf("output = %q, want %q", got, "hi 5\n")
	}
}

func TestDeliverNoArgs(t *testing.T) {
	var buf bytes.Buffer
	table := New(&buf, strings.NewReader(""), fakeCall)
	call(t, table, "deliver")
	if got := buf.String(); got != "\n" {
		t.Errorf("output = %q, want a bare newline", got)
	}
}

func TestSlices(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)

	tests := []struct {
		name string
		arg  object.Object
		want int64
	}{
		{"string, rune count not byte count", &object.String{Value: "café"}, 4},
		{"empty string", &object.String{Value: ""}, 0},
		{"list", object.NewList([]object.Object{object.NewInteger(1), object.NewInteger(2)}), 2},
		{"tuple", object.NewTuple([]object.Object{object.NewInteger(1), object.NewInteger(2), object.NewInteger(3)}), 3},
		{"set", func() object.Object { s := object.NewSet(); s.Add(object.NewInteger(1)); return s }(), 1},
		{"map", func() object.Object {
			m := object.NewMap()
			m.Set(&object.String{Value: "a"}, object.NewInteger(1))
			return m
		}(), 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantInteger(t, call(t, table, "slices", tt.arg), tt.want)
		})
	}
}

func TestSlicesWrongArgs(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "slices"))
	wantError(t, call(t, table, "slices", object.NewInteger(1)))
}

func TestSauce(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)

	got := call(t, table, "sauce", object.NULL, object.NewInteger(9))
	wantInteger(t, got, 9)

	got = call(t, table, "sauce", object.NewInteger(0), object.NewInteger(9))
	wantInteger(t, got, 0)

	got = call(t, table, "sauce", object.FALSE, object.NewInteger(9))
	if got != object.FALSE {
		t.Errorf("sauce(thin, 9) = %v, want thin unchanged (not nobox, so not replaced)", got)
	}
}

func TestChars(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	got := call(t, table, "chars", &object.String{Value: "ab"})
	list, ok := got.(*object.List)
	if !ok {
		t.Fatalf("got %T, want *object.List", got)
	}
	if len(list.Elements) != 2 {
		t.Fatalf("got %d elements, want 2", len(list.Elements))
	}
	if list.Elements[0].(*object.String).Value != "a" || list.Elements[1].(*object.String).Value != "b" {
		t.Errorf("elements = %v, want [a b]", list.Elements)
	}
}

func TestCharsWrongType(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "chars", object.NewInteger(1)))
}

func TestInts(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	got := call(t, table, "ints", &object.String{Value: "1029"})
	list, ok := got.(*object.List)
	if !ok {
		t.Fatalf("got %T, want *object.List", got)
	}
	want := []int64{1, 0, 2, 9}
	if len(list.Elements) != len(want) {
		t.Fatalf("got %d elements, want %d", len(list.Elements), len(want))
	}
	for i, w := range want {
		wantInteger(t, list.Elements[i], w)
	}
}

func TestIntsEmptyString(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	got := call(t, table, "ints", &object.String{Value: ""}).(*object.List)
	if len(got.Elements) != 0 {
		t.Errorf("got %d elements, want 0", len(got.Elements))
	}
}

func TestIntsNonDigit(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	errObj := wantError(t, call(t, table, "ints", &object.String{Value: "12a4"}))
	if !strings.Contains(errObj.Message, "not a digit") {
		t.Errorf("Message = %q, want it to mention not a digit", errObj.Message)
	}
}

func TestIntsWrongType(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "ints", object.NewInteger(1)))
}

func TestIntsOnList(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	list := object.NewList([]object.Object{
		&object.String{Value: "12"}, &object.String{Value: "3"}, &object.String{Value: "456"},
	})
	got := call(t, table, "ints", list).(*object.List)
	want := []int64{12, 3, 456}
	if len(got.Elements) != len(want) {
		t.Fatalf("got %d elements, want %d", len(got.Elements), len(want))
	}
	for i, w := range want {
		wantInteger(t, got.Elements[i], w)
	}
}

func TestIntsOnListWithSplit(t *testing.T) {
	// The motivating pattern: ints(split(line)).
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	split := call(t, table, "split", &object.String{Value: "10 20 30"})
	got := call(t, table, "ints", split).(*object.List)
	want := []int64{10, 20, 30}
	if len(got.Elements) != len(want) {
		t.Fatalf("got %d elements, want %d", len(got.Elements), len(want))
	}
	for i, w := range want {
		wantInteger(t, got.Elements[i], w)
	}
}

func TestIntsOnListNonStringElement(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	list := object.NewList([]object.Object{&object.String{Value: "1"}, object.NewInteger(2)})
	errObj := wantError(t, call(t, table, "ints", list))
	if !strings.Contains(errObj.Message, "not a String") {
		t.Errorf("Message = %q, want it to mention not a String", errObj.Message)
	}
}

func TestIntsOnListUnparsableElement(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	list := object.NewList([]object.Object{&object.String{Value: "abc"}})
	errObj := wantError(t, call(t, table, "ints", list))
	if !strings.Contains(errObj.Message, "cannot parse") {
		t.Errorf("Message = %q, want it to mention cannot parse", errObj.Message)
	}
}

func TestIntsWrongArgCount(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "ints"))
	wantError(t, call(t, table, "ints", object.NewInteger(1), object.NewInteger(2)))
}

func TestIdiv(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantInteger(t, call(t, table, "idiv", object.NewInteger(7), object.NewInteger(2)), 3)
	wantInteger(t, call(t, table, "idiv", object.NewInteger(-7), object.NewInteger(2)), -3)
}

func TestIdivByZero(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	errObj := wantError(t, call(t, table, "idiv", object.NewInteger(1), object.NewInteger(0)))
	if !strings.Contains(errObj.Message, "zero") {
		t.Errorf("Message = %q, want it to mention zero", errObj.Message)
	}
}

func TestIdivWrongTypes(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "idiv", &object.Float{Value: 1}, object.NewInteger(2)))
	wantError(t, call(t, table, "idiv", object.NewInteger(1), &object.Float{Value: 2}))
}

func TestGather(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	list := object.NewList([]object.Object{object.NewInteger(1), object.NewInteger(1), object.NewInteger(2)})
	got := call(t, table, "gather", list)
	set, ok := got.(*object.Set)
	if !ok {
		t.Fatalf("got %T, want *object.Set", got)
	}
	if set.Len() != 2 {
		t.Errorf("Len() = %d, want 2 (duplicates dropped)", set.Len())
	}
}

func TestSprinkleAndScrape(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	set := object.NewSet()

	call(t, table, "sprinkle", set, object.NewInteger(1))
	if !set.Has(object.NewInteger(1)) {
		t.Fatal("sprinkle did not add the item")
	}

	call(t, table, "scrape", set, object.NewInteger(1))
	if set.Has(object.NewInteger(1)) {
		t.Fatal("scrape did not remove the item")
	}

	// Removing an absent item is a no-op, not an error.
	result := call(t, table, "scrape", set, object.NewInteger(99))
	if result != object.NULL {
		t.Errorf("scrape of an absent item = %v, want NULL (no error)", result)
	}
}

func TestTopped(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	set := object.NewSet()
	set.Add(object.NewInteger(1))

	wantBoolean(t, call(t, table, "topped", set, object.NewInteger(1)), true)
	wantBoolean(t, call(t, table, "topped", set, object.NewInteger(2)), false)
}

func TestContainsList(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	list := object.NewList([]object.Object{object.NewInteger(1), object.NewInteger(2), object.NewInteger(3)})
	wantBoolean(t, call(t, table, "contains", list, object.NewInteger(2)), true)
	wantBoolean(t, call(t, table, "contains", list, object.NewInteger(9)), false)
}

func TestContainsListUsesValueEquality(t *testing.T) {
	// 1 (Integer) and 1.0 (Float) are one "number" category for ==
	// (SPEC.md §6) -- contains() has to agree, not just do a type-and-
	// value check.
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	list := object.NewList([]object.Object{object.NewInteger(1)})
	wantBoolean(t, call(t, table, "contains", list, &object.Float{Value: 1.0}), true)
}

func TestContainsTuple(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	tup := object.NewTuple([]object.Object{object.NewInteger(1), object.NewInteger(2)})
	wantBoolean(t, call(t, table, "contains", tup, object.NewInteger(2)), true)
	wantBoolean(t, call(t, table, "contains", tup, object.NewInteger(9)), false)
}

func TestContainsSet(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	set := setOf(1, 2, 3)
	wantBoolean(t, call(t, table, "contains", set, object.NewInteger(2)), true)
	wantBoolean(t, call(t, table, "contains", set, object.NewInteger(9)), false)
}

func TestContainsMapChecksKeys(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	m := object.NewMap()
	m.Set(&object.String{Value: "cheese"}, object.NewInteger(1))
	wantBoolean(t, call(t, table, "contains", m, &object.String{Value: "cheese"}), true)
	wantBoolean(t, call(t, table, "contains", m, &object.String{Value: "pepperoni"}), false)
	// The value, not the key, must not register as present.
	wantBoolean(t, call(t, table, "contains", m, object.NewInteger(1)), false)
}

func TestContainsWrongCollectionType(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "contains", object.NewInteger(1), object.NewInteger(1)))
}

func TestContainsWrongArgCount(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "contains", object.NewList(nil)))
}

func setOf(vals ...int64) *object.Set {
	s := object.NewSet()
	for _, v := range vals {
		s.Add(object.NewInteger(v))
	}
	return s
}

func TestCombineSharedStrip(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	a := setOf(1, 2, 3)
	b := setOf(2, 3, 4)

	union := call(t, table, "combine", a, b).(*object.Set)
	if union.Len() != 4 {
		t.Errorf("combine: Len() = %d, want 4", union.Len())
	}

	inter := call(t, table, "shared", a, b).(*object.Set)
	if inter.Len() != 2 || !inter.Has(object.NewInteger(2)) || !inter.Has(object.NewInteger(3)) {
		t.Errorf("shared: got %s, want {2, 3}", inter.Inspect())
	}

	diff := call(t, table, "strip", a, b).(*object.Set)
	if diff.Len() != 1 || !diff.Has(object.NewInteger(1)) {
		t.Errorf("strip: got %s, want {1}", diff.Inspect())
	}
}

func TestBinarySetBuiltinsRejectNonSets(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	for _, name := range []string{"combine", "shared", "strip", "sprinkle", "scrape", "topped"} {
		t.Run(name+" first arg", func(t *testing.T) {
			wantError(t, call(t, table, name, object.NewInteger(1), object.NewInteger(2)))
		})
	}
	for _, name := range []string{"combine", "shared", "strip"} {
		t.Run(name+" second arg", func(t *testing.T) {
			wantError(t, call(t, table, name, setOf(1), object.NewInteger(2)))
		})
	}
}

func TestWrongArgCounts(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	set := setOf(1)

	tests := []struct {
		name string
		args []object.Object
	}{
		{"sauce", []object.Object{object.NewInteger(1)}},
		{"chars", nil},
		{"idiv", []object.Object{object.NewInteger(1)}},
		{"gather", nil},
		{"sprinkle", []object.Object{set}},
		{"scrape", []object.Object{set}},
		{"topped", []object.Object{set}},
		{"combine", []object.Object{set}},
		{"shared", []object.Object{set}},
		{"strip", []object.Object{set}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantError(t, call(t, table, tt.name, tt.args...))
		})
	}
}

func TestGatherUnhashableElement(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	list := object.NewList([]object.Object{object.NewList(nil)})
	errObj := wantError(t, call(t, table, "gather", list))
	if !strings.Contains(errObj.Message, "unhashable") {
		t.Errorf("Message = %q, want it to mention unhashable", errObj.Message)
	}
}

func TestGatherWrongType(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "gather", object.NewInteger(1)))
}

func TestListConvertsTupleToList(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	tup := object.NewTuple([]object.Object{object.NewInteger(1), object.NewInteger(2)})
	got := call(t, table, "list", tup)
	list, ok := got.(*object.List)
	if !ok {
		t.Fatalf("got %T, want *object.List", got)
	}
	if len(list.Elements) != 2 {
		t.Fatalf("got %d elements, want 2", len(list.Elements))
	}
	wantInteger(t, list.Elements[0], 1)
}

func TestListConvertsSetToList(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	set := object.NewSet()
	set.Add(object.NewInteger(1))
	set.Add(object.NewInteger(2))
	got := call(t, table, "list", set)
	list, ok := got.(*object.List)
	if !ok {
		t.Fatalf("got %T, want *object.List", got)
	}
	if len(list.Elements) != 2 {
		t.Fatalf("got %d elements, want 2", len(list.Elements))
	}
}

func TestListOfListIsAShallowCopy(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	original := object.NewList([]object.Object{object.NewInteger(1)})
	got := call(t, table, "list", original).(*object.List)
	if got == original {
		t.Fatal("list(list) returned the same pointer, want a new List")
	}
	got.Elements = append(got.Elements, object.NewInteger(2))
	if len(original.Elements) != 1 {
		t.Errorf("mutating the result changed the original: len = %d, want 1", len(original.Elements))
	}
}

func TestListWrongType(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "list", object.NewInteger(1)))
}

func TestListWrongArgCount(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "list"))
	wantError(t, call(t, table, "list", object.NewList(nil), object.NewList(nil)))
}

func TestTupleConvertsListToTuple(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	list := object.NewList([]object.Object{object.NewInteger(1), object.NewInteger(2)})
	got := call(t, table, "tuple", list)
	tup, ok := got.(*object.Tuple)
	if !ok {
		t.Fatalf("got %T, want *object.Tuple", got)
	}
	if len(tup.Elements) != 2 {
		t.Fatalf("got %d elements, want 2", len(tup.Elements))
	}
	wantInteger(t, tup.Elements[1], 2)
}

func TestTupleConvertsSetToTuple(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	set := object.NewSet()
	set.Add(object.NewInteger(1))
	got := call(t, table, "tuple", set)
	tup, ok := got.(*object.Tuple)
	if !ok {
		t.Fatalf("got %T, want *object.Tuple", got)
	}
	if len(tup.Elements) != 1 {
		t.Fatalf("got %d elements, want 1", len(tup.Elements))
	}
}

func TestTupleUnhashableElementIsError(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	list := object.NewList([]object.Object{object.NewList(nil)})
	errObj := wantError(t, call(t, table, "tuple", list))
	if !strings.Contains(errObj.Message, "unhashable") {
		t.Errorf("Message = %q, want it to mention unhashable", errObj.Message)
	}
}

func TestTupleWrongType(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "tuple", object.NewInteger(1)))
}

func TestTupleWrongArgCount(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "tuple"))
	wantError(t, call(t, table, "tuple", object.NewList(nil), object.NewList(nil)))
}

func TestSetConvertsListToSetDroppingDuplicates(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	list := object.NewList([]object.Object{object.NewInteger(1), object.NewInteger(1), object.NewInteger(2)})
	got := call(t, table, "set", list)
	set, ok := got.(*object.Set)
	if !ok {
		t.Fatalf("got %T, want *object.Set", got)
	}
	if set.Len() != 2 {
		t.Errorf("Len() = %d, want 2 (duplicates dropped)", set.Len())
	}
}

func TestSetConvertsTupleToSet(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	tup := object.NewTuple([]object.Object{object.NewInteger(1), object.NewInteger(2)})
	got := call(t, table, "set", tup)
	set, ok := got.(*object.Set)
	if !ok {
		t.Fatalf("got %T, want *object.Set", got)
	}
	if set.Len() != 2 {
		t.Errorf("Len() = %d, want 2", set.Len())
	}
}

func TestSetOfSetIsANewObject(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	original := object.NewSet()
	original.Add(object.NewInteger(1))
	got := call(t, table, "set", original)
	set, ok := got.(*object.Set)
	if !ok {
		t.Fatalf("got %T, want *object.Set", got)
	}
	if set == original {
		t.Fatal("set(set) returned the same pointer, want a new Set")
	}
	if set.Len() != 1 {
		t.Errorf("Len() = %d, want 1", set.Len())
	}
}

func TestSetUnhashableElementIsError(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	list := object.NewList([]object.Object{object.NewList(nil)})
	errObj := wantError(t, call(t, table, "set", list))
	if !strings.Contains(errObj.Message, "unhashable") {
		t.Errorf("Message = %q, want it to mention unhashable", errObj.Message)
	}
}

func TestSetWrongType(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "set", object.NewInteger(1)))
}

func TestSetWrongArgCount(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "set"))
	wantError(t, call(t, table, "set", object.NewList(nil), object.NewList(nil)))
}

func TestFreqCountsOccurrences(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	list := object.NewList([]object.Object{
		&object.String{Value: "a"},
		&object.String{Value: "b"},
		&object.String{Value: "a"},
		&object.String{Value: "a"},
	})
	got := call(t, table, "freq", list)
	m, ok := got.(*object.Map)
	if !ok {
		t.Fatalf("got %T, want *object.Map", got)
	}
	a, ok := m.Get(&object.String{Value: "a"})
	if !ok {
		t.Fatal("map has no entry for \"a\"")
	}
	wantInteger(t, a, 3)
	b, ok := m.Get(&object.String{Value: "b"})
	if !ok {
		t.Fatal("map has no entry for \"b\"")
	}
	wantInteger(t, b, 1)
}

func TestFreqOfTuple(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	tup := object.NewTuple([]object.Object{object.NewInteger(1), object.NewInteger(1)})
	got := call(t, table, "freq", tup).(*object.Map)
	count, ok := got.Get(object.NewInteger(1))
	if !ok {
		t.Fatal("map has no entry for 1")
	}
	wantInteger(t, count, 2)
}

func TestFreqOfSetIsAllOnes(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	set := object.NewSet()
	set.Add(object.NewInteger(1))
	set.Add(object.NewInteger(2))
	got := call(t, table, "freq", set).(*object.Map)
	if len(got.Pairs) != 2 {
		t.Fatalf("got %d entries, want 2", len(got.Pairs))
	}
	count, ok := got.Get(object.NewInteger(1))
	if !ok {
		t.Fatal("map has no entry for 1")
	}
	wantInteger(t, count, 1)
}

func TestFreqOfEmptyListIsEmptyMap(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	got := call(t, table, "freq", object.NewList(nil)).(*object.Map)
	if len(got.Pairs) != 0 {
		t.Errorf("got %d entries, want 0", len(got.Pairs))
	}
}

func TestFreqUnhashableElementIsError(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	list := object.NewList([]object.Object{object.NewList(nil)})
	errObj := wantError(t, call(t, table, "freq", list))
	if !strings.Contains(errObj.Message, "unhashable") {
		t.Errorf("Message = %q, want it to mention unhashable", errObj.Message)
	}
}

func TestFreqWrongType(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "freq", object.NewInteger(1)))
}

func TestFreqWrongArgCount(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "freq"))
	wantError(t, call(t, table, "freq", object.NewList(nil), object.NewList(nil)))
}

func TestSprinkleUnhashableItem(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	set := object.NewSet()
	errObj := wantError(t, call(t, table, "sprinkle", set, object.NewList(nil)))
	if !strings.Contains(errObj.Message, "unhashable") {
		t.Errorf("Message = %q, want it to mention unhashable", errObj.Message)
	}
}

func TestPush(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	list := object.NewList([]object.Object{object.NewInteger(1), object.NewInteger(2)})

	result := call(t, table, "push", list, object.NewInteger(3))
	if result != object.NULL {
		t.Errorf("push(...) = %v, want NULL", result)
	}
	if len(list.Elements) != 3 {
		t.Fatalf("got %d elements, want 3", len(list.Elements))
	}
	wantInteger(t, list.Elements[2], 3)
}

func TestPushMutatesInPlace(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	original := object.NewList([]object.Object{object.NewInteger(1)})
	alias := original

	call(t, table, "push", original, object.NewInteger(2))

	if len(alias.Elements) != 2 {
		t.Errorf("push didn't mutate through alias: len = %d, want 2", len(alias.Elements))
	}
}

func TestPushWrongType(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "push", object.NewInteger(1), object.NewInteger(2)))
}

func TestPushWrongArgCount(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "push", object.NewList(nil)))
	wantError(t, call(t, table, "push"))
}

func TestCopyListIsIndependent(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	original := object.NewList([]object.Object{object.NewInteger(1), object.NewInteger(2)})

	got := call(t, table, "copy", original).(*object.List)
	if got == original {
		t.Fatal("copy(list) returned the same pointer, want a new List")
	}
	got.Elements = append(got.Elements, object.NewInteger(3))
	if len(original.Elements) != 2 {
		t.Errorf("mutating the copy changed the original: len = %d, want 2", len(original.Elements))
	}
}

func TestCopyScalarPassesThroughUnchanged(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantInteger(t, call(t, table, "copy", object.NewInteger(5)), 5)
}

func TestCopyWrongArgCount(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "copy"))
	wantError(t, call(t, table, "copy", object.NewInteger(1), object.NewInteger(2)))
}

var doubleFn = &object.Builtin{Fn: func(args ...object.Object) object.Object {
	return object.NewInteger(args[0].(*object.Integer).Value * 2)
}}

func TestMapOnList(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	list := object.NewList([]object.Object{object.NewInteger(1), object.NewInteger(2), object.NewInteger(3)})
	got := call(t, table, "map", list, doubleFn).(*object.List)
	want := []int64{2, 4, 6}
	if len(got.Elements) != len(want) {
		t.Fatalf("got %d elements, want %d", len(got.Elements), len(want))
	}
	for i, w := range want {
		wantInteger(t, got.Elements[i], w)
	}
}

func TestMapOnTuple(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	tup := object.NewTuple([]object.Object{object.NewInteger(5), object.NewInteger(6)})
	got := call(t, table, "map", tup, doubleFn).(*object.List)
	if len(got.Elements) != 2 {
		t.Fatalf("got %d elements, want 2", len(got.Elements))
	}
	wantInteger(t, got.Elements[0], 10)
	wantInteger(t, got.Elements[1], 12)
}

func TestMapEmptyList(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	got := call(t, table, "map", object.NewList(nil), doubleFn).(*object.List)
	if len(got.Elements) != 0 {
		t.Errorf("got %d elements, want 0", len(got.Elements))
	}
}

func TestMapPropagatesFnError(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	failFn := &object.Builtin{Fn: func(args ...object.Object) object.Object {
		return &object.Error{Message: "boom"}
	}}
	list := object.NewList([]object.Object{object.NewInteger(1)})
	errObj := wantError(t, call(t, table, "map", list, failFn))
	if errObj.Message != "boom" {
		t.Errorf("Message = %q, want %q", errObj.Message, "boom")
	}
}

func TestMapNotCallableIsError(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	list := object.NewList([]object.Object{object.NewInteger(1)})
	wantError(t, call(t, table, "map", list, object.NewInteger(5)))
}

func TestMapWrongCollectionType(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "map", object.NewInteger(1), doubleFn))
}

func TestMapWrongArgCount(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "map", object.NewList(nil)))
	wantError(t, call(t, table, "map"))
}

func TestFindOnListReturnsLowestIndexMatch(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	list := object.NewList([]object.Object{object.NewInteger(10), object.NewInteger(20), object.NewInteger(20), object.NewInteger(30)})
	got := call(t, table, "find", list, object.NewInteger(20))
	wantInteger(t, got, 1)
}

func TestFindOnTuple(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	tup := object.NewTuple([]object.Object{object.NewInteger(5), object.NewInteger(6), object.NewInteger(7)})
	got := call(t, table, "find", tup, object.NewInteger(6))
	wantInteger(t, got, 1)
}

func TestFindUsesValueEquality(t *testing.T) {
	// 2 (Integer) and 2.0 (Float) are one "number" category for ==
	// (SPEC.md §6) -- find() has to agree, the same way contains() does.
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	list := object.NewList([]object.Object{object.NewInteger(1), &object.Float{Value: 2.0}, object.NewInteger(3)})
	got := call(t, table, "find", list, object.NewInteger(2))
	wantInteger(t, got, 1)
}

func TestFindNoMatchReturnsNobox(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	list := object.NewList([]object.Object{object.NewInteger(1), object.NewInteger(3)})
	got := call(t, table, "find", list, object.NewInteger(99))
	if got != object.NULL {
		t.Errorf("find() = %v, want NULL (nobox) when nothing matches", got)
	}
}

func TestFindEmptyListReturnsNobox(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	got := call(t, table, "find", object.NewList(nil), object.NewInteger(1))
	if got != object.NULL {
		t.Errorf("find() = %v, want NULL (nobox) for an empty List", got)
	}
}

func TestFindWrongCollectionType(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "find", object.NewInteger(1), object.NewInteger(1)))
}

func TestFindWrongArgCount(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "find", object.NewList(nil)))
	wantError(t, call(t, table, "find"))
}

func TestMinMaxOfDirectArgs(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantInteger(t, call(t, table, "min", object.NewInteger(3), object.NewInteger(1), object.NewInteger(2)), 1)
	wantInteger(t, call(t, table, "max", object.NewInteger(3), object.NewInteger(1), object.NewInteger(2)), 3)
}

func TestMinMaxOfList(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	list := object.NewList([]object.Object{object.NewInteger(3), object.NewInteger(1), object.NewInteger(2)})
	wantInteger(t, call(t, table, "min", list), 1)
	wantInteger(t, call(t, table, "max", list), 3)
}

func TestMinMaxOfTuple(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	tup := object.NewTuple([]object.Object{object.NewInteger(3), object.NewInteger(1), object.NewInteger(2)})
	wantInteger(t, call(t, table, "min", tup), 1)
	wantInteger(t, call(t, table, "max", tup), 3)
}

func TestMinMaxMixesIntAndFloat(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	got := call(t, table, "max", object.NewInteger(1), &object.Float{Value: 2.5})
	wantFloat(t, got, 2.5)
}

func TestMinMaxOfStrings(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantString(t, call(t, table, "min", &object.String{Value: "banana"}, &object.String{Value: "apple"}), "apple")
	wantString(t, call(t, table, "max", &object.String{Value: "banana"}, &object.String{Value: "apple"}), "banana")
}

func TestMinMaxReturnsOriginalElement(t *testing.T) {
	// The winning element itself comes back, not a converted copy.
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	f := &object.Float{Value: 2.5}
	got := call(t, table, "max", object.NewInteger(1), f)
	if got != object.Object(f) {
		t.Errorf("got %v (%T), want the exact same Float value back", got, got)
	}
}

func TestMinMaxEmptyListIsError(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "min", object.NewList(nil)))
	wantError(t, call(t, table, "max", object.NewList(nil)))
}

func TestMinMaxSingleArgWrongType(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "min", object.NewInteger(1)))
}

func TestMinMaxNoArgsIsError(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "min"))
}

func TestMinMaxEqualElements(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantInteger(t, call(t, table, "min", object.NewInteger(2), object.NewInteger(2)), 2)
	wantInteger(t, call(t, table, "max", object.NewInteger(2), object.NewInteger(2)), 2)
}

func TestMinMaxMismatchedTypesIsError(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	errObj := wantError(t, call(t, table, "min", object.NewInteger(1), &object.String{Value: "x"}))
	if !strings.Contains(errObj.Message, "cannot compare") {
		t.Errorf("Message = %q, want it to mention \"cannot compare\"", errObj.Message)
	}
}

func TestPizzasortIntegers(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	list := object.NewList([]object.Object{object.NewInteger(5), object.NewInteger(3), object.NewInteger(8), object.NewInteger(1)})
	got := call(t, table, "pizzasort", list).(*object.List)
	want := []int64{1, 3, 5, 8}
	if len(got.Elements) != len(want) {
		t.Fatalf("got %d elements, want %d", len(got.Elements), len(want))
	}
	for i, w := range want {
		wantInteger(t, got.Elements[i], w)
	}
}

func TestPizzasortStrings(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	list := object.NewList([]object.Object{&object.String{Value: "banana"}, &object.String{Value: "apple"}, &object.String{Value: "cherry"}})
	got := call(t, table, "pizzasort", list).(*object.List)
	want := []string{"apple", "banana", "cherry"}
	if len(got.Elements) != len(want) {
		t.Fatalf("got %d elements, want %d", len(got.Elements), len(want))
	}
	for i, w := range want {
		wantString(t, got.Elements[i], w)
	}
}

func TestPizzasortMixesIntAndFloat(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	tup := object.NewTuple([]object.Object{&object.Float{Value: 3.5}, object.NewInteger(1), object.NewInteger(2)})
	got := call(t, table, "pizzasort", tup).(*object.List)
	if len(got.Elements) != 3 {
		t.Fatalf("got %d elements, want 3", len(got.Elements))
	}
	wantInteger(t, got.Elements[0], 1)
	wantInteger(t, got.Elements[1], 2)
	wantFloat(t, got.Elements[2], 3.5)
}

func TestPizzasortDoesNotMutateOriginal(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	list := object.NewList([]object.Object{object.NewInteger(3), object.NewInteger(1)})
	call(t, table, "pizzasort", list)
	wantInteger(t, list.Elements[0], 3)
	wantInteger(t, list.Elements[1], 1)
}

func TestPizzasortEmptyList(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	got := call(t, table, "pizzasort", object.NewList(nil)).(*object.List)
	if len(got.Elements) != 0 {
		t.Errorf("got %d elements, want 0", len(got.Elements))
	}
}

func TestPizzasortSingleElement(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	list := object.NewList([]object.Object{object.NewInteger(7)})
	got := call(t, table, "pizzasort", list).(*object.List)
	wantInteger(t, got.Elements[0], 7)
}

func TestPizzasortMismatchedTypesIsError(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	list := object.NewList([]object.Object{object.NewInteger(1), &object.String{Value: "x"}})
	errObj := wantError(t, call(t, table, "pizzasort", list))
	if !strings.Contains(errObj.Message, "cannot compare") {
		t.Errorf("Message = %q, want it to mention \"cannot compare\"", errObj.Message)
	}
}

func TestPizzasortWrongCollectionType(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "pizzasort", object.NewInteger(1)))
}

func TestPizzasortWrongArgCount(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "pizzasort"))
	wantError(t, call(t, table, "pizzasort", object.NewList(nil), object.NewList(nil)))
}

func TestCombosOfPairs(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	list := object.NewList([]object.Object{object.NewInteger(1), object.NewInteger(2), object.NewInteger(3)})
	got := call(t, table, "combos", list, object.NewInteger(2)).(*object.List)
	if len(got.Elements) != 3 {
		t.Fatalf("got %d combinations, want 3", len(got.Elements))
	}
	wantTupleOfInts(t, got.Elements[0], 1, 2)
	wantTupleOfInts(t, got.Elements[1], 1, 3)
	wantTupleOfInts(t, got.Elements[2], 2, 3)
}

func TestCombosOfTriples(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	list := object.NewList([]object.Object{object.NewInteger(1), object.NewInteger(2), object.NewInteger(3), object.NewInteger(4)})
	got := call(t, table, "combos", list, object.NewInteger(3)).(*object.List)
	want := [][]int64{{1, 2, 3}, {1, 2, 4}, {1, 3, 4}, {2, 3, 4}}
	if len(got.Elements) != len(want) {
		t.Fatalf("got %d combinations, want %d", len(got.Elements), len(want))
	}
	for i, w := range want {
		wantTupleOfInts(t, got.Elements[i], w...)
	}
}

func TestCombosOfTuple(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	tup := object.NewTuple([]object.Object{object.NewInteger(1), object.NewInteger(2), object.NewInteger(3)})
	got := call(t, table, "combos", tup, object.NewInteger(2)).(*object.List)
	if len(got.Elements) != 3 {
		t.Fatalf("got %d combinations, want 3", len(got.Elements))
	}
}

func TestCombosZero(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	list := object.NewList([]object.Object{object.NewInteger(1), object.NewInteger(2)})
	got := call(t, table, "combos", list, object.NewInteger(0)).(*object.List)
	if len(got.Elements) != 1 {
		t.Fatalf("got %d combinations, want 1 (the empty combination)", len(got.Elements))
	}
	wantTupleOfInts(t, got.Elements[0])
}

func TestCombosNGreaterThanLength(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	list := object.NewList([]object.Object{object.NewInteger(1), object.NewInteger(2)})
	got := call(t, table, "combos", list, object.NewInteger(5)).(*object.List)
	if len(got.Elements) != 0 {
		t.Errorf("got %d combinations, want 0", len(got.Elements))
	}
}

func TestCombosNegativeNIsError(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	list := object.NewList([]object.Object{object.NewInteger(1)})
	errObj := wantError(t, call(t, table, "combos", list, object.NewInteger(-1)))
	if !strings.Contains(errObj.Message, "non-negative") {
		t.Errorf("Message = %q, want it to mention \"non-negative\"", errObj.Message)
	}
}

func TestCombosUnhashableElementIsError(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	list := object.NewList([]object.Object{object.NewList(nil)})
	errObj := wantError(t, call(t, table, "combos", list, object.NewInteger(1)))
	if !strings.Contains(errObj.Message, "unhashable") {
		t.Errorf("Message = %q, want it to mention \"unhashable\"", errObj.Message)
	}
}

func TestCombosWrongCollectionType(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "combos", object.NewInteger(1), object.NewInteger(2)))
}

func TestCombosNNotInteger(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	list := object.NewList([]object.Object{object.NewInteger(1)})
	wantError(t, call(t, table, "combos", list, &object.String{Value: "2"}))
}

func TestCombosWrongArgCount(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "combos", object.NewList(nil)))
}

func TestEnumerateOfList(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	list := object.NewList([]object.Object{&object.String{Value: "a"}, &object.String{Value: "b"}, &object.String{Value: "c"}})
	got := call(t, table, "enumerate", list).(*object.List)
	if len(got.Elements) != 3 {
		t.Fatalf("got %d pairs, want 3", len(got.Elements))
	}
	for i, want := range []string{"a", "b", "c"} {
		pair, ok := got.Elements[i].(*object.Tuple)
		if !ok {
			t.Fatalf("element %d = %T, want *object.Tuple", i, got.Elements[i])
		}
		if len(pair.Elements) != 2 {
			t.Fatalf("pair %d has %d elements, want 2", i, len(pair.Elements))
		}
		idx, ok := pair.Elements[0].(*object.Integer)
		if !ok || idx.Value != int64(i) {
			t.Errorf("pair %d's index = %v, want %d", i, pair.Elements[0], i)
		}
		wantString(t, pair.Elements[1], want)
	}
}

func TestEnumerateOfTuple(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	tup := object.NewTuple([]object.Object{object.NewInteger(10), object.NewInteger(20)})
	got := call(t, table, "enumerate", tup).(*object.List)
	if len(got.Elements) != 2 {
		t.Fatalf("got %d pairs, want 2", len(got.Elements))
	}
	wantTupleOfInts(t, got.Elements[0], 0, 10)
	wantTupleOfInts(t, got.Elements[1], 1, 20)
}

func TestEnumerateEmptyList(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	got := call(t, table, "enumerate", object.NewList(nil)).(*object.List)
	if len(got.Elements) != 0 {
		t.Errorf("got %d pairs, want 0 for an empty List", len(got.Elements))
	}
}

func TestEnumerateUnhashableElementIsError(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	list := object.NewList([]object.Object{object.NewList(nil)})
	errObj := wantError(t, call(t, table, "enumerate", list))
	if !strings.Contains(errObj.Message, "unhashable") {
		t.Errorf("Message = %q, want it to mention \"unhashable\"", errObj.Message)
	}
}

func TestEnumerateWrongCollectionType(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "enumerate", object.NewInteger(1)))
}

func TestEnumerateWrongArgCount(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "enumerate", object.NewList(nil), object.NewList(nil)))
}

func TestGridParsesRowsAndCols(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	got := call(t, table, "grid", &object.String{Value: "abc\nde\n"}).(*object.Grid)
	if got.Height() != 2 {
		t.Fatalf("got %d rows, want 2", got.Height())
	}
	wantString(t, got.Rows[0][0], "a")
	wantString(t, got.Rows[0][2], "c")
	if len(got.Rows[1]) != 2 {
		t.Fatalf("got %d cols in row 1, want 2", len(got.Rows[1]))
	}
}

func TestGridNoTrailingBlankRow(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	got := call(t, table, "grid", &object.String{Value: "ab\ncd\n"}).(*object.Grid)
	if got.Height() != 2 {
		t.Errorf("got %d rows, want 2 (no trailing blank row)", got.Height())
	}
}

func TestGridReturnsZeroOffset(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	got := call(t, table, "grid", &object.String{Value: "ab"}).(*object.Grid)
	if got.RowOffset != 0 || got.ColOffset != 0 {
		t.Errorf("RowOffset/ColOffset = %d/%d, want 0/0", got.RowOffset, got.ColOffset)
	}
}

func TestGridWrongArgType(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "grid", object.NewInteger(1)))
}

func TestGridWrongArgCount(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "grid"))
}

func testGrid(t *testing.T, table map[string]*object.Builtin, s string) object.Object {
	t.Helper()
	return call(t, table, "grid", &object.String{Value: s})
}

func TestNewGridIsEmpty(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	got := call(t, table, "newGrid").(*object.Grid)
	if got.Height() != 0 || got.Width() != 0 {
		t.Errorf("Height/Width = %d/%d, want 0/0", got.Height(), got.Width())
	}
}

func TestNewGridWrongArgCount(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "newGrid", object.NewInteger(1)))
}

func TestGridBoundsOnEmptyGridIsNobox(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	g := call(t, table, "newGrid")
	if got := call(t, table, "gridBounds", g); got != object.NULL {
		t.Errorf("gridBounds(newGrid()) = %v, want nobox", got)
	}
}

func TestGridBoundsAfterParsing(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	g := testGrid(t, table, "abc\ndef")
	got := call(t, table, "gridBounds", g).(*object.Tuple)
	wantInteger(t, got.Elements[0], 0)
	wantInteger(t, got.Elements[1], 0)
	wantInteger(t, got.Elements[2], 1)
	wantInteger(t, got.Elements[3], 2)
}

func TestGridBoundsAfterExpansion(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	g := testGrid(t, table, "a")
	call(t, table, "setAt", g, object.NewTuple([]object.Object{object.NewInteger(-3), object.NewInteger(4)}), &object.String{Value: "z"})
	got := call(t, table, "gridBounds", g).(*object.Tuple)
	wantInteger(t, got.Elements[0], -3)
	wantInteger(t, got.Elements[1], 0)
	wantInteger(t, got.Elements[2], 0)
	wantInteger(t, got.Elements[3], 4)
}

func TestGridBoundsWrongType(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "gridBounds", object.NewInteger(1)))
}

func TestGridBoundsWrongArgCount(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "gridBounds"))
}

func TestAtInBounds(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	g := testGrid(t, table, "abc\ndef\nghi")
	pos := object.NewTuple([]object.Object{object.NewInteger(1), object.NewInteger(2)})
	wantString(t, call(t, table, "at", g, pos), "f")
}

func TestAtOutOfBoundsReadsAsNobox(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	g := testGrid(t, table, "abc\ndef")
	cases := []*object.Tuple{
		object.NewTuple([]object.Object{object.NewInteger(-1), object.NewInteger(0)}),
		object.NewTuple([]object.Object{object.NewInteger(5), object.NewInteger(0)}),
		object.NewTuple([]object.Object{object.NewInteger(0), object.NewInteger(-1)}),
		object.NewTuple([]object.Object{object.NewInteger(0), object.NewInteger(5)}),
	}
	for _, pos := range cases {
		got := call(t, table, "at", g, pos)
		if got != object.NULL {
			t.Errorf("at(g, %s) = %v, want nobox", pos.Inspect(), got)
		}
	}
}

func TestAtWrongCollectionType(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	pos := object.NewTuple([]object.Object{object.NewInteger(0), object.NewInteger(0)})
	wantError(t, call(t, table, "at", object.NewInteger(1), pos))
}

func TestAtPosNotATuple(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	g := testGrid(t, table, "ab")
	wantError(t, call(t, table, "at", g, object.NewInteger(0)))
}

func TestAtPosWrongArity(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	g := testGrid(t, table, "ab")
	pos := object.NewTuple([]object.Object{object.NewInteger(0)})
	wantError(t, call(t, table, "at", g, pos))
}

func TestAtPosNonIntegerElements(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	g := testGrid(t, table, "ab")
	pos := object.NewTuple([]object.Object{&object.String{Value: "0"}, object.NewInteger(0)})
	wantError(t, call(t, table, "at", g, pos))
}

func TestAtPosColNonInteger(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	g := testGrid(t, table, "ab")
	pos := object.NewTuple([]object.Object{object.NewInteger(0), &object.String{Value: "0"}})
	wantError(t, call(t, table, "at", g, pos))
}

func TestAtWrongArgCount(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	g := testGrid(t, table, "ab")
	wantError(t, call(t, table, "at", g))
}

func TestAtRowNotAListOrTuple(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	g := object.NewList([]object.Object{object.NewInteger(1)})
	pos := object.NewTuple([]object.Object{object.NewInteger(0), object.NewInteger(0)})
	errObj := wantError(t, call(t, table, "at", g, pos))
	if !strings.Contains(errObj.Message, "not a List or Tuple") {
		t.Errorf("Message = %q, want it to mention \"not a List or Tuple\"", errObj.Message)
	}
}

func TestAtOnPlainListOutOfRange(t *testing.T) {
	// The legacy List-of-List path (grid() no longer returns this
	// shape, but at() still accepts one for backward compatibility)
	// still reads out-of-range as nobox, same as the Grid path.
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	g := object.NewList([]object.Object{
		object.NewList([]object.Object{object.NewInteger(1), object.NewInteger(2)}),
	})
	rowOOB := object.NewTuple([]object.Object{object.NewInteger(5), object.NewInteger(0)})
	if got := call(t, table, "at", g, rowOOB); got != object.NULL {
		t.Errorf("at(list, row-out-of-range) = %v, want nobox", got)
	}
	colOOB := object.NewTuple([]object.Object{object.NewInteger(0), object.NewInteger(5)})
	if got := call(t, table, "at", g, colOOB); got != object.NULL {
		t.Errorf("at(list, col-out-of-range) = %v, want nobox", got)
	}
}

func TestAtOnTupleRow(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	g := object.NewList([]object.Object{
		object.NewTuple([]object.Object{object.NewInteger(1), object.NewInteger(2)}),
	})
	pos := object.NewTuple([]object.Object{object.NewInteger(0), object.NewInteger(1)})
	wantInteger(t, call(t, table, "at", g, pos), 2)
}

func TestSetAtMutatesInPlace(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	g := testGrid(t, table, "ab\ncd")
	pos := object.NewTuple([]object.Object{object.NewInteger(1), object.NewInteger(0)})
	call(t, table, "setAt", g, pos, &object.String{Value: "Z"})
	wantString(t, call(t, table, "at", g, pos), "Z")
}

func TestSetAtExpandsPositivelyInsteadOfErroring(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	g := testGrid(t, table, "ab").(*object.Grid)
	pos := object.NewTuple([]object.Object{object.NewInteger(9), object.NewInteger(9)})
	result := call(t, table, "setAt", g, pos, &object.String{Value: "Z"})
	if _, isErr := result.(*object.Error); isErr {
		t.Fatalf("setAt out of range returned an Error, want it to expand instead: %v", result)
	}
	wantString(t, call(t, table, "at", g, pos), "Z")
	if g.Height() != 10 || g.Width() != 10 {
		t.Errorf("Height/Width after expanding to (9,9) = %d/%d, want 10/10", g.Height(), g.Width())
	}
}

func TestSetAtExpandsNegativelyAndPreservesExistingCells(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	g := testGrid(t, table, "ab\ncd").(*object.Grid)
	original := object.NewTuple([]object.Object{object.NewInteger(0), object.NewInteger(0)})

	pos := object.NewTuple([]object.Object{object.NewInteger(-2), object.NewInteger(-2)})
	call(t, table, "setAt", g, pos, &object.String{Value: "Z"})

	wantString(t, call(t, table, "at", g, pos), "Z")
	// The cell that used to be (0,0) must still read the same value --
	// the whole point of tracking an offset instead of just erroring.
	wantString(t, call(t, table, "at", g, original), "a")
}

func TestSetAtNewCellsDefaultToNobox(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	g := testGrid(t, table, "a").(*object.Grid)
	call(t, table, "setAt", g, object.NewTuple([]object.Object{object.NewInteger(0), object.NewInteger(3)}), &object.String{Value: "z"})
	skipped := object.NewTuple([]object.Object{object.NewInteger(0), object.NewInteger(1)})
	if got := call(t, table, "at", g, skipped); got != object.NULL {
		t.Errorf("at(g, (0,1)) (never written, just grown past) = %v, want nobox", got)
	}
}

func TestSetAtRequiresAGrid(t *testing.T) {
	// The old plain-List setAt path is gone entirely: growing "in
	// place" needs a persistent offset a plain List has no room for
	// (see object/grid.go), so setAt now only accepts what grid()/
	// newGrid() actually return.
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	list := object.NewList([]object.Object{
		object.NewList([]object.Object{object.NewInteger(1), object.NewInteger(2)}),
	})
	pos := object.NewTuple([]object.Object{object.NewInteger(0), object.NewInteger(0)})
	errObj := wantError(t, call(t, table, "setAt", list, pos, object.NewInteger(9)))
	if !strings.Contains(errObj.Message, "Grid") {
		t.Errorf("Message = %q, want it to mention Grid", errObj.Message)
	}
}

func TestSetAtWrongCollectionType(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	pos := object.NewTuple([]object.Object{object.NewInteger(0), object.NewInteger(0)})
	wantError(t, call(t, table, "setAt", object.NewInteger(1), pos, object.NewInteger(9)))
}

func TestSetAtPosNotATuple(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	g := testGrid(t, table, "ab")
	wantError(t, call(t, table, "setAt", g, object.NewInteger(0), object.NewInteger(9)))
}

func TestSetAtWrongArgCount(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	g := testGrid(t, table, "ab")
	wantError(t, call(t, table, "setAt", g))
}

func TestNeighbors4(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	pos := object.NewTuple([]object.Object{object.NewInteger(2), object.NewInteger(2)})
	got := call(t, table, "neighbors4", pos).(*object.List)
	want := [][2]int64{{1, 2}, {2, 1}, {2, 3}, {3, 2}}
	if len(got.Elements) != len(want) {
		t.Fatalf("got %d neighbors, want %d", len(got.Elements), len(want))
	}
	for i, w := range want {
		wantTupleOfInts(t, got.Elements[i], w[0], w[1])
	}
}

func TestNeighbors8(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	pos := object.NewTuple([]object.Object{object.NewInteger(2), object.NewInteger(2)})
	got := call(t, table, "neighbors8", pos).(*object.List)
	if len(got.Elements) != 8 {
		t.Fatalf("got %d neighbors, want 8", len(got.Elements))
	}
	want := [][2]int64{{1, 1}, {1, 2}, {1, 3}, {2, 1}, {2, 3}, {3, 1}, {3, 2}, {3, 3}}
	for i, w := range want {
		wantTupleOfInts(t, got.Elements[i], w[0], w[1])
	}
}

func TestNeighborsAtNegativeOrigin(t *testing.T) {
	// Pure coordinate arithmetic -- no bounds checking, so negative
	// positions come back unfiltered (pair with `at`'s nobox-on-miss to
	// filter against a real grid).
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	pos := object.NewTuple([]object.Object{object.NewInteger(0), object.NewInteger(0)})
	got := call(t, table, "neighbors4", pos).(*object.List)
	wantTupleOfInts(t, got.Elements[0], -1, 0)
}

func TestNeighborsWrongArgCount(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "neighbors4"))
	wantError(t, call(t, table, "neighbors8"))
}

func TestNeighborsPosNotATuple(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "neighbors4", object.NewInteger(1)))
}

func TestUnboxNoArgReadsStdin(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader("puzzle input\nline two\n"), fakeCall)
	wantString(t, call(t, table, "unbox"), "puzzle input\nline two\n")
}

func TestUnboxWithPathReadsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(path, []byte("42\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantString(t, call(t, table, "unbox", &object.String{Value: path}), "42\n")
}

func TestUnboxMissingFile(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "unbox", &object.String{Value: "/no/such/file.txt"}))
}

func TestUnboxWrongArgCount(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "unbox", object.NewInteger(1), object.NewInteger(2)))
}

func TestUnboxWrongArgType(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "unbox", object.NewInteger(1)))
}

func TestLines(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)

	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"no trailing newline", "a\nb\nc", []string{"a", "b", "c"}},
		{"trailing newline produces no extra blank entry", "a\nb\n", []string{"a", "b"}},
		{"CRLF", "a\r\nb\r\n", []string{"a", "b"}},
		{"empty string", "", nil},
		{"single line", "only", []string{"only"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := call(t, table, "lines", &object.String{Value: tt.input}).(*object.List)
			if len(result.Elements) != len(tt.want) {
				t.Fatalf("got %d lines, want %d: %v", len(result.Elements), len(tt.want), result.Inspect())
			}
			for i, w := range tt.want {
				wantString(t, result.Elements[i], w)
			}
		})
	}
}

func TestLinesWrongType(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "lines", object.NewInteger(1)))
}

func wantStringList(t *testing.T, got object.Object, want []string) {
	t.Helper()
	list, ok := got.(*object.List)
	if !ok {
		t.Fatalf("got %T, want *object.List", got)
	}
	if len(list.Elements) != len(want) {
		t.Fatalf("got %d elements, want %d: %v", len(list.Elements), len(want), list.Inspect())
	}
	for i, w := range want {
		wantString(t, list.Elements[i], w)
	}
}

func TestJoin(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	list := object.NewList([]object.Object{
		&object.String{Value: "a"}, &object.String{Value: "b"}, &object.String{Value: "c"},
	})
	wantString(t, call(t, table, "join", list, &object.String{Value: ", "}), "a, b, c")
}

func TestJoinEmptyList(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantString(t, call(t, table, "join", object.NewList(nil), &object.String{Value: ","}), "")
}

func TestJoinSingleElement(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	list := object.NewList([]object.Object{&object.String{Value: "only"}})
	wantString(t, call(t, table, "join", list, &object.String{Value: ","}), "only")
}

func TestJoinRoundTripsWithSplit(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	split := call(t, table, "split", &object.String{Value: "a,b,c"}, &object.String{Value: ","})
	joined := call(t, table, "join", split, &object.String{Value: ","})
	wantString(t, joined, "a,b,c")
}

func TestJoinNonStringElementIsError(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	list := object.NewList([]object.Object{&object.String{Value: "a"}, object.NewInteger(1)})
	errObj := wantError(t, call(t, table, "join", list, &object.String{Value: ","}))
	if !strings.Contains(errObj.Message, "str()") {
		t.Errorf("Message = %q, want it to mention str()", errObj.Message)
	}
}

func TestJoinWrongType(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "join", object.NewInteger(1), &object.String{Value: ","}))
	wantError(t, call(t, table, "join", object.NewList(nil), object.NewInteger(1)))
}

func TestJoinWrongArgCount(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "join", object.NewList(nil)))
	wantError(t, call(t, table, "join"))
}

func TestSplitNoDelimCollapsesWhitespace(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"single spaces", "1 2 3", []string{"1", "2", "3"}},
		{"irregular spacing", "  1   2\t3  ", []string{"1", "2", "3"}},
		{"newlines too", "a\nb\nc", []string{"a", "b", "c"}},
		{"all whitespace", "   ", []string{}},
		{"empty", "", []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantStringList(t, call(t, table, "split", &object.String{Value: tt.input}), tt.want)
		})
	}
}

func TestSplitWithDelim(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	tests := []struct {
		name  string
		input string
		delim string
		want  []string
	}{
		{"comma", "a,b,c", ",", []string{"a", "b", "c"}},
		{"preserves empty entries", "a,,b", ",", []string{"a", "", "b"}},
		{"multi-char delim", "a::b::c", "::", []string{"a", "b", "c"}},
		{"delim not found", "abc", ",", []string{"abc"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := call(t, table, "split", &object.String{Value: tt.input}, &object.String{Value: tt.delim})
			wantStringList(t, result, tt.want)
		})
	}
}

func TestSplitEmptyDelimIsError(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	errObj := wantError(t, call(t, table, "split", &object.String{Value: "abc"}, &object.String{Value: ""}))
	if !strings.Contains(errObj.Message, "chars") {
		t.Errorf("Message = %q, want it to point at chars(s)", errObj.Message)
	}
}

func TestSplitWrongType(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "split", object.NewInteger(1)))
	wantError(t, call(t, table, "split", &object.String{Value: "a"}, object.NewInteger(1)))
}

func TestSplitWrongArgCount(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "split"))
	wantError(t, call(t, table, "split", &object.String{Value: "a"}, &object.String{Value: "b"}, &object.String{Value: "c"}))
}

func TestTrim(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"leading and trailing spaces", "  hello  ", "hello"},
		{"trailing newline (unbox() output)", "42\n", "42"},
		{"tabs and mixed whitespace", "\t\n hi \n\t", "hi"},
		{"nothing to trim", "clean", "clean"},
		{"all whitespace", "   \n\t  ", ""},
		{"interior whitespace preserved", "  a b  ", "a b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantString(t, call(t, table, "trim", &object.String{Value: tt.input}), tt.want)
		})
	}
}

func TestTrimWrongType(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "trim", object.NewInteger(1)))
}

func TestStr(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)

	wantString(t, call(t, table, "str", object.NewInteger(42)), "42")
	wantString(t, call(t, table, "str", &object.Float{Value: 3.5}), "3.5")
	wantString(t, call(t, table, "str", object.TRUE), "stuffed")
	wantString(t, call(t, table, "str", object.NULL), "nobox")
	wantString(t, call(t, table, "str", &object.String{Value: "already"}), "already")
	wantString(t, call(t, table, "str", object.NewList([]object.Object{object.NewInteger(1), object.NewInteger(2)})), "[1, 2]")
}

func TestInt(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)

	wantInteger(t, call(t, table, "int", object.NewInteger(7)), 7)
	wantInteger(t, call(t, table, "int", &object.Float{Value: 3.9}), 3)
	wantInteger(t, call(t, table, "int", &object.Float{Value: -3.9}), -3)
	wantInteger(t, call(t, table, "int", &object.String{Value: "123"}), 123)
	wantInteger(t, call(t, table, "int", &object.String{Value: "  -5  "}), -5)
}

func TestIntParseError(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	errObj := wantError(t, call(t, table, "int", &object.String{Value: "not a number"}))
	if !strings.Contains(errObj.Message, "cannot parse") {
		t.Errorf("Message = %q, want it to mention cannot parse", errObj.Message)
	}
}

func TestIntWrongType(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "int", object.TRUE))
}

func TestFloat(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)

	wantFloat(t, call(t, table, "float", &object.Float{Value: 2.5}), 2.5)
	wantFloat(t, call(t, table, "float", object.NewInteger(4)), 4.0)
	wantFloat(t, call(t, table, "float", &object.String{Value: "3.14"}), 3.14)
}

func TestFloatParseError(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "float", &object.String{Value: "nope"}))
}

func TestFloatWrongType(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	wantError(t, call(t, table, "float", object.TRUE))
}

func TestBool(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)

	tests := []struct {
		name string
		arg  object.Object
		want bool
	}{
		{"stuffed stays stuffed", object.TRUE, true},
		{"thin stays thin", object.FALSE, false},
		{"nobox is falsy", object.NULL, false},
		{"zero is truthy", object.NewInteger(0), true},
		{"empty string is truthy", &object.String{Value: ""}, true},
		{"empty list is truthy", object.NewList(nil), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantBoolean(t, call(t, table, "bool", tt.arg), tt.want)
		})
	}
}

func TestConversionBuiltinsWrongArgCounts(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""), fakeCall)
	for _, name := range []string{"str", "int", "float", "bool", "lines", "trim"} {
		t.Run(name, func(t *testing.T) {
			wantError(t, call(t, table, name))
			wantError(t, call(t, table, name, object.NewInteger(1), object.NewInteger(2)))
		})
	}
}
