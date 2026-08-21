//go:build js && wasm

package main

import (
	"fmt"
	"math/rand"
	"syscall/js"

	"github.com/Sintfoap/cRust/internal/object"
)

// nextHandleID, pressedKeys, and onFrameFn are the whole in-page game's
// mutable state -- package-level vars rather than a struct, since
// there's only ever one game running in a given WASM instance (one
// browser tab, one page), the same "only one of these exists" reasoning
// cmd/wasm/main.go's own single crustRun export relies on. crustGameInit
// (main.go) calls resetGameState before every (re)run, so pressing
// "Run" again after an edit starts clean rather than leaking the
// previous run's sprites/handlers into the new one.
var (
	nextHandleID int64
	pressedKeys  = map[string]bool{}
	onFrameFn    object.Object // nil until onFrame(fn) is called
)

// resetGameState clears everything the previous run (if any) left
// behind -- called once per crustGameInit, so re-running an edited
// program never carries over stale key state or a stale onFrame
// handler from the run before it.
func resetGameState() {
	nextHandleID = 0
	pressedKeys = map[string]bool{}
	onFrameFn = nil
}

// host is the JS-side object crust-game.html installs before loading
// crust-game.wasm (see that file's __crustGameHost). Every rendering
// builtin below is a thin relay onto one of its methods -- cRust
// allocates handle IDs and validates arguments; the host owns the
// actual PixiJS objects and the id -> object.Objectbox lookup, so nGo
// never has to hold (or leak) a js.Value across calls.
func host() js.Value {
	return js.Global().Get("__crustGameHost")
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

// handleArg reads args[index] as a handle (an Integer returned by a
// prior rect/circle call) -- every mutating builtin (setPos, destroy,
// ...) takes one as its first argument.
func handleArg(name string, index int, obj object.Object) (int64, *object.Error) {
	i, ok := obj.(*object.Integer)
	if !ok {
		return 0, wrongArgType(name, index, "a handle (from rect/circle)", obj)
	}
	return i.Value, nil
}

// gameBuiltins returns the game-only builtin table, merged into a
// fresh Interpreter's own Builtins map (interpreter.New's ordinary
// table doesn't know about any of this -- see cmd/wasmgame/main.go's
// crustGameInit) rather than living in internal/builtins itself: this
// whole file only exists under the js/wasm build, and internal/builtins
// is imported by the plain CLI interpreter too, which has no
// __crustGameHost to call out to.
func gameBuiltins() map[string]*object.Builtin {
	return map[string]*object.Builtin{
		"rect":        {Fn: rectFn},
		"circle":      {Fn: circleFn},
		"setPos":      {Fn: setPosFn},
		"setRotation": {Fn: setRotationFn},
		"setScale":    {Fn: setScaleFn},
		"destroy":     {Fn: destroyFn},
		"keyDown":     {Fn: keyDownFn},
		"stageSize":   {Fn: stageSizeFn},
		"onFrame":     {Fn: onFrameFn_},
		"random":      {Fn: randomFn},
	}
}

// rect(w, h, color) -> handle. color is a CSS-style hex string
// ("#ff6a3d") -- a String rather than a bare Integer 0xff6a3d, since
// cRust has no hex integer literal syntax (SPEC.md §2) and typing the
// decimal equivalent by hand is exactly the kind of friction a color
// argument shouldn't have.
func rectFn(args ...object.Object) object.Object {
	if len(args) != 3 {
		return wrongArgCount("rect", "3", len(args))
	}
	w, ok := numericValue(args[0])
	if !ok {
		return wrongArgType("rect", 0, "a number", args[0])
	}
	h, ok := numericValue(args[1])
	if !ok {
		return wrongArgType("rect", 1, "a number", args[1])
	}
	color, ok := args[2].(*object.String)
	if !ok {
		return wrongArgType("rect", 2, `a String hex color (e.g. "#ff6a3d")`, args[2])
	}
	id := nextHandleID
	nextHandleID++
	host().Call("rectCreate", id, w, h, color.Value)
	return object.NewInteger(id)
}

// circle(radius, color) -> handle.
func circleFn(args ...object.Object) object.Object {
	if len(args) != 2 {
		return wrongArgCount("circle", "2", len(args))
	}
	r, ok := numericValue(args[0])
	if !ok {
		return wrongArgType("circle", 0, "a number", args[0])
	}
	color, ok := args[1].(*object.String)
	if !ok {
		return wrongArgType("circle", 1, `a String hex color (e.g. "#ff6a3d")`, args[1])
	}
	id := nextHandleID
	nextHandleID++
	host().Call("circleCreate", id, r, color.Value)
	return object.NewInteger(id)
}

// setPos(handle, x, y) -- (0, 0) is the stage's top-left corner, same
// as every browser canvas API, rather than cRust's own Grid type's
// center-origin convention (SPEC.md's grid()/at()/setAt()): a game
// stage is screen space, not a puzzle grid, and matching the DOM/canvas
// convention here is one less thing to translate when reading PixiJS's
// own docs alongside this.
func setPosFn(args ...object.Object) object.Object {
	if len(args) != 3 {
		return wrongArgCount("setPos", "3", len(args))
	}
	id, errObj := handleArg("setPos", 0, args[0])
	if errObj != nil {
		return errObj
	}
	x, ok := numericValue(args[1])
	if !ok {
		return wrongArgType("setPos", 1, "a number", args[1])
	}
	y, ok := numericValue(args[2])
	if !ok {
		return wrongArgType("setPos", 2, "a number", args[2])
	}
	host().Call("setPos", id, x, y)
	return object.NULL
}

// setRotation(handle, radians).
func setRotationFn(args ...object.Object) object.Object {
	if len(args) != 2 {
		return wrongArgCount("setRotation", "2", len(args))
	}
	id, errObj := handleArg("setRotation", 0, args[0])
	if errObj != nil {
		return errObj
	}
	a, ok := numericValue(args[1])
	if !ok {
		return wrongArgType("setRotation", 1, "a number", args[1])
	}
	host().Call("setRotation", id, a)
	return object.NULL
}

// setScale(handle, sx, sy).
func setScaleFn(args ...object.Object) object.Object {
	if len(args) != 3 {
		return wrongArgCount("setScale", "3", len(args))
	}
	id, errObj := handleArg("setScale", 0, args[0])
	if errObj != nil {
		return errObj
	}
	sx, ok := numericValue(args[1])
	if !ok {
		return wrongArgType("setScale", 1, "a number", args[1])
	}
	sy, ok := numericValue(args[2])
	if !ok {
		return wrongArgType("setScale", 2, "a number", args[2])
	}
	host().Call("setScale", id, sx, sy)
	return object.NULL
}

// destroy(handle) -- removes the sprite from the stage. Calling any
// other builtin on a destroyed handle is a host-side no-op (the JS
// side just won't find it in its id table), not a cRust runtime error;
// keeping that check out of the interpreter loop is worth the small
// risk of a silently-ignored stale handle, the same trade every other
// "the host owns the real object" design here makes.
func destroyFn(args ...object.Object) object.Object {
	if len(args) != 1 {
		return wrongArgCount("destroy", "1", len(args))
	}
	id, errObj := handleArg("destroy", 0, args[0])
	if errObj != nil {
		return errObj
	}
	host().Call("destroy", id)
	return object.NULL
}

// keyDown(name) -> bool. Polled rather than event-based -- the natural
// fit for a per-frame game loop (SPEC.md-style "ask what's true right
// now" rather than "subscribe to a stream of events"), and it's the
// browser's own KeyboardEvent.key strings ("ArrowLeft", "a", " ",
// ...) coming straight through from crustGameKey (main.go), not a
// cRust-invented naming scheme to document separately.
func keyDownFn(args ...object.Object) object.Object {
	if len(args) != 1 {
		return wrongArgCount("keyDown", "1", len(args))
	}
	name, ok := args[0].(*object.String)
	if !ok {
		return wrongArgType("keyDown", 0, "a String", args[0])
	}
	if pressedKeys[name.Value] {
		return object.TRUE
	}
	return object.FALSE
}

// stageSize() -> (width, height), in pixels.
func stageSizeFn(args ...object.Object) object.Object {
	if len(args) != 0 {
		return wrongArgCount("stageSize", "0", len(args))
	}
	v := host().Call("stageSize")
	return object.NewTuple([]object.Object{
		&object.Float{Value: v.Index(0).Float()},
		&object.Float{Value: v.Index(1).Float()},
	})
}

// onFrame(fn) registers fn as the per-frame callback, called with one
// argument (dt, elapsed seconds since the previous frame) every time
// the browser's requestAnimationFrame fires (crustGameFrame, main.go).
// Not type-checked here -- if fn isn't actually callable, that surfaces
// as an ordinary cRust runtime error the moment the first frame tries
// to call it (interp.Call's own job), the same "let the real call site
// report it" posture map/filter/reduce's own call parameter already
// takes.
func onFrameFn_(args ...object.Object) object.Object {
	if len(args) != 1 {
		return wrongArgCount("onFrame", "1", len(args))
	}
	onFrameFn = args[0]
	return object.NULL
}

// random() -> a Float in [0, 1) -- games need enemy spawn points, wait
// times, drop chances, exactly the kind of thing that has no business
// being deterministic, unlike the rest of cRust's stdlib (which never
// needed randomness at all: AoC puzzles have one correct answer, and
// every builtin up to this one is a pure function of its arguments).
// Go's global math/rand source has been auto-seeded from a real
// entropy source since Go 1.20, so no explicit seeding here.
func randomFn(args ...object.Object) object.Object {
	if len(args) != 0 {
		return wrongArgCount("random", "0", len(args))
	}
	return &object.Float{Value: rand.Float64()}
}
