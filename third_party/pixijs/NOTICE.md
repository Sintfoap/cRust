# PixiJS

`cmd/crust/game_assets/pixi.min.js` is PixiJS 8.20.0's own prebuilt
browser bundle (`dist/pixi.min.js` from the `pixi.js` npm package),
vendored unmodified so `crust game` (README.md) works fully offline —
the same "nothing fetched at request time" posture `crust bake
playground` already has for crust.wasm/wasm_exec.js. Upstream:
<https://github.com/pixijs/pixijs>, MIT licensed (`LICENSE` in this
directory, copied from the same npm package).

To update: `npm view pixi.js dist.tarball`, download and extract that
tarball's `package/dist/pixi.min.js` over
`cmd/crust/game_assets/pixi.min.js`, and re-check
`cmd/crust/game_assets/index.html`'s PixiJS calls against whatever
changed — `Application`'s `init()` is async as of v8 and `Graphics`
uses chained `.rect(...).fill(...)` instead of the older
`beginFill`/`drawRect`/`endFill`, both breaking changes from v6/v7.
