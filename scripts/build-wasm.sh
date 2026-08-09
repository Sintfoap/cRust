#!/usr/bin/env bash
# Regenerates the assets `crust bake playground` embeds and serves:
# crust.wasm (built from cmd/wasm) and wasm_exec.js (the JS runtime
# glue Go's WASM output needs, copied straight from the toolchain so it
# always matches the Go version that built the .wasm). Run this after
# any language change — cmd/crust/playground.go embeds these files at
# compile time, so a stale crust.wasm silently serves old behavior.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

out=cmd/crust/playground_assets
mkdir -p "$out"

echo "Building crust.wasm (GOOS=js GOARCH=wasm)..."
GOOS=js GOARCH=wasm go build -o "$out/crust.wasm" ./cmd/wasm/

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
