<p align="center">
  <img src="./assets/banner.png" alt="cRust — language baked better" width="700">
</p>

An interpreted programming language, written in Go, where every keyword is
pizza jargon. Built to solve [Advent of Code 2026](https://adventofcode.com/).

## Status

cRust actually runs code now. Project foundations, language design, the
lexer, the parser, and the interpreter (Phases 0–4) are done —
`.crust` source turns into a token stream (`internal/lexer`), a full
AST (`internal/ast`, `internal/parser`), and now a real result
(`internal/interpreter`, `internal/object`), with a first pass at the
standard library (`internal/builtins`: `deliver`, `slices`, `sauce`,
`chars`, `idiv`, and the Set family) built alongside it. `crust run
<file.crust>` (and the bare-file shorthand) both work end to end,
closures and all — see [Building](#building) below. See
[TODO.md](./TODO.md) for the roadmap and milestones,
[docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md) for the technical design
behind each phase (including a
[Performance Strategy](./docs/ARCHITECTURE.md#5-performance-strategy)
section), and [docs/SPEC.md](./docs/SPEC.md) for the actual language —
keyword table, grammar, and semantics. Next up: rounding out Phase 5's
standard library (string/math builtins, file input) and Phase 6's REPL.

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

See [`examples/`](./examples) for runnable sample programs — AoC-shaped
ones plus `the_works.crust` and `closures.crust`, which between them
exercise every keyword, operator, and builtin in `SPEC.md` at least
once (`crust run examples/the_works.crust` and friends all work today).

## Building

Requires Go 1.24+.

```
go build ./cmd/crust
./crust --version
./crust --help
./crust examples/hello.crust     # or: ./crust run examples/hello.crust
```

`crust repl` still just says "not implemented yet" (see
[TODO.md](./TODO.md)) — `crust run <file.crust>` (also reachable as a
bare `crust <file.crust>`) is real, though: it lexes, parses, and
evaluates the file end to end. If the file defines a `recipe store()`
(or named variants, `recipe store_part1()`/`store_part2()`/...), that's
run as the entry point after the rest of the file's top-level code;
`--store=<name>` picks a named one instead of the bare `store`
(SPEC.md §9) — handy for AoC's usual part-1/part-2 split:

```
crust day01.crust                  # runs store, if the file has one
crust day01.crust --store=part2    # runs store_part2 instead
```

`--help` (and running `crust` with no arguments) prints the pizza
banner in color; customize it with:

```
crust --toppings=all --help     # everything: pepperoni + basil
crust --toppings=plain --help   # just cheese
crust --no-color --help         # plain text, no ANSI (also respects $NO_COLOR)
crust --no-banner --help        # usage only, no pizza
```

The lexer and parser also have their own debug commands, useful for
seeing exactly how a file lexes/parses without running it:

```
crust tokens examples/hello.crust
#    1:1    IDENT      deliver
#    1:8    (          (
#    1:9    STRING     "Hello, World!"
#    1:24   )          )
#    1:25   NEWLINE    \n
#    2:1    EOF

crust parse examples/hello.crust
#1: deliver("Hello, World!")
```

### With Nix

```
nix run github:Sintfoap/cRust -- --version
nix build github:Sintfoap/cRust      # ./result/bin/crust
nix develop github:Sintfoap/cRust    # dev shell with Go on PATH
```

Works from NixOS, Nix-on-WSL, or Nix on any other Linux/macOS system.

### Editor syntax highlighting

Vim/Neovim and VSCode both have working syntax highlighting for
`.crust` files — see [`editors/vim`](./editors/vim) and
[`editors/vscode`](./editors/vscode) for install instructions. Neovim
users who want more accurate, parser-driven highlighting instead of
the regex-based Vim syntax file (the same kind `nvim-treesitter`-style
tooling is built on) can use [`editors/tree-sitter-crust`](./editors/tree-sitter-crust)
instead — a real grammar, not a token-pattern list.
