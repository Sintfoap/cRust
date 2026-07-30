package builtins

import (
	"bytes"
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

func TestNewRegistersEveryBuiltin(t *testing.T) {
	table := New(&bytes.Buffer{})
	want := []string{
		"deliver", "slices", "sauce", "chars", "idiv",
		"gather", "sprinkle", "scrape", "topped", "combine", "shared", "strip",
	}
	for _, name := range want {
		if _, ok := table[name]; !ok {
			t.Errorf("New() table missing builtin %q", name)
		}
	}
}

func TestDeliver(t *testing.T) {
	var buf bytes.Buffer
	table := New(&buf)

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
	table := New(&buf)
	call(t, table, "deliver")
	if got := buf.String(); got != "\n" {
		t.Errorf("output = %q, want a bare newline", got)
	}
}

func TestSlices(t *testing.T) {
	table := New(&bytes.Buffer{})

	tests := []struct {
		name string
		arg  object.Object
		want int64
	}{
		{"string, rune count not byte count", &object.String{Value: "café"}, 4},
		{"empty string", &object.String{Value: ""}, 0},
		{"list", object.NewList([]object.Object{object.NewInteger(1), object.NewInteger(2)}), 2},
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
	table := New(&bytes.Buffer{})
	wantError(t, call(t, table, "slices"))
	wantError(t, call(t, table, "slices", object.NewInteger(1)))
}

func TestSauce(t *testing.T) {
	table := New(&bytes.Buffer{})

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
	table := New(&bytes.Buffer{})
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
	table := New(&bytes.Buffer{})
	wantError(t, call(t, table, "chars", object.NewInteger(1)))
}

func TestIdiv(t *testing.T) {
	table := New(&bytes.Buffer{})
	wantInteger(t, call(t, table, "idiv", object.NewInteger(7), object.NewInteger(2)), 3)
	wantInteger(t, call(t, table, "idiv", object.NewInteger(-7), object.NewInteger(2)), -3)
}

func TestIdivByZero(t *testing.T) {
	table := New(&bytes.Buffer{})
	errObj := wantError(t, call(t, table, "idiv", object.NewInteger(1), object.NewInteger(0)))
	if !strings.Contains(errObj.Message, "zero") {
		t.Errorf("Message = %q, want it to mention zero", errObj.Message)
	}
}

func TestIdivWrongTypes(t *testing.T) {
	table := New(&bytes.Buffer{})
	wantError(t, call(t, table, "idiv", &object.Float{Value: 1}, object.NewInteger(2)))
	wantError(t, call(t, table, "idiv", object.NewInteger(1), &object.Float{Value: 2}))
}

func TestGather(t *testing.T) {
	table := New(&bytes.Buffer{})
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
	table := New(&bytes.Buffer{})
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
	table := New(&bytes.Buffer{})
	set := object.NewSet()
	set.Add(object.NewInteger(1))

	wantBoolean(t, call(t, table, "topped", set, object.NewInteger(1)), true)
	wantBoolean(t, call(t, table, "topped", set, object.NewInteger(2)), false)
}

func setOf(vals ...int64) *object.Set {
	s := object.NewSet()
	for _, v := range vals {
		s.Add(object.NewInteger(v))
	}
	return s
}

func TestCombineSharedStrip(t *testing.T) {
	table := New(&bytes.Buffer{})
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
	table := New(&bytes.Buffer{})
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
	table := New(&bytes.Buffer{})
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
	table := New(&bytes.Buffer{})
	list := object.NewList([]object.Object{object.NewList(nil)})
	errObj := wantError(t, call(t, table, "gather", list))
	if !strings.Contains(errObj.Message, "unhashable") {
		t.Errorf("Message = %q, want it to mention unhashable", errObj.Message)
	}
}

func TestGatherWrongType(t *testing.T) {
	table := New(&bytes.Buffer{})
	wantError(t, call(t, table, "gather", object.NewInteger(1)))
}

func TestSprinkleUnhashableItem(t *testing.T) {
	table := New(&bytes.Buffer{})
	set := object.NewSet()
	errObj := wantError(t, call(t, table, "sprinkle", set, object.NewList(nil)))
	if !strings.Contains(errObj.Message, "unhashable") {
		t.Errorf("Message = %q, want it to mention unhashable", errObj.Message)
	}
}
