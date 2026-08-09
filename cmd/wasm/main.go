//go:build js && wasm

// Command wasm builds crust.wasm, the browser-side half of `crust bake
// playground`. It exposes a single crustRun(source, storeFlag, stdin)
// global to JS, backed by the exact same internal/runner.Run pipeline
// the crust CLI's `run` subcommand and `develop`'s Run tab use — so the
// playground can't drift from what `crust run` actually does. Rebuild
// with scripts/build-wasm.sh after any language change; the built
// artifact is checked into cmd/crust/playground_assets/ since
// cmd/crust embeds it with go:embed at compile time.
package main

import (
	"bytes"
	"strings"
	"syscall/js"

	"github.com/Sintfoap/cRust/internal/runner"
)

// crustRun(source, storeFlag, stdin) -> {stdout, stderr, code}. Runs
// synchronously on the calling (browser main) thread, same as any other
// syscall/js export — a program with an infinite loop will hang the tab
// until reloaded, same caveat as pasting one into any other in-browser
// code sandbox.
func crustRun(_ js.Value, args []js.Value) any {
	var src, storeFlag, stdin string
	if len(args) > 0 {
		src = args[0].String()
	}
	if len(args) > 1 {
		storeFlag = args[1].String()
	}
	if len(args) > 2 {
		stdin = args[2].String()
	}

	var stdout, stderr bytes.Buffer
	code := runner.Run(src, storeFlag, "", "", strings.NewReader(stdin), &stdout, &stderr)

	return js.ValueOf(map[string]any{
		"stdout": stdout.String(),
		"stderr": stderr.String(),
		"code":   code,
	})
}

func main() {
	js.Global().Set("crustRun", js.FuncOf(crustRun))
	select {}
}
