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
`chars`, `idiv`, the Set family, input (`unbox`/`lines`), and type
conversion (`str`/`int`/`float`/`bool`)) built alongside it. `crust run
<file.crust>` (and the bare-file shorthand) both work end to end,
closures and all — see [Building](#building) below. `crust lsp` adds
editor hover, diagnostics, go-to-definition, and more on top
(`internal/lsp`, JSON-RPC over stdio) — see
[Language server](#language-server) below. `crust debug` adds a
step-by-step debugger with time/memory-per-function KPIs
(`internal/trace`, `internal/debugger`) — see
[Debugging](#debugging-time-and-memory-per-function) above. See [TODO.md](./TODO.md)
for the roadmap and milestones,
[docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md) for the technical design
behind each phase (including a
[Performance Strategy](./docs/ARCHITECTURE.md#5-performance-strategy)
section), and [docs/SPEC.md](./docs/SPEC.md) for the actual language —
keyword table, grammar, and semantics. Next up: rounding out Phase 5's
standard library (general string helpers, math) and Phase 6's REPL.

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

### Debugging: time and memory per function

`crust debug day01.crust` runs the program and shows exactly where the
time and memory went, statement by statement: a step-by-step trace
tree (recipe calls and `knead`/`bake` loop laps are frames you can see
into) plus a ranking of every function/loop by *self* time — the work
it's actually responsible for, not counting whatever it delegated to a
call or another loop — so a recursive `fib` shows up as one bucket
across every recursion depth, not one row per call site. On a real
terminal this opens an interactive two-tab TUI (KPI pie charts, plus
the tree stepper); piped or redirected, or with `--plain`, it prints
the same information as text:

```
crust debug day01.crust
# day01.crust — 11 steps · 102µs total
# step                                    out            size   time    self%
# recipe findPair(nums, target) { …       recipe(...)      —  1.4µs     1.3%
# entries = [1721, 979, ...]              [1721, ...]      6  1.5µs     1.5%
# pair = findPair(entries, 2020)          [1721, 299]      2 35.1µs     2.3%
#   findPair(...)
#     ...
#
# by self time:
#   name                     calls    self      %  size
#   findPair(...)                1  32.7µs  32.0%     —
```

`--store=<name>` picks the entry point the same way `crust run` does;
`--max-steps N` bounds the recording for a program that loops far more
than a terminal (or a human) wants to read through.

### With Nix

```
nix run github:Sintfoap/cRust -- --version
nix build github:Sintfoap/cRust           # ./result/bin/crust
nix develop github:Sintfoap/cRust         # dev shell with Go on PATH, for hacking on cRust itself
nix develop github:Sintfoap/cRust#crust   # dev shell with just the `crust` CLI on PATH, for using it
```

`nix profile add github:Sintfoap/cRust` also works, but needs the
resulting `~/.nix-profile/bin` on your `PATH` — if `crust` isn't found
afterward, that's almost always why. `nix develop .#crust` (run from
inside a clone of this repo, or any local flake with `crust.url =
"github:Sintfoap/cRust"` in its inputs and `crust.packages.${system}.default`
in a devShell) sidesteps that entirely: no profile, no PATH changes,
`crust` is just there for the shell session.

Works from NixOS, Nix-on-WSL, or Nix on any other Linux/macOS system.

### Editor syntax highlighting

Vim/Neovim and VSCode both have working syntax highlighting for
`.crust` files — see [`editors/vim`](./editors/vim) and
[`editors/vscode`](./editors/vscode) for install instructions. Neovim
users who want more accurate, parser-driven highlighting instead of
the regex-based Vim syntax file (the same kind `nvim-treesitter`-style
tooling is built on) can use [`editors/tree-sitter-crust`](./editors/tree-sitter-crust)
instead — a real grammar, not a token-pattern list.

### Language server

`crust lsp` starts a language server on stdin/stdout: hover docs for
keywords/builtins/literals, live diagnostics (lex/parse errors), go-to-
definition, find-references, rename, document symbols (recipe
outline), and completion (keywords, builtins, and every declared name
in the file) — all resolved through cRust's real function-scope
nesting (SPEC.md §3), not plain text matching, so two functions with
identically-named parameters correctly resolve to two different
declarations. It speaks plain JSON-RPC 2.0 over stdio, so any LSP
client can launch `crust lsp` as the command; for Neovim specifically,
`vim.lsp.start()` needs no plugin beyond what you likely already have:

```lua
vim.filetype.add({ extension = { crust = "crust" } })

vim.api.nvim_create_autocmd("FileType", {
  pattern = "crust",
  callback = function(args)
    vim.lsp.start({
      name = "crust_ls",
      cmd = { "crust", "lsp" },
      root_dir = vim.fs.root(args.buf, { ".git", "flake.nix" }) or vim.fn.getcwd(),
    })
  end,
})
```

Drop that in `lua/config/autocmds.lua` (or wherever your config keeps
general autocommands) and make sure `crust` itself is on `PATH` (see
[With Nix](#with-nix) above). No LSP server registration/Mason install
needed — `crust` *is* the language server.
