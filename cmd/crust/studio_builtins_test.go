package main

import (
	"testing"

	"github.com/Sintfoap/cRust/internal/object"
)

func TestStudioStageDimsReservesHeaderAndHelpLines(t *testing.T) {
	cols, rows := studioStageDims(60, 24)
	if cols != 60 || rows != 22 {
		t.Errorf("studioStageDims(60, 24) = (%d, %d), want (60, 22)", cols, rows)
	}
}

func TestStudioStageDimsClampsDegenerateSize(t *testing.T) {
	tests := []struct {
		name         string
		w, h         int
		wantCols     int
		wantRowsAtLo int
	}{
		{"zero", 0, 0, studioMinCols, studioMinRows},
		{"tiny", 1, 2, studioMinCols, studioMinRows},
		{"negative", -5, -5, studioMinCols, studioMinRows},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cols, rows := studioStageDims(tt.w, tt.h)
			if cols < studioMinCols || rows < studioMinRows {
				t.Errorf("studioStageDims(%d, %d) = (%d, %d), want both clamped to at least (%d, %d)",
					tt.w, tt.h, cols, rows, studioMinCols, studioMinRows)
			}
		})
	}
}

func newTestStudioState() *studioState {
	return newStudioState(20, 10)
}

func TestStudioCellFnSpawnsAndReturnsHandle(t *testing.T) {
	state := newTestStudioState()
	fn := studioCellFn(state)
	result := fn(&object.String{Value: "@"}, &object.String{Value: "#c0392b"})
	handle, ok := result.(*object.Integer)
	if !ok {
		t.Fatalf("cell() returned %T (%s), want *object.Integer", result, result.Inspect())
	}
	e, ok := state.entities[handle.Value]
	if !ok {
		t.Fatalf("entity %d not found in state.entities after cell()", handle.Value)
	}
	if e.ch != '@' || e.color != "#c0392b" {
		t.Errorf("entity = {ch: %q, color: %q}, want {ch: '@', color: \"#c0392b\"}", e.ch, e.color)
	}
}

func TestStudioCellFnRejectsMultiCharString(t *testing.T) {
	state := newTestStudioState()
	fn := studioCellFn(state)
	result := fn(&object.String{Value: "ab"}, &object.String{Value: "#c0392b"})
	if _, ok := result.(*object.Error); !ok {
		t.Errorf("cell(\"ab\", ...) = %T, want *object.Error for a multi-character glyph", result)
	}
}

func TestStudioCellFnWrongArgCount(t *testing.T) {
	state := newTestStudioState()
	fn := studioCellFn(state)
	result := fn(&object.String{Value: "@"})
	if _, ok := result.(*object.Error); !ok {
		t.Errorf("cell() with 1 arg = %T, want *object.Error", result)
	}
}

func TestStudioSetPosMovesEntity(t *testing.T) {
	state := newTestStudioState()
	id := state.nextID
	state.entities[id] = &studioEntity{ch: '@', color: "#fff"}
	state.nextID++

	fn := studioSetPosFn(state)
	result := fn(object.NewInteger(id), object.NewInteger(3), object.NewInteger(4))
	if _, ok := result.(*object.Error); ok {
		t.Fatalf("setPos: %s", result.Inspect())
	}
	if state.entities[id].x != 3 || state.entities[id].y != 4 {
		t.Errorf("entity pos = (%d, %d), want (3, 4)", state.entities[id].x, state.entities[id].y)
	}
}

func TestStudioSetPosTruncatesFloatArgs(t *testing.T) {
	state := newTestStudioState()
	id := state.nextID
	state.entities[id] = &studioEntity{ch: '@', color: "#fff"}
	state.nextID++

	fn := studioSetPosFn(state)
	fn(object.NewInteger(id), &object.Float{Value: 3.9}, &object.Float{Value: 4.1})
	if state.entities[id].x != 3 || state.entities[id].y != 4 {
		t.Errorf("entity pos = (%d, %d), want (3, 4) (truncated, not rounded)", state.entities[id].x, state.entities[id].y)
	}
}

func TestStudioSetPosOnStaleHandleIsSilentNoOp(t *testing.T) {
	state := newTestStudioState()
	fn := studioSetPosFn(state)
	result := fn(object.NewInteger(999), object.NewInteger(1), object.NewInteger(1))
	if _, ok := result.(*object.Error); ok {
		t.Errorf("setPos on a stale handle = %s, want a silent no-op (NULL), not an error", result.Inspect())
	}
}

func TestStudioSetCharAndSetColorMutateInPlace(t *testing.T) {
	state := newTestStudioState()
	id := state.nextID
	state.entities[id] = &studioEntity{ch: '@', color: "#fff", x: 1, y: 1}
	state.nextID++

	studioSetCharFn(state)(object.NewInteger(id), &object.String{Value: "#"})
	studioSetColorFn(state)(object.NewInteger(id), &object.String{Value: "#000"})

	e := state.entities[id]
	if e.ch != '#' || e.color != "#000" {
		t.Errorf("entity = {ch: %q, color: %q}, want {ch: '#', color: \"#000\"}", e.ch, e.color)
	}
}

func TestStudioDestroyRemovesEntity(t *testing.T) {
	state := newTestStudioState()
	id := state.nextID
	state.entities[id] = &studioEntity{ch: '@', color: "#fff"}
	state.nextID++

	studioDestroyFn(state)(object.NewInteger(id))
	if _, ok := state.entities[id]; ok {
		t.Error("entity still present after destroy()")
	}
}

func TestStudioKeyDownReflectsPressedSet(t *testing.T) {
	state := newTestStudioState()
	fn := studioKeyDownFn(state)

	if got := fn(&object.String{Value: "up"}); got != object.FALSE {
		t.Errorf("keyDown(\"up\") before any press = %s, want thin", got.Inspect())
	}
	state.pressed["up"] = true
	if got := fn(&object.String{Value: "up"}); got != object.TRUE {
		t.Errorf("keyDown(\"up\") after press = %s, want stuffed", got.Inspect())
	}
}

func TestStudioStageSizeReturnsCurrentDims(t *testing.T) {
	state := newStudioState(15, 7)
	result := studioStageSizeFn(state)()
	tuple, ok := result.(*object.Tuple)
	if !ok || len(tuple.Elements) != 2 {
		t.Fatalf("stageSize() = %v, want a 2-element Tuple", result)
	}
	w, ok1 := tuple.Elements[0].(*object.Integer)
	h, ok2 := tuple.Elements[1].(*object.Integer)
	if !ok1 || !ok2 || w.Value != 15 || h.Value != 7 {
		t.Errorf("stageSize() = %s, want (15, 7)", result.Inspect())
	}
}

func TestStudioOnFrameRegistersCallback(t *testing.T) {
	state := newTestStudioState()
	fn := object.Builtin{}
	studioOnFrameFn(state)(&fn)
	if state.onFrame != &fn {
		t.Error("onFrame(fn) didn't register fn as state.onFrame")
	}
}

func TestStudioRandomReturnsFloatInRange(t *testing.T) {
	for i := 0; i < 50; i++ {
		result := studioRandomFn()
		f, ok := result.(*object.Float)
		if !ok {
			t.Fatalf("random() = %T, want *object.Float", result)
		}
		if f.Value < 0 || f.Value >= 1 {
			t.Fatalf("random() = %v, want a value in [0, 1)", f.Value)
		}
	}
}

func TestStudioBuiltinsIncludesEveryVerb(t *testing.T) {
	state := newTestStudioState()
	table := studioBuiltins(state)
	for _, name := range []string{"cell", "setPos", "setChar", "setColor", "destroy", "keyDown", "stageSize", "onFrame", "random"} {
		if _, ok := table[name]; !ok {
			t.Errorf("studioBuiltins() is missing %q", name)
		}
	}
}
