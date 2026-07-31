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

func TestNewRegistersEveryBuiltin(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""))
	want := []string{
		"deliver", "slices", "sauce", "chars", "ints", "push", "join", "split", "idiv",
		"gather", "sprinkle", "scrape", "topped", "combine", "shared", "strip",
		"unbox", "lines", "trim", "str", "int", "float", "bool",
	}
	for _, name := range want {
		if _, ok := table[name]; !ok {
			t.Errorf("New() table missing builtin %q", name)
		}
	}
}

func TestDeliver(t *testing.T) {
	var buf bytes.Buffer
	table := New(&buf, strings.NewReader(""))

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
	table := New(&buf, strings.NewReader(""))
	call(t, table, "deliver")
	if got := buf.String(); got != "\n" {
		t.Errorf("output = %q, want a bare newline", got)
	}
}

func TestSlices(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""))

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
	table := New(&bytes.Buffer{}, strings.NewReader(""))
	wantError(t, call(t, table, "slices"))
	wantError(t, call(t, table, "slices", object.NewInteger(1)))
}

func TestSauce(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""))

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
	table := New(&bytes.Buffer{}, strings.NewReader(""))
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
	table := New(&bytes.Buffer{}, strings.NewReader(""))
	wantError(t, call(t, table, "chars", object.NewInteger(1)))
}

func TestInts(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""))
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
	table := New(&bytes.Buffer{}, strings.NewReader(""))
	got := call(t, table, "ints", &object.String{Value: ""}).(*object.List)
	if len(got.Elements) != 0 {
		t.Errorf("got %d elements, want 0", len(got.Elements))
	}
}

func TestIntsNonDigit(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""))
	errObj := wantError(t, call(t, table, "ints", &object.String{Value: "12a4"}))
	if !strings.Contains(errObj.Message, "not a digit") {
		t.Errorf("Message = %q, want it to mention not a digit", errObj.Message)
	}
}

func TestIntsWrongType(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""))
	wantError(t, call(t, table, "ints", object.NewInteger(1)))
}

func TestIntsWrongArgCount(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""))
	wantError(t, call(t, table, "ints"))
	wantError(t, call(t, table, "ints", object.NewInteger(1), object.NewInteger(2)))
}

func TestIdiv(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""))
	wantInteger(t, call(t, table, "idiv", object.NewInteger(7), object.NewInteger(2)), 3)
	wantInteger(t, call(t, table, "idiv", object.NewInteger(-7), object.NewInteger(2)), -3)
}

func TestIdivByZero(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""))
	errObj := wantError(t, call(t, table, "idiv", object.NewInteger(1), object.NewInteger(0)))
	if !strings.Contains(errObj.Message, "zero") {
		t.Errorf("Message = %q, want it to mention zero", errObj.Message)
	}
}

func TestIdivWrongTypes(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""))
	wantError(t, call(t, table, "idiv", &object.Float{Value: 1}, object.NewInteger(2)))
	wantError(t, call(t, table, "idiv", object.NewInteger(1), &object.Float{Value: 2}))
}

func TestGather(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""))
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
	table := New(&bytes.Buffer{}, strings.NewReader(""))
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
	table := New(&bytes.Buffer{}, strings.NewReader(""))
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
	table := New(&bytes.Buffer{}, strings.NewReader(""))
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
	table := New(&bytes.Buffer{}, strings.NewReader(""))
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
	table := New(&bytes.Buffer{}, strings.NewReader(""))
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
	table := New(&bytes.Buffer{}, strings.NewReader(""))
	list := object.NewList([]object.Object{object.NewList(nil)})
	errObj := wantError(t, call(t, table, "gather", list))
	if !strings.Contains(errObj.Message, "unhashable") {
		t.Errorf("Message = %q, want it to mention unhashable", errObj.Message)
	}
}

func TestGatherWrongType(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""))
	wantError(t, call(t, table, "gather", object.NewInteger(1)))
}

func TestSprinkleUnhashableItem(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""))
	set := object.NewSet()
	errObj := wantError(t, call(t, table, "sprinkle", set, object.NewList(nil)))
	if !strings.Contains(errObj.Message, "unhashable") {
		t.Errorf("Message = %q, want it to mention unhashable", errObj.Message)
	}
}

func TestPush(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""))
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
	table := New(&bytes.Buffer{}, strings.NewReader(""))
	original := object.NewList([]object.Object{object.NewInteger(1)})
	alias := original

	call(t, table, "push", original, object.NewInteger(2))

	if len(alias.Elements) != 2 {
		t.Errorf("push didn't mutate through alias: len = %d, want 2", len(alias.Elements))
	}
}

func TestPushWrongType(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""))
	wantError(t, call(t, table, "push", object.NewInteger(1), object.NewInteger(2)))
}

func TestPushWrongArgCount(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""))
	wantError(t, call(t, table, "push", object.NewList(nil)))
	wantError(t, call(t, table, "push"))
}

func TestUnboxNoArgReadsStdin(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader("puzzle input\nline two\n"))
	wantString(t, call(t, table, "unbox"), "puzzle input\nline two\n")
}

func TestUnboxWithPathReadsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(path, []byte("42\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	table := New(&bytes.Buffer{}, strings.NewReader(""))
	wantString(t, call(t, table, "unbox", &object.String{Value: path}), "42\n")
}

func TestUnboxMissingFile(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""))
	wantError(t, call(t, table, "unbox", &object.String{Value: "/no/such/file.txt"}))
}

func TestUnboxWrongArgCount(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""))
	wantError(t, call(t, table, "unbox", object.NewInteger(1), object.NewInteger(2)))
}

func TestUnboxWrongArgType(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""))
	wantError(t, call(t, table, "unbox", object.NewInteger(1)))
}

func TestLines(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""))

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
	table := New(&bytes.Buffer{}, strings.NewReader(""))
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
	table := New(&bytes.Buffer{}, strings.NewReader(""))
	list := object.NewList([]object.Object{
		&object.String{Value: "a"}, &object.String{Value: "b"}, &object.String{Value: "c"},
	})
	wantString(t, call(t, table, "join", list, &object.String{Value: ", "}), "a, b, c")
}

func TestJoinEmptyList(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""))
	wantString(t, call(t, table, "join", object.NewList(nil), &object.String{Value: ","}), "")
}

func TestJoinSingleElement(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""))
	list := object.NewList([]object.Object{&object.String{Value: "only"}})
	wantString(t, call(t, table, "join", list, &object.String{Value: ","}), "only")
}

func TestJoinRoundTripsWithSplit(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""))
	split := call(t, table, "split", &object.String{Value: "a,b,c"}, &object.String{Value: ","})
	joined := call(t, table, "join", split, &object.String{Value: ","})
	wantString(t, joined, "a,b,c")
}

func TestJoinNonStringElementIsError(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""))
	list := object.NewList([]object.Object{&object.String{Value: "a"}, object.NewInteger(1)})
	errObj := wantError(t, call(t, table, "join", list, &object.String{Value: ","}))
	if !strings.Contains(errObj.Message, "str()") {
		t.Errorf("Message = %q, want it to mention str()", errObj.Message)
	}
}

func TestJoinWrongType(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""))
	wantError(t, call(t, table, "join", object.NewInteger(1), &object.String{Value: ","}))
	wantError(t, call(t, table, "join", object.NewList(nil), object.NewInteger(1)))
}

func TestJoinWrongArgCount(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""))
	wantError(t, call(t, table, "join", object.NewList(nil)))
	wantError(t, call(t, table, "join"))
}

func TestSplitNoDelimCollapsesWhitespace(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""))
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
	table := New(&bytes.Buffer{}, strings.NewReader(""))
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
	table := New(&bytes.Buffer{}, strings.NewReader(""))
	errObj := wantError(t, call(t, table, "split", &object.String{Value: "abc"}, &object.String{Value: ""}))
	if !strings.Contains(errObj.Message, "chars") {
		t.Errorf("Message = %q, want it to point at chars(s)", errObj.Message)
	}
}

func TestSplitWrongType(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""))
	wantError(t, call(t, table, "split", object.NewInteger(1)))
	wantError(t, call(t, table, "split", &object.String{Value: "a"}, object.NewInteger(1)))
}

func TestSplitWrongArgCount(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""))
	wantError(t, call(t, table, "split"))
	wantError(t, call(t, table, "split", &object.String{Value: "a"}, &object.String{Value: "b"}, &object.String{Value: "c"}))
}

func TestTrim(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""))

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
	table := New(&bytes.Buffer{}, strings.NewReader(""))
	wantError(t, call(t, table, "trim", object.NewInteger(1)))
}

func TestStr(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""))

	wantString(t, call(t, table, "str", object.NewInteger(42)), "42")
	wantString(t, call(t, table, "str", &object.Float{Value: 3.5}), "3.5")
	wantString(t, call(t, table, "str", object.TRUE), "stuffed")
	wantString(t, call(t, table, "str", object.NULL), "nobox")
	wantString(t, call(t, table, "str", &object.String{Value: "already"}), "already")
	wantString(t, call(t, table, "str", object.NewList([]object.Object{object.NewInteger(1), object.NewInteger(2)})), "[1, 2]")
}

func TestInt(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""))

	wantInteger(t, call(t, table, "int", object.NewInteger(7)), 7)
	wantInteger(t, call(t, table, "int", &object.Float{Value: 3.9}), 3)
	wantInteger(t, call(t, table, "int", &object.Float{Value: -3.9}), -3)
	wantInteger(t, call(t, table, "int", &object.String{Value: "123"}), 123)
	wantInteger(t, call(t, table, "int", &object.String{Value: "  -5  "}), -5)
}

func TestIntParseError(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""))
	errObj := wantError(t, call(t, table, "int", &object.String{Value: "not a number"}))
	if !strings.Contains(errObj.Message, "cannot parse") {
		t.Errorf("Message = %q, want it to mention cannot parse", errObj.Message)
	}
}

func TestIntWrongType(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""))
	wantError(t, call(t, table, "int", object.TRUE))
}

func TestFloat(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""))

	wantFloat(t, call(t, table, "float", &object.Float{Value: 2.5}), 2.5)
	wantFloat(t, call(t, table, "float", object.NewInteger(4)), 4.0)
	wantFloat(t, call(t, table, "float", &object.String{Value: "3.14"}), 3.14)
}

func TestFloatParseError(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""))
	wantError(t, call(t, table, "float", &object.String{Value: "nope"}))
}

func TestFloatWrongType(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""))
	wantError(t, call(t, table, "float", object.TRUE))
}

func TestBool(t *testing.T) {
	table := New(&bytes.Buffer{}, strings.NewReader(""))

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
	table := New(&bytes.Buffer{}, strings.NewReader(""))
	for _, name := range []string{"str", "int", "float", "bool", "lines", "trim"} {
		t.Run(name, func(t *testing.T) {
			wantError(t, call(t, table, name))
			wantError(t, call(t, table, name, object.NewInteger(1), object.NewInteger(2)))
		})
	}
}
