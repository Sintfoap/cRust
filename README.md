<p align="center">
  <img src="./assets/banner.png" alt="cRust — language baked better" width="700">
</p>

An interpreted programming language, written in Go, where every keyword is
pizza jargon. Built to solve [Advent of Code 2026](https://adventofcode.com/).

## Status

Project foundations, language design, and the lexer (Phases 0–2) are
done — `.crust` source turns into a token stream (`internal/lexer`),
but there's no parser or interpreter yet, so `crust run` still just
says "not implemented yet." See [TODO.md](./TODO.md) for the roadmap
and milestones, [docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md) for the
technical design behind each phase, and [docs/SPEC.md](./docs/SPEC.md)
for the actual language — keyword table, grammar, and semantics. Next
up: Phase 3, the parser.

## Why

AoC rewards a language you enjoy typing at 6am with a puzzle timer running.
cRust is that language: a small, tree-walking interpreter with just enough
features (ints, floats, strings, lists, maps, sets, functions, control
flow) to comfortably solve AoC-style puzzles, wrapped in a
pizzeria-themed syntax. No `let`/`const` ceremony — variables just get
assigned, Python-style.

## A Taste

```
recipe findPair(nums, target) {
    knead (i = 0; i < slices(nums); i++) {
        knead (j = i + 1; j < slices(nums); j++) {
            order (nums[i] + nums[j] == target) {
                serve [nums[i], nums[j]]
            }
        }
    }
    serve nobox
}
```

See [`examples/`](./examples) for runnable-once-the-interpreter-exists
sample programs.

## Building

Requires Go 1.24+.

```
go build ./cmd/crust
./crust --version
./crust --help
```

`crust run`/`crust repl` (also reachable as a bare `crust <file>`) exist
but just say "not implemented yet" — the interpreter behind them lands
in Phases 2–6. `--help` (and running `crust` with no arguments) prints
the pizza banner in color; customize it with:

```
crust --toppings=all --help     # everything: pepperoni + basil
crust --toppings=plain --help   # just cheese
crust --no-color --help         # plain text, no ANSI (also respects $NO_COLOR)
crust --no-banner --help        # usage only, no pizza
```

### With Nix

```
nix run github:Sintfoap/cRust -- --version
nix build github:Sintfoap/cRust      # ./result/bin/crust
nix develop github:Sintfoap/cRust    # dev shell with Go on PATH
```

Works from NixOS, Nix-on-WSL, or Nix on any other Linux/macOS system.
