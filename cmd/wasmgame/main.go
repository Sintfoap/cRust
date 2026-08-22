//go:build js && wasm

// Command wasmgame builds crust-game.wasm, the browser-side half of
// `crust game` -- a live, in-browser game runtime for cRust, distinct
// from cmd/wasm's crust.wasm (which runs a whole program to completion
// once per crustRun call, exactly what `crust bake playground` needs
// and exactly wrong for a game). A game has to keep running: the same
// interpreter and environment stay alive across many calls, one per
// browser animation frame, so a program's own state (sprite positions,
// score, whatever variables it declared at the top level) persists
// between frames instead of resetting every time.
//
// Three globals, mirroring cmd/wasm's single crustRun export:
//
//   - crustGameInit(source) -- parse + run source's top-level code once
//     (the same "top-level code runs first" any cRust program already
//     has -- SPEC.md §9's store/store_<name> entry-point convention
//     doesn't apply here at all; there's no one-shot "answer" to
//     compute, so top-level code doubles as setup: call rect/circle to
//     spawn sprites, onFrame to register the per-frame callback).
//   - crustGameFrame(dtSeconds) -- calls whatever onFrame(fn) registered,
//     once per requestAnimationFrame tick (crust-game.html drives this).
//   - crustGameKey(name, down) -- updates the pressed-keys set keyDown()
//     reads, from the page's own keydown/keyup listeners.
//
// Rebuild with scripts/build-wasm-game.sh after any language or game
// builtin change; the built artifact is checked into
// cmd/crust/game_assets/ since cmd/crust embeds it with go:embed at
// compile time, same as crust.wasm.
package main

import (
	"fmt"
	"strings"
	"syscall/js"

	"github.com/Sintfoap/cRust/internal/interpreter"
	"github.com/Sintfoap/cRust/internal/lexer"
	"github.com/Sintfoap/cRust/internal/object"
	"github.com/Sintfoap/cRust/internal/parser"
)

// interp is the one live Interpreter for the currently-running game,
// nil until crustGameInit succeeds at least once. Rebuilt (not reused)
// on every crustGameInit call, so a re-run after editing the source
// gets a fresh Environment rather than one still holding the previous
// run's variables.
var interp *interpreter.Interpreter

// consoleWriter relays deliver() output to the browser's own devtools
// console -- the natural "where does print output go" answer for code
// that never has a terminal to write to, and it means deliver() stays
// usable for ad hoc debugging inside a running game exactly like it
// already is everywhere else in cRust. One console.log call per
// deliverFn call (SPEC.md §7 -- deliver always writes one line at a
// time), not a buffered writer -- there's no "end of program" moment to
// flush at when the whole point is running forever.
type consoleWriter struct{}

func (consoleWriter) Write(p []byte) (int, error) {
	js.Global().Get("console").Call("log", strings.TrimRight(string(p), "\n"))
	return len(p), nil
}

func okResult() any {
	return js.ValueOf(map[string]any{"error": ""})
}

func errResult(msg string) any {
	return js.ValueOf(map[string]any{"error": msg})
}

// crustGameInit(source) -> {error}. Parses and evaluates source's
// top-level code against a brand-new Interpreter/Environment, with the
// game builtin table (builtins.go) merged in on top of the ordinary
// stdlib -- rect/circle/setPos/... are only ever meaningful here, never
// in the plain CLI interpreter, so they're added after
// interpreter.New's own construction rather than folded into
// internal/builtins itself (see gameBuiltins' own doc comment).
// Leaves the previous run's interp untouched until this one succeeds,
// so a parse/runtime error while editing doesn't kill an
// already-running game's next frame out from under it.
func crustGameInit(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return errResult("crustGameInit: missing source argument")
	}
	src := args[0].String()

	l := lexer.New(src)
	p := parser.New(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		return errResult(fmt.Sprintf("%d parse error(s):\n  %s", len(errs), strings.Join(errs, "\n  ")))
	}

	resetGameState()
	next := interpreter.New(consoleWriter{}, strings.NewReader(""))
	for name, b := range gameBuiltins() {
		next.Builtins[name] = b
	}
	env := object.NewEnvironment()

	result := next.Eval(program, env)
	if errObj, ok := result.(*object.Error); ok {
		return errResult(runtimeErrMsg(errObj))
	}

	interp = next
	return okResult()
}

// crustGameFrame(dtSeconds) -> {error}. A no-op success (not an error)
// when no game is running yet or the running one never called
// onFrame(fn) -- both are ordinary states (page just loaded; a program
// that only ever draws a static scene), not failures. Ticks any
// playAnimation()'d sprites first, before onFrameFn == nil's own early
// return, so an animation keeps cycling even in a program that never
// registers a per-frame callback at all.
func crustGameFrame(_ js.Value, args []js.Value) any {
	if interp == nil {
		return okResult()
	}
	dt := 0.0
	if len(args) > 0 {
		dt = args[0].Float()
	}
	tickAnimations(dt)
	if onFrameFn == nil {
		return okResult()
	}
	result := interp.Call(onFrameFn, []object.Object{&object.Float{Value: dt}})
	if errObj, ok := result.(*object.Error); ok {
		return errResult(runtimeErrMsg(errObj))
	}
	return okResult()
}

// crustGameKey(name, down) updates the key state keyDown() reads. name
// is the browser's own KeyboardEvent.key ("ArrowLeft", "a", " ", ...),
// forwarded verbatim by crust-game.html's own keydown/keyup listeners.
func crustGameKey(_ js.Value, args []js.Value) any {
	if len(args) < 2 {
		return nil
	}
	pressedKeys[args[0].String()] = args[1].Bool()
	return nil
}

func runtimeErrMsg(errObj *object.Error) string {
	return fmt.Sprintf("%d:%d: %s", errObj.Line, errObj.Col, errObj.Message)
}

func main() {
	js.Global().Set("crustGameInit", js.FuncOf(crustGameInit))
	js.Global().Set("crustGameFrame", js.FuncOf(crustGameFrame))
	js.Global().Set("crustGameKey", js.FuncOf(crustGameKey))
	select {}
}
