// `crust studio`'s own builtin table -- the terminal-native counterpart
// to cmd/wasmgame/builtins.go's browser/PixiJS one. Deliberately a
// different vocabulary (cell(char, color) instead of rect/circle):
// a terminal cell IS the drawing primitive here, one character glyph
// per grid position, not a shape to be filled -- but the same
// handle-based spawn/move/destroy lifecycle and several identical verb
// names (setPos, destroy, keyDown, stageSize, onFrame, random) on
// purpose, so anyone who's used one tool already knows most of the
// other's shape. Runs as an ordinary part of the main `crust` binary
// -- unlike cmd/wasmgame, there's no browser/WASM boundary to cross
// here, so no build tag, no syscall/js, no separate binary at all.
package main

import (
	"fmt"
	"math/rand"

	"github.com/Sintfoap/cRust/internal/object"
)

// studioEntity is one on-screen thing: a single terminal character
// cell with a position and a foreground color (a lipgloss-compatible
// hex string, downsampled automatically to whatever color depth the
// real terminal actually supports).
type studioEntity struct {
	ch    rune
	color string
	x, y  int
}

// studioState is `crust studio`'s whole mutable game world, held as a
// pointer inside studioModel (studio_tui.go) rather than a value field
// -- specifically so builtin closures capturing *studioState keep
// working correctly across bubbletea's own "Update returns a
// (possibly copied) model" convention. Go maps and a pointed-to
// struct's fields both share their backing storage no matter how many
// times the containing Model value gets copied around, so mutating
// through a builtin's captured *studioState is always visible to
// whatever View() renders next, with nothing extra to wire up.
type studioState struct {
	entities   map[int64]*studioEntity
	nextID     int64
	pressed    map[string]bool // keys seen since the last onFrame call -- see keyDownFn's own doc comment
	onFrame    object.Object   // nil until onFrame(fn) is called
	cols, rows int
	lastLog    string // most recent deliver() line -- see studioLogWriter (studio_tui.go)
}

func newStudioState(cols, rows int) *studioState {
	return &studioState{
		entities: map[int64]*studioEntity{},
		pressed:  map[string]bool{},
		cols:     cols,
		rows:     rows,
	}
}

func studioNewError(format string, args ...any) *object.Error {
	return &object.Error{Message: fmt.Sprintf(format, args...)}
}

func studioWrongArgCount(name, want string, got int) *object.Error {
	return studioNewError("%s: expected %s argument(s), got %d", name, want, got)
}

func studioWrongArgType(name string, index int, want string, got object.Object) *object.Error {
	return studioNewError("%s: argument %d must be %s, got %s", name, index+1, want, got.Type())
}

func studioNumericValue(obj object.Object) (float64, bool) {
	switch v := obj.(type) {
	case *object.Integer:
		return float64(v.Value), true
	case *object.Float:
		return v.Value, true
	default:
		return 0, false
	}
}

// studioHandleArg reads args[index] as a handle (an Integer returned
// by a prior cell() call) -- every mutating builtin below takes one as
// its first argument.
func studioHandleArg(name string, index int, obj object.Object) (int64, *object.Error) {
	i, ok := obj.(*object.Integer)
	if !ok {
		return 0, studioWrongArgType(name, index, "a handle (from cell)", obj)
	}
	return i.Value, nil
}

// studioBuiltins returns the game-only builtin table, merged into a
// fresh Interpreter's own Builtins map the same way
// cmd/wasmgame/builtins.go's gameBuiltins is -- see that file's own
// doc comment for why these live outside internal/builtins entirely.
func studioBuiltins(state *studioState) map[string]*object.Builtin {
	return map[string]*object.Builtin{
		"cell":      {Fn: studioCellFn(state)},
		"setPos":    {Fn: studioSetPosFn(state)},
		"setChar":   {Fn: studioSetCharFn(state)},
		"setColor":  {Fn: studioSetColorFn(state)},
		"destroy":   {Fn: studioDestroyFn(state)},
		"keyDown":   {Fn: studioKeyDownFn(state)},
		"stageSize": {Fn: studioStageSizeFn(state)},
		"onFrame":   {Fn: studioOnFrameFn(state)},
		"random":    {Fn: studioRandomFn},
	}
}

// cell(char, color) -> handle. char must be exactly one character --
// a terminal cell can only ever show one glyph, so a longer String
// (an easy mistake, e.g. accidentally passing a whole word) is a
// runtime error here rather than silently truncated or overflowing
// into neighboring cells.
func studioCellFn(state *studioState) object.BuiltinFunction {
	return func(args ...object.Object) object.Object {
		if len(args) != 2 {
			return studioWrongArgCount("cell", "2", len(args))
		}
		s, ok := args[0].(*object.String)
		if !ok {
			return studioWrongArgType("cell", 0, "a single-character String", args[0])
		}
		runes := []rune(s.Value)
		if len(runes) != 1 {
			return studioNewError("cell: argument 1 must be exactly one character, got %q", s.Value)
		}
		color, ok := args[1].(*object.String)
		if !ok {
			return studioWrongArgType("cell", 1, `a String hex color (e.g. "#c0392b")`, args[1])
		}
		id := state.nextID
		state.nextID++
		state.entities[id] = &studioEntity{ch: runes[0], color: color.Value}
		return object.NewInteger(id)
	}
}

// setPos(handle, col, row) -- (0, 0) is the stage's top-left cell.
// Numeric (Integer or Float) x/y are both accepted and truncated to a
// cell index, the same "don't force a caller to int() a `w / 2`"
// leniency rect/circle's own w/h arguments already have; a stale or
// already-destroyed handle is a silent no-op, matching crust game's
// own "the host owns the object" posture.
func studioSetPosFn(state *studioState) object.BuiltinFunction {
	return func(args ...object.Object) object.Object {
		if len(args) != 3 {
			return studioWrongArgCount("setPos", "3", len(args))
		}
		id, errObj := studioHandleArg("setPos", 0, args[0])
		if errObj != nil {
			return errObj
		}
		x, ok := studioNumericValue(args[1])
		if !ok {
			return studioWrongArgType("setPos", 1, "a number", args[1])
		}
		y, ok := studioNumericValue(args[2])
		if !ok {
			return studioWrongArgType("setPos", 2, "a number", args[2])
		}
		if e, ok := state.entities[id]; ok {
			e.x, e.y = int(x), int(y)
		}
		return object.NULL
	}
}

// setChar(handle, char) -- change a cell's glyph in place (a torch
// flickering, a player facing a different direction) without a
// destroy+cell round trip.
func studioSetCharFn(state *studioState) object.BuiltinFunction {
	return func(args ...object.Object) object.Object {
		if len(args) != 2 {
			return studioWrongArgCount("setChar", "2", len(args))
		}
		id, errObj := studioHandleArg("setChar", 0, args[0])
		if errObj != nil {
			return errObj
		}
		s, ok := args[1].(*object.String)
		if !ok {
			return studioWrongArgType("setChar", 1, "a single-character String", args[1])
		}
		runes := []rune(s.Value)
		if len(runes) != 1 {
			return studioNewError("setChar: argument 2 must be exactly one character, got %q", s.Value)
		}
		if e, ok := state.entities[id]; ok {
			e.ch = runes[0]
		}
		return object.NULL
	}
}

// setColor(handle, color) -- change a cell's foreground color in place.
func studioSetColorFn(state *studioState) object.BuiltinFunction {
	return func(args ...object.Object) object.Object {
		if len(args) != 2 {
			return studioWrongArgCount("setColor", "2", len(args))
		}
		id, errObj := studioHandleArg("setColor", 0, args[0])
		if errObj != nil {
			return errObj
		}
		color, ok := args[1].(*object.String)
		if !ok {
			return studioWrongArgType("setColor", 1, `a String hex color (e.g. "#c0392b")`, args[1])
		}
		if e, ok := state.entities[id]; ok {
			e.color = color.Value
		}
		return object.NULL
	}
}

// destroy(handle) -- remove a cell from the stage.
func studioDestroyFn(state *studioState) object.BuiltinFunction {
	return func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return studioWrongArgCount("destroy", "1", len(args))
		}
		id, errObj := studioHandleArg("destroy", 0, args[0])
		if errObj != nil {
			return errObj
		}
		delete(state.entities, id)
		return object.NULL
	}
}

// keyDown(name) -> bool. Unlike crust game's browser-based keyDown,
// there is no key-up event a terminal can ever report (raw-mode
// keypresses have no release signal at all), so this can't be
// genuinely level-triggered the way the browser version is. Instead:
// true if `name` was pressed at any point since the *previous*
// onFrame call (studio_tui.go's tick handler clears state.pressed
// right after each call), false otherwise. Holding a key down still
// reads as roughly "held" in practice, because the terminal's own
// OS-level key-repeat keeps re-sending press events for as long as
// it's down -- but expect the same short initial pause real terminal
// key-repeat always has before it kicks in, and a single tap will
// read true for exactly one frame, not fade out over some arbitrary
// timeout. `name` is bubbletea's own tea.KeyMsg.String() output
// ("up", "down", "left", "right", "a".."z", " ", ...), not the
// browser's KeyboardEvent.key spelling -- the two tools' keyDown
// share a name and a "true while active" shape, not a wire format,
// since neither runs the other's source anyway (different builtin
// vocabularies entirely).
func studioKeyDownFn(state *studioState) object.BuiltinFunction {
	return func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return studioWrongArgCount("keyDown", "1", len(args))
		}
		name, ok := args[0].(*object.String)
		if !ok {
			return studioWrongArgType("keyDown", 0, "a String", args[0])
		}
		if state.pressed[name.Value] {
			return object.TRUE
		}
		return object.FALSE
	}
}

// stageSize() -> (cols, rows), the usable grid size in character
// cells (the full terminal width, minus the header/help lines
// studio_tui.go's View reserves for its own chrome).
func studioStageSizeFn(state *studioState) object.BuiltinFunction {
	return func(args ...object.Object) object.Object {
		if len(args) != 0 {
			return studioWrongArgCount("stageSize", "0", len(args))
		}
		return object.NewTuple([]object.Object{
			object.NewInteger(int64(state.cols)),
			object.NewInteger(int64(state.rows)),
		})
	}
}

// onFrame(fn) registers fn as the per-tick callback, called with one
// argument (dt, elapsed seconds since the previous tick).
func studioOnFrameFn(state *studioState) object.BuiltinFunction {
	return func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return studioWrongArgCount("onFrame", "1", len(args))
		}
		state.onFrame = args[0]
		return object.NULL
	}
}

// random() -> a Float in [0, 1) -- identical to crust game's own, kept
// here rather than shared since the two builtin tables otherwise have
// nothing to import from each other (cmd/wasmgame can't be imported
// from a normal host build at all -- it's js/wasm-build-tagged).
func studioRandomFn(args ...object.Object) object.Object {
	if len(args) != 0 {
		return studioWrongArgCount("random", "0", len(args))
	}
	return &object.Float{Value: rand.Float64()}
}
