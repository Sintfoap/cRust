//go:build js && wasm

package main

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"syscall/js"

	"github.com/Sintfoap/cRust/internal/object"
)

// gameEntity is what Go itself remembers about a handle, alongside
// whatever PixiJS object the host keeps under the same ID (see host()
// below). Earlier versions of this file kept no state at all here --
// Go only ever allocated an ID and relayed calls -- but overlaps()
// needs real position/size to compute a collision without a round
// trip to JS for every check, every frame, so position and an
// (optional) hitbox are tracked here now. kind is "" for a handle with
// no meaningful collision shape (text, sound) -- overlaps() refuses to
// run against one of those rather than silently comparing against a
// zero-size point.
type gameEntity struct {
	kind    string // "rect", "circle", "sprite", or "" (no hitbox)
	x, y    float64
	w, h, r float64
}

// animState is playAnimation()'s own per-handle timer -- how far
// through the current frame's dwell time a looping sprite-sheet
// animation is, and which frame (column) it's currently showing.
// Ticked once per crustGameFrame by tickAnimations, entirely
// independent of whatever the game script's own onFrame(fn) does.
type animState struct {
	row, frameCount int
	frameW, frameH  float64
	fps             float64
	elapsed         float64
	frame           int
}

// nextHandleID, pressedKeys, onFrameFn, entities, and animations are
// the whole in-page game's mutable state -- package-level vars rather
// than a struct, since there's only ever one game running in a given
// WASM instance (one browser tab, one page), the same "only one of
// these exists" reasoning cmd/wasm/main.go's own single crustRun
// export relies on. crustGameInit (main.go) calls resetGameState
// before every (re)run, so pressing "Run" again after an edit starts
// clean rather than leaking the previous run's sprites/handlers into
// the new one.
var (
	nextHandleID int64
	pressedKeys  = map[string]bool{}
	onFrameFn    object.Object // nil until onFrame(fn) is called
	entities     = map[int64]*gameEntity{}
	animations   = map[int64]*animState{}
)

// resetGameState clears everything the previous run (if any) left
// behind -- called once per crustGameInit, so re-running an edited
// program never carries over stale key state or a stale onFrame
// handler from the run before it.
func resetGameState() {
	nextHandleID = 0
	pressedKeys = map[string]bool{}
	onFrameFn = nil
	entities = map[int64]*gameEntity{}
	animations = map[int64]*animState{}
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
		"rect":          {Fn: rectFn},
		"circle":        {Fn: circleFn},
		"sprite":        {Fn: spriteFn},
		"text":          {Fn: textFn},
		"setText":       {Fn: setTextFn},
		"setPos":        {Fn: setPosFn},
		"setRotation":   {Fn: setRotationFn},
		"setScale":      {Fn: setScaleFn},
		"destroy":       {Fn: destroyFn},
		"keyDown":       {Fn: keyDownFn},
		"stageSize":     {Fn: stageSizeFn},
		"onFrame":       {Fn: onFrameFn_},
		"random":        {Fn: randomFn},
		"overlaps":      {Fn: overlapsFn},
		"setCamera":     {Fn: setCameraFn},
		"setCameraZoom": {Fn: setCameraZoomFn},
		"loadTilemap":   {Fn: loadTilemapFn},
		"sound":         {Fn: soundFn},
		"playSound":     {Fn: playSoundFn},
		"stopSound":     {Fn: stopSoundFn},
		"mouseX":        {Fn: mouseXFn},
		"mouseY":        {Fn: mouseYFn},
		"mouseDown":     {Fn: mouseDownFn},
		"mouseClicked":  {Fn: mouseClickedFn},
		"setFrame":      {Fn: setFrameFn},
		"setLayer":      {Fn: setLayerFn},
		"clearScene":    {Fn: clearSceneFn},
		"tileAt":        {Fn: tileAtFn},
		"emitParticles": {Fn: emitParticlesFn},
		"playAnimation": {Fn: playAnimationFn},
		"stopAnimation": {Fn: stopAnimationFn},
		"save":          {Fn: saveFn},
		"load":          {Fn: loadFn},
	}
}

// spawnRect/spawnCircle/spawnSprite do the actual work behind
// rect()/circle()/sprite() and loadTilemap() (which needs to spawn
// many rects without going through cRust-Object argument boxing for
// each one) -- allocate an ID, remember its kind/size for overlaps()
// to use later, and hand it to the host to actually draw.
func spawnRect(w, h float64, color string) int64 {
	id := nextHandleID
	nextHandleID++
	entities[id] = &gameEntity{kind: "rect", w: w, h: h}
	host().Call("rectCreate", id, w, h, color)
	return id
}

func spawnCircle(r float64, color string) int64 {
	id := nextHandleID
	nextHandleID++
	entities[id] = &gameEntity{kind: "circle", r: r}
	host().Call("circleCreate", id, r, color)
	return id
}

// moveEntity is setPos()'s own logic, factored out so loadTilemap can
// position each spawned tile the same way without re-boxing plain
// float64s into object.Objects just to immediately unwrap them again.
func moveEntity(id int64, x, y float64) {
	if e, ok := entities[id]; ok {
		e.x, e.y = x, y
	}
	host().Call("setPos", id, x, y)
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
	return object.NewInteger(spawnRect(w, h, color.Value))
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
	return object.NewInteger(spawnCircle(r, color.Value))
}

// sprite(url, w, h) -> handle. url is fetched relative to the served
// page -- either one of the game's own assets (nothing ships built
// in), or a file sitting next to the .crust file `crust game` was
// pointed at (game.go serves that directory as a fallback specifically
// so this resolves). Loading is unavoidably asynchronous in a browser,
// but every other builtin here is synchronous, so the handle exists
// (and setPos/setRotation/etc all work on it) immediately, drawing as
// a blank placeholder that swaps to the real image in place once it
// finishes loading -- there's no separate "wait for it" step or
// callback to write.
func spriteFn(args ...object.Object) object.Object {
	if len(args) != 3 {
		return wrongArgCount("sprite", "3", len(args))
	}
	url, ok := args[0].(*object.String)
	if !ok {
		return wrongArgType("sprite", 0, "a String (image URL)", args[0])
	}
	w, ok := numericValue(args[1])
	if !ok {
		return wrongArgType("sprite", 1, "a number", args[1])
	}
	h, ok := numericValue(args[2])
	if !ok {
		return wrongArgType("sprite", 2, "a number", args[2])
	}
	id := nextHandleID
	nextHandleID++
	entities[id] = &gameEntity{kind: "sprite", w: w, h: h}
	host().Call("spriteCreate", id, url.Value, w, h)
	return object.NewInteger(id)
}

// text(str, size, color) -> handle. size is a font size in pixels.
// Has no collision hitbox (overlaps() refuses a text handle) -- text
// bounds depend on rendered glyph metrics the Go side never sees, so
// there's nothing honest to check it against.
func textFn(args ...object.Object) object.Object {
	if len(args) != 3 {
		return wrongArgCount("text", "3", len(args))
	}
	str, ok := args[0].(*object.String)
	if !ok {
		return wrongArgType("text", 0, "a String", args[0])
	}
	size, ok := numericValue(args[1])
	if !ok {
		return wrongArgType("text", 1, "a number", args[1])
	}
	color, ok := args[2].(*object.String)
	if !ok {
		return wrongArgType("text", 2, `a String hex color (e.g. "#3b2a1a")`, args[2])
	}
	id := nextHandleID
	nextHandleID++
	entities[id] = &gameEntity{kind: ""}
	host().Call("textCreate", id, str.Value, size, color.Value)
	return object.NewInteger(id)
}

// setText(handle, str) -- change a text handle's displayed string in
// place (a score counter, a HUD line), the same "mutate, don't
// destroy+recreate" shape setChar/setColor already give crust studio's
// own cells.
func setTextFn(args ...object.Object) object.Object {
	if len(args) != 2 {
		return wrongArgCount("setText", "2", len(args))
	}
	id, errObj := handleArg("setText", 0, args[0])
	if errObj != nil {
		return errObj
	}
	str, ok := args[1].(*object.String)
	if !ok {
		return wrongArgType("setText", 1, "a String", args[1])
	}
	host().Call("setText", id, str.Value)
	return object.NULL
}

// setPos(handle, x, y) -- (0, 0) is the stage's top-left corner, same
// as every browser canvas API, rather than cRust's own Grid type's
// center-origin convention (SPEC.md's grid()/at()/setAt()): a game
// stage is screen space, not a puzzle grid, and matching the DOM/canvas
// convention here is one less thing to translate when reading PixiJS's
// own docs alongside this. Unaffected by the camera (setCamera below)
// -- this always sets a handle's position in world space, exactly as
// it always has; the camera only changes what part of that world space
// is currently visible.
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
	moveEntity(id, x, y)
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
// "the host owns the real object" design here makes. Go's own
// entities map entry is removed too, so a destroyed handle correctly
// stops being collidable rather than overlaps() still comparing
// against wherever it was last positioned.
func destroyFn(args ...object.Object) object.Object {
	if len(args) != 1 {
		return wrongArgCount("destroy", "1", len(args))
	}
	id, errObj := handleArg("destroy", 0, args[0])
	if errObj != nil {
		return errObj
	}
	delete(entities, id)
	delete(animations, id)
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

// closestPointOnRect clamps (px, py) to the axis-aligned rectangle
// centered at (cx, cy) with half-extents (hw, hh) -- the standard
// building block for circle-vs-rectangle collision: the circle
// overlaps the rectangle exactly when its center is within its own
// radius of the clamped (closest) point.
func closestPointOnRect(px, py, cx, cy, hw, hh float64) (float64, float64) {
	x := px
	if x < cx-hw {
		x = cx - hw
	} else if x > cx+hw {
		x = cx + hw
	}
	y := py
	if y < cy-hh {
		y = cy - hh
	} else if y > cy+hh {
		y = cy + hh
	}
	return x, y
}

// overlaps(handle1, handle2) -> bool. Exact for circle-vs-circle
// (distance between centers vs the sum of radii) and rect-vs-rect
// (axis-aligned bounding box overlap -- rotation from setRotation is
// ignored for this check, the standard simplification every "AABB
// collision" helper in a lightweight 2D engine makes), and a real
// circle-vs-rectangle test (via closestPointOnRect) for a mix of the
// two, including a sprite's own w/h rectangle. Refuses rather than
// silently comparing against a zero-size point when either handle has
// no hitbox at all (a text or sound handle, or one destroy()'d/never
// created) -- see gameEntity's own doc comment.
func overlapsFn(args ...object.Object) object.Object {
	if len(args) != 2 {
		return wrongArgCount("overlaps", "2", len(args))
	}
	id1, errObj := handleArg("overlaps", 0, args[0])
	if errObj != nil {
		return errObj
	}
	id2, errObj := handleArg("overlaps", 1, args[1])
	if errObj != nil {
		return errObj
	}
	e1, ok := entities[id1]
	if !ok || e1.kind == "" {
		return newError("overlaps: argument 1 (handle %d) has no collision shape (from text() or an unknown/destroyed handle)", id1)
	}
	e2, ok := entities[id2]
	if !ok || e2.kind == "" {
		return newError("overlaps: argument 2 (handle %d) has no collision shape (from text() or an unknown/destroyed handle)", id2)
	}

	if e1.kind == "circle" && e2.kind == "circle" {
		dx, dy := e1.x-e2.x, e1.y-e2.y
		dist2 := dx*dx + dy*dy
		rSum := e1.r + e2.r
		return object.NativeBoolToBooleanObject(dist2 <= rSum*rSum)
	}
	if e1.kind != "circle" && e2.kind != "circle" {
		// both rect-shaped (rect() or sprite()): AABB overlap.
		return object.NativeBoolToBooleanObject(
			e1.x-e1.w/2 < e2.x+e2.w/2 && e1.x+e1.w/2 > e2.x-e2.w/2 &&
				e1.y-e1.h/2 < e2.y+e2.h/2 && e1.y+e1.h/2 > e2.y-e2.h/2,
		)
	}
	// one circle, one rectangle -- put the circle in a known slot.
	circle, rect := e1, e2
	if e2.kind == "circle" {
		circle, rect = e2, e1
	}
	cx, cy := closestPointOnRect(circle.x, circle.y, rect.x, rect.y, rect.w/2, rect.h/2)
	dx, dy := circle.x-cx, circle.y-cy
	return object.NativeBoolToBooleanObject(dx*dx+dy*dy <= circle.r*circle.r)
}

// setCamera(x, y) pans the world so (x, y) in world space is centered
// in the viewport -- everything spawned by rect/circle/sprite/text
// lives in one PixiJS Container the host moves as a whole (see
// crust-game.html's own `world`), so this is the one call a
// side-scroller/top-down level needs for "the camera follows the
// player." setPos itself is never affected -- it always sets world-
// space position, camera or no camera (see setPosFn's own doc
// comment).
func setCameraFn(args ...object.Object) object.Object {
	if len(args) != 2 {
		return wrongArgCount("setCamera", "2", len(args))
	}
	x, ok := numericValue(args[0])
	if !ok {
		return wrongArgType("setCamera", 0, "a number", args[0])
	}
	y, ok := numericValue(args[1])
	if !ok {
		return wrongArgType("setCamera", 1, "a number", args[1])
	}
	host().Call("setCamera", x, y)
	return object.NULL
}

// setCameraZoom(zoom) scales the whole world -- 2 is twice as close,
// 0.5 is zoomed out to half size. A separate builtin from setCamera
// rather than a third argument to it: panning and zooming are
// independently useful (many games only ever call one of the two), and
// keeping the panning call's own arity stable avoids a silent behavior
// change for existing setCamera(x, y) call sites if zoom is ever
// extended further (a transition duration, an easing curve).
func setCameraZoomFn(args ...object.Object) object.Object {
	if len(args) != 1 {
		return wrongArgCount("setCameraZoom", "1", len(args))
	}
	zoom, ok := numericValue(args[0])
	if !ok {
		return wrongArgType("setCameraZoom", 0, "a number", args[0])
	}
	host().Call("setCameraZoom", zoom)
	return object.NULL
}

// loadTilemap(layout, tileSize, palette) -> List of handles. layout is
// a multi-line String, one character per tile (the same shape SPEC.md's
// own grid(s) builtin already parses for puzzle grids -- deliberately
// reused here rather than inventing a second "ascii map" convention);
// palette is a Map from single-character String to a hex color String,
// e.g. {"#": "#7a6a55", ".": "#fdf6e3"}. A character with no entry in
// palette is treated as empty space -- no tile spawned there at all --
// the standard "unmarked means walkable floor" convention every ascii
// roguelike map already uses. Each tile is an ordinary rect() handle
// (nothing new on the host side at all: loadTilemap is pure Go sugar
// over spawnRect, called directly rather than through rectFn's own
// object.Object argument boxing since there's nothing to unbox here),
// so setColor/destroy/overlaps all work on an individual tile exactly
// like any other rect -- there's no separate "tile" concept to learn.
func loadTilemapFn(args ...object.Object) object.Object {
	if len(args) != 3 {
		return wrongArgCount("loadTilemap", "3", len(args))
	}
	layout, ok := args[0].(*object.String)
	if !ok {
		return wrongArgType("loadTilemap", 0, "a String (rows separated by newlines)", args[0])
	}
	tileSize, ok := numericValue(args[1])
	if !ok {
		return wrongArgType("loadTilemap", 1, "a number", args[1])
	}
	palette, ok := args[2].(*object.Map)
	if !ok {
		return wrongArgType("loadTilemap", 2, "a Map of single-character String -> hex color String", args[2])
	}

	var handles []object.Object
	for row, line := range strings.Split(layout.Value, "\n") {
		col := 0
		for _, ch := range line {
			colorObj, ok := palette.Get(&object.String{Value: string(ch)})
			if ok {
				if color, ok := colorObj.(*object.String); ok {
					id := spawnRect(tileSize, tileSize, color.Value)
					moveEntity(id, float64(col)*tileSize+tileSize/2, float64(row)*tileSize+tileSize/2)
					handles = append(handles, object.NewInteger(id))
				}
			}
			col++
		}
	}
	return object.NewList(handles)
}

// sound(url) -> handle -- loads an audio file, the same "fetched
// relative to the served page" resolution sprite()'s own url has.
// Doesn't share the entities map with visual handles (kind "") since
// setPos/setRotation/overlaps make no sense against a sound and
// there's no reason to make them silently no-op instead of just never
// accepting a sound handle in the first place -- playSound/stopSound
// are the only builtins that take one.
func soundFn(args ...object.Object) object.Object {
	if len(args) != 1 {
		return wrongArgCount("sound", "1", len(args))
	}
	url, ok := args[0].(*object.String)
	if !ok {
		return wrongArgType("sound", 0, "a String (audio URL)", args[0])
	}
	id := nextHandleID
	nextHandleID++
	host().Call("soundCreate", id, url.Value)
	return object.NewInteger(id)
}

// playSound(handle) -- starts playback from the beginning. Overlapping
// calls (a pickup sound firing again before the last one finished) all
// play independently rather than cutting each other off -- the host
// plays a fresh clone each time (crust-game.html's own soundCreate/
// playSound), the standard trick for short SFX.
func playSoundFn(args ...object.Object) object.Object {
	if len(args) != 1 {
		return wrongArgCount("playSound", "1", len(args))
	}
	id, errObj := handleArg("playSound", 0, args[0])
	if errObj != nil {
		return errObj
	}
	host().Call("playSound", id)
	return object.NULL
}

// stopSound(handle) -- stops (and rewinds) this sound's own base
// track. Only affects a currently-looping playSound(handle, ...)-style
// long-running track, not independent one-shot clones already fired by
// earlier playSound calls -- those finish on their own, the same
// "fire and forget" reasoning playSound's own doc comment gives for
// why they're cloned in the first place.
func stopSoundFn(args ...object.Object) object.Object {
	if len(args) != 1 {
		return wrongArgCount("stopSound", "1", len(args))
	}
	id, errObj := handleArg("stopSound", 0, args[0])
	if errObj != nil {
		return errObj
	}
	host().Call("stopSound", id)
	return object.NULL
}

// mouseX()/mouseY() -> the pointer's current position in world space
// (i.e. already run through the same camera-aware conversion setPos's
// own coordinates live in -- a click at a given world (x, y) lines up
// with whatever's actually drawn there, panned/zoomed camera or not,
// rather than forcing every game to redo that math itself). Touch
// input reports through the same two calls -- the host tracks the most
// recent touch point as the mouse position, so a game never has to
// branch on input device.
func mouseXFn(args ...object.Object) object.Object {
	if len(args) != 0 {
		return wrongArgCount("mouseX", "0", len(args))
	}
	return &object.Float{Value: host().Call("mouseX").Float()}
}

func mouseYFn(args ...object.Object) object.Object {
	if len(args) != 0 {
		return wrongArgCount("mouseY", "0", len(args))
	}
	return &object.Float{Value: host().Call("mouseY").Float()}
}

// mouseDown(button) -> bool. Level-triggered, same as keyDown -- true
// for every frame the button is physically held, from the real
// pointerdown/pointerup pair the browser gives us (no per-tick
// redefinition needed here the way crust studio's terminal-only keyDown
// required, since a browser mouse genuinely has a release event).
// button is "left", "right", or "middle" -- named the way a game
// script reads naturally, rather than the DOM's own 0/1/2 button
// index.
func mouseDownFn(args ...object.Object) object.Object {
	if len(args) != 1 {
		return wrongArgCount("mouseDown", "1", len(args))
	}
	button, ok := args[0].(*object.String)
	if !ok {
		return wrongArgType("mouseDown", 0, "a String", args[0])
	}
	return object.NativeBoolToBooleanObject(host().Call("mouseDown", button.Value).Bool())
}

// mouseClicked(button) -> bool. Edge-triggered -- true only during the
// frame a press started, then false again, the "GetMouseButtonDown"
// half of the usual down/held/clicked trio (mouseDown above is the
// "held" half). A touch tap fires this the same way a click does.
func mouseClickedFn(args ...object.Object) object.Object {
	if len(args) != 1 {
		return wrongArgCount("mouseClicked", "1", len(args))
	}
	button, ok := args[0].(*object.String)
	if !ok {
		return wrongArgType("mouseClicked", 0, "a String", args[0])
	}
	return object.NativeBoolToBooleanObject(host().Call("mouseClicked", button.Value).Bool())
}

// setFrame(handle, col, row, frameW, frameH) -- crops a sprite() handle
// to a single frameW x frameH cell out of its source image, at column
// col, row row (both 0-based), the standard sprite-sheet convention:
// draw one big image once, then just change which rectangle of it is
// visible each animation frame instead of loading a separate image per
// frame. Only meaningful on a sprite() handle -- the host silently
// no-ops on anything else, the same "the host just won't find a
// matching case" posture destroy() already documented for a stale
// handle.
func setFrameFn(args ...object.Object) object.Object {
	if len(args) != 5 {
		return wrongArgCount("setFrame", "5", len(args))
	}
	id, errObj := handleArg("setFrame", 0, args[0])
	if errObj != nil {
		return errObj
	}
	col, ok := numericValue(args[1])
	if !ok {
		return wrongArgType("setFrame", 1, "a number", args[1])
	}
	row, ok := numericValue(args[2])
	if !ok {
		return wrongArgType("setFrame", 2, "a number", args[2])
	}
	frameW, ok := numericValue(args[3])
	if !ok {
		return wrongArgType("setFrame", 3, "a number", args[3])
	}
	frameH, ok := numericValue(args[4])
	if !ok {
		return wrongArgType("setFrame", 4, "a number", args[4])
	}
	host().Call("setFrame", id, col, row, frameW, frameH)
	return object.NULL
}

// setLayer(handle, z) -- controls draw order within the world: higher z
// draws on top of lower z, ties broken by spawn order (PixiJS's own
// default). Every handle starts at z 0 (spawn order alone decides
// overlap until this is called), so a game that never needs layering
// never has to think about it.
func setLayerFn(args ...object.Object) object.Object {
	if len(args) != 2 {
		return wrongArgCount("setLayer", "2", len(args))
	}
	id, errObj := handleArg("setLayer", 0, args[0])
	if errObj != nil {
		return errObj
	}
	z, ok := numericValue(args[1])
	if !ok {
		return wrongArgType("setLayer", 1, "a number", args[1])
	}
	host().Call("setLayer", id, z)
	return object.NULL
}

// clearScene() destroys every handle spawned so far (rect/circle/
// sprite/text/sound alike) in one call -- the minimal primitive a
// level transition or "restart this wave" needs, deliberately not a
// full scene-graph/stack: onFrame's own registered callback, key
// state, and the camera are all left exactly as they are, since a new
// scene usually still wants the same input handler and often the same
// camera position it already had. A full run restart (pressing "Run"
// again) still goes through resetGameState, which clears those too.
func clearSceneFn(args ...object.Object) object.Object {
	if len(args) != 0 {
		return wrongArgCount("clearScene", "0", len(args))
	}
	entities = map[int64]*gameEntity{}
	animations = map[int64]*animState{}
	host().Call("clearScene")
	return object.NULL
}

// tileAt(layout, tileSize, x, y) -> String. Pure function -- recomputes
// the answer from the same layout String loadTilemap was given rather
// than tracking tile solidity engine-side, so it works even against a
// map that was never passed to loadTilemap at all (or a second map
// layered on top of the first, or one that changes at runtime). Which
// characters count as "solid" is left entirely to the caller, e.g.
// tileAt(layout, tileSize, x, y) == "#" -- the same reasoning
// loadTilemap's own palette argument already leans on: cRust doesn't
// get to decide what a map's symbols mean. Returns "" for a world
// position outside the layout's rows/columns (or past the end of a
// short row) -- querying past a map's edge is normal, not an error.
func tileAtFn(args ...object.Object) object.Object {
	if len(args) != 4 {
		return wrongArgCount("tileAt", "4", len(args))
	}
	layout, ok := args[0].(*object.String)
	if !ok {
		return wrongArgType("tileAt", 0, "a String (rows separated by newlines)", args[0])
	}
	tileSize, ok := numericValue(args[1])
	if !ok || tileSize <= 0 {
		return wrongArgType("tileAt", 1, "a positive number", args[1])
	}
	x, ok := numericValue(args[2])
	if !ok {
		return wrongArgType("tileAt", 2, "a number", args[2])
	}
	y, ok := numericValue(args[3])
	if !ok {
		return wrongArgType("tileAt", 3, "a number", args[3])
	}

	col := int(x / tileSize)
	row := int(y / tileSize)
	if col < 0 || row < 0 {
		return &object.String{Value: ""}
	}
	rows := strings.Split(layout.Value, "\n")
	if row >= len(rows) {
		return &object.String{Value: ""}
	}
	runes := []rune(rows[row])
	if col >= len(runes) {
		return &object.String{Value: ""}
	}
	return &object.String{Value: string(runes[col])}
}

// emitParticles(x, y, count, color, speed, lifetime) -- a fire-and-
// forget burst of count small dots at (x, y), each flying off in a
// random direction at up to speed pixels/second and fading out over
// lifetime seconds. Deliberately has no handle -- a hit spark or a
// death poof isn't a thing a game script ever needs to setPos or
// destroy again once it's launched, so there's nothing to hand back
// (unlike every other spawning builtin here). The host owns and
// animates the particles entirely on its own, ticked from the same
// frame(ts) rAF loop that already drives onFrame.
func emitParticlesFn(args ...object.Object) object.Object {
	if len(args) != 6 {
		return wrongArgCount("emitParticles", "6", len(args))
	}
	x, ok := numericValue(args[0])
	if !ok {
		return wrongArgType("emitParticles", 0, "a number", args[0])
	}
	y, ok := numericValue(args[1])
	if !ok {
		return wrongArgType("emitParticles", 1, "a number", args[1])
	}
	count, ok := numericValue(args[2])
	if !ok {
		return wrongArgType("emitParticles", 2, "a number", args[2])
	}
	color, ok := args[3].(*object.String)
	if !ok {
		return wrongArgType("emitParticles", 3, `a String hex color (e.g. "#ff6a3d")`, args[3])
	}
	speed, ok := numericValue(args[4])
	if !ok {
		return wrongArgType("emitParticles", 4, "a number", args[4])
	}
	lifetime, ok := numericValue(args[5])
	if !ok {
		return wrongArgType("emitParticles", 5, "a number (seconds)", args[5])
	}
	host().Call("emitParticles", x, y, count, color.Value, speed, lifetime)
	return object.NULL
}

// tickAnimations advances every playAnimation()'d handle by dt seconds
// -- called once per crustGameFrame (main.go), before the game
// script's own onFrame(fn), so an animated sprite keeps cycling even
// in a program that never calls onFrame at all (a static scene with
// one idle decoration). Catches up more than one frame step per call
// with a bounded while rather than a single if, so a stutter (a slow
// frame, a backgrounded tab) skips intermediate frames instead of
// visibly slowing the animation down.
func tickAnimations(dt float64) {
	for id, a := range animations {
		a.elapsed += dt
		frameDur := 1.0 / a.fps
		advanced := false
		for a.elapsed >= frameDur {
			a.elapsed -= frameDur
			a.frame = (a.frame + 1) % a.frameCount
			advanced = true
		}
		if advanced {
			host().Call("setFrame", id, a.frame, a.row, a.frameW, a.frameH)
		}
	}
}

// playAnimation(handle, row, frameCount, fps, frameW, frameH) starts a
// continuously looping sprite-sheet animation on a sprite() handle --
// row/frameW/frameH pick out the same sprite-sheet cells setFrame's
// own col/row/frameW/frameH argument would, cycling through columns
// 0..frameCount-1 at fps frames per second instead of a script having
// to compute which frame is due itself every tick. There's no
// play-once mode -- the common "loop this walk cycle" case doesn't
// need one, and a script that wants a one-shot animation can call
// stopAnimation(handle) once its own exit condition (a fixed frame
// count elapsed, a state change) is met. Calling this again on a
// handle that's already animating just replaces its animation with the
// new one, restarting from frame 0 -- the same "last call wins, no
// hidden queueing" contract every other mutating builtin here has.
func playAnimationFn(args ...object.Object) object.Object {
	if len(args) != 6 {
		return wrongArgCount("playAnimation", "6", len(args))
	}
	id, errObj := handleArg("playAnimation", 0, args[0])
	if errObj != nil {
		return errObj
	}
	row, ok := numericValue(args[1])
	if !ok {
		return wrongArgType("playAnimation", 1, "a number", args[1])
	}
	frameCount, ok := numericValue(args[2])
	if !ok || frameCount < 1 {
		return wrongArgType("playAnimation", 2, "a positive number", args[2])
	}
	fps, ok := numericValue(args[3])
	if !ok || fps <= 0 {
		return wrongArgType("playAnimation", 3, "a positive number", args[3])
	}
	frameW, ok := numericValue(args[4])
	if !ok {
		return wrongArgType("playAnimation", 4, "a number", args[4])
	}
	frameH, ok := numericValue(args[5])
	if !ok {
		return wrongArgType("playAnimation", 5, "a number", args[5])
	}
	animations[id] = &animState{row: int(row), frameCount: int(frameCount), frameW: frameW, frameH: frameH, fps: fps}
	host().Call("setFrame", id, 0, row, frameW, frameH)
	return object.NULL
}

// stopAnimation(handle) freezes handle on whatever frame it was
// currently showing -- doesn't reset it back to frame 0 or touch
// setFrame's own crop at all, so a script that wants a specific
// "landed" pose afterward can still call setFrame(handle, ...) itself
// right after this.
func stopAnimationFn(args ...object.Object) object.Object {
	if len(args) != 1 {
		return wrongArgCount("stopAnimation", "1", len(args))
	}
	id, errObj := handleArg("stopAnimation", 0, args[0])
	if errObj != nil {
		return errObj
	}
	delete(animations, id)
	return object.NULL
}

// objectToJSON converts a cRust value into a JSON-compatible Go value
// for save() to hand to the host's localStorage-backed store --
// deliberately supports only what JSON itself can represent
// losslessly: Integer, Float, String, Boolean, Nil, List, and a Map
// with String keys. cRust's other Hashable Map key types (Integer,
// Float, Boolean) have no JSON object-key equivalent, so a Map keyed
// by anything else is a save() error rather than a silent
// stringify-the-key that would quietly change the key's type on the
// next load(). Tuple, Set, and Function have no JSON shape at all and
// are errors too.
func objectToJSON(name string, obj object.Object) (any, *object.Error) {
	switch v := obj.(type) {
	case *object.Integer:
		return v.Value, nil
	case *object.Float:
		return v.Value, nil
	case *object.String:
		return v.Value, nil
	case *object.Boolean:
		return v.Value, nil
	case *object.Null:
		return nil, nil
	case *object.List:
		out := make([]any, len(v.Elements))
		for i, e := range v.Elements {
			j, errObj := objectToJSON(name, e)
			if errObj != nil {
				return nil, errObj
			}
			out[i] = j
		}
		return out, nil
	case *object.Map:
		out := make(map[string]any, len(v.Pairs))
		for _, pair := range v.Pairs {
			key, ok := pair.Key.(*object.String)
			if !ok {
				return nil, newError("%s: Map keys must be String to be saved, got %s", name, pair.Key.Type())
			}
			j, errObj := objectToJSON(name, pair.Value)
			if errObj != nil {
				return nil, errObj
			}
			out[key.Value] = j
		}
		return out, nil
	default:
		return nil, newError("%s: cannot save a %s value (only Integer/Float/String/Boolean/nobox/List/Map are supported)", name, obj.Type())
	}
}

// jsonToObject is objectToJSON's inverse, used by load() on whatever
// encoding/json.Unmarshal hands back. JSON has no separate integer
// type -- Go's decoder always produces float64 -- so a decoded whole
// number round-trips back to an Integer (matching what a script saved
// with save(key, someInteger) would expect to read back), and only a
// genuinely fractional value becomes a Float.
func jsonToObject(v any) object.Object {
	switch t := v.(type) {
	case nil:
		return object.NULL
	case bool:
		return object.NativeBoolToBooleanObject(t)
	case float64:
		if t == math.Trunc(t) && !math.IsInf(t, 0) {
			return object.NewInteger(int64(t))
		}
		return &object.Float{Value: t}
	case string:
		return &object.String{Value: t}
	case []any:
		elems := make([]object.Object, len(t))
		for i, e := range t {
			elems[i] = jsonToObject(e)
		}
		return object.NewList(elems)
	case map[string]any:
		m := object.NewMap()
		for k, val := range t {
			m.Set(&object.String{Value: k}, jsonToObject(val))
		}
		return m
	default:
		return object.NULL
	}
}

// save(key, value) persists value under key, surviving a page reload
// or the tab closing -- backed by the browser's own localStorage
// (index.html's own save/load host methods), scoped per-origin the
// same way every other localStorage use is, so a high score or a
// settings Map set by one `crust game` session is still there next
// time the same page is opened. value is serialized to JSON first
// (see objectToJSON) rather than stored as some opaque cRust-specific
// format, so what's actually sitting in localStorage is ordinary,
// inspectable JSON if anyone goes looking in devtools.
func saveFn(args ...object.Object) object.Object {
	if len(args) != 2 {
		return wrongArgCount("save", "2", len(args))
	}
	key, ok := args[0].(*object.String)
	if !ok {
		return wrongArgType("save", 0, "a String", args[0])
	}
	data, errObj := objectToJSON("save", args[1])
	if errObj != nil {
		return errObj
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return newError("save: %s", err)
	}
	host().Call("save", key.Value, string(encoded))
	return object.NULL
}

// load(key) -> the value previously save()'d under key, or nobox if
// nothing was ever saved there (a brand new player, a cleared
// browser profile) -- a missing key is an ordinary, expected state to
// handle, not an error, the same "absence reads as nobox" convention
// Map's own pop/Get already give a missing key.
func loadFn(args ...object.Object) object.Object {
	if len(args) != 1 {
		return wrongArgCount("load", "1", len(args))
	}
	key, ok := args[0].(*object.String)
	if !ok {
		return wrongArgType("load", 0, "a String", args[0])
	}
	v := host().Call("load", key.Value)
	if v.IsNull() || v.IsUndefined() {
		return object.NULL
	}
	var decoded any
	if err := json.Unmarshal([]byte(v.String()), &decoded); err != nil {
		return newError("load: corrupt saved data for %q: %s", key.Value, err)
	}
	return jsonToObject(decoded)
}
