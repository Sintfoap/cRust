#!/usr/bin/env bash
# Regenerates the assets `crust game` embeds and serves: crust-game.wasm
# (built from cmd/wasmgame) and wasm_exec.js (the JS runtime glue Go's
# WASM output needs, copied straight from the toolchain so it always
# matches the Go version that built the .wasm) -- the same pipeline
# build-wasm.sh already runs for `crust bake playground`'s crust.wasm,
# duplicated rather than shared since the two .wasm files come from
# different cmd/ packages and land in different embedded directories.
# Run this after any language or game-builtin change (cmd/wasmgame) --
# cmd/crust/game.go embeds these files at compile time, so a stale
# crust-game.wasm silently serves old behavior.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

out=cmd/crust/game_assets
mkdir -p "$out"

echo "Building crust-game.wasm (GOOS=js GOARCH=wasm)..."
GOOS=js GOARCH=wasm go build -o "$out/crust-game.wasm" ./cmd/wasmgame/

wasm_exec="$(go env GOROOT)/lib/wasm/wasm_exec.js"
if [ ! -f "$wasm_exec" ]; then
    # Go < 1.24 shipped this under misc/wasm instead of lib/wasm.
    wasm_exec="$(go env GOROOT)/misc/wasm/wasm_exec.js"
fi
if [ ! -f "$wasm_exec" ]; then
    echo "error: couldn't find wasm_exec.js under \$(go env GOROOT)" >&2
    exit 1
fi
echo "Copying $wasm_exec"
cp "$wasm_exec" "$out/wasm_exec.js"

echo "Done. $out now has:"
ls -la "$out"
