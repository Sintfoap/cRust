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
[Language server](#language-server) below. `crust develop` adds a
step-by-step debugger with time/memory-per-function KPIs
(`internal/trace`, `internal/debugger`) — see
[Debugging](#debugging-time-and-memory-per-function) above. `crust bake
documentation` serves a browsable, pizza-themed reference site on
`localhost` — see [Documentation site](#documentation-site) below.
`crust repl` starts an interactive session — see
[Building](#building) below. `crust fmt` canonically reformats source
(`internal/format`), reachable both as a standalone CLI and as
`crust lsp`'s `textDocument/formatting` — see
[Language server](#language-server) below. See [TODO.md](./TODO.md) for the roadmap
and milestones, [docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md) for the
technical design behind each phase (including a
[Performance Strategy](./docs/ARCHITECTURE.md#5-performance-strategy)
section), [docs/SPEC.md](./docs/SPEC.md) for the actual language —
keyword table, grammar, and semantics — and
[docs/CHEATSHEET.md](./docs/CHEATSHEET.md) for a one-page quick
reference (keywords, operators, every builtin, a handful of idioms)
once you already know the language and just need a lookup. Phase 6's tooling is now
essentially complete; next up is rounding out Phase 7's test coverage
and Phase 8's AoC-readiness checklist.

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
[`examples/aoc2020/`](./examples/aoc2020) goes a step further: real,
complete two-part solutions to AoC 2020 days 1–5, each verified against
the puzzle's own documented example answers — `crust run
examples/aoc2020/day01.crust --store=part1 < examples/aoc2020/day01_input.txt`
and friends.

## Building

Requires Go 1.24+.

```
go build ./cmd/crust
./crust --version
./crust --help
./crust examples/hello.crust     # or: ./crust run examples/hello.crust
```

`crust run <file.crust>` (also reachable as a bare `crust
<file.crust>`) lexes, parses, and evaluates the file end to end. If the
file defines a `recipe store()` (or named variants, `recipe
store_part1()`/`store_part2()`/...), that's run as the entry point
after the rest of the file's top-level code; `--store=<name>` picks a
named one instead of the bare `store` (SPEC.md §9) — handy for AoC's
usual part-1/part-2 split:

```
crust day01.crust                  # runs store, if the file has one
crust day01.crust --store=part2    # runs store_part2 instead
```

`crust repl` starts an interactive session — one persistent
environment for as long as it's open, so a variable or recipe defined
on one line is still there on the next:

```
$ crust repl
crust repl — Ctrl+D to quit
crust> x = 21
crust> x * 2
42
crust> recipe double(n) { serve n * 2 }
recipe(n) { ... }
crust> double(x)
42
```

An unclosed `{`/`(`/`[` (or a trailing operator with nothing after it)
switches to a `...>` continuation prompt instead of reporting an error
— keep typing and it picks back up once the construct is closed. Only
a bare expression's value echoes back (`x * 2` above, or a recipe
definition's own `recipe(n) { ... }` representation, as one last
confirmation it took); an assignment, loop, or conditional doesn't,
same as most REPLs. A runtime error is reported without ending the
session — everything bound before it is still there afterward.

`crust fmt <file.crust>` prints the file reformatted to cRust's one
canonical style (consistent spacing, indentation, and brace placement,
minimal-but-correct parens, comments and blank-line paragraph breaks
preserved) — same convention as `gofmt`: prints to stdout by default,
`-w` rewrites the file in place:

```
crust fmt messy.crust        # print the formatted result
crust fmt -w messy.crust     # rewrite messy.crust in place
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

`crust develop day01.crust` shows exactly where the time and memory
went in a run, statement by statement. If `day01.crust` doesn't exist
yet, it's created empty rather than treated as an error — starting a
new AoC day's file is the most common reason to point `develop` at a
path that isn't there, so it opens straight into the same tool instead
of a dead end (only the file itself is created, never a missing parent
directory). It's a step-by-step trace tree
(recipe calls and `knead`/`bake` loop laps are frames you can see
into, each closed by a `// end ...` marker so a long block's extent
reads the same way matching braces would) plus a ranking of every
function/loop by *self* time — the work it's actually responsible for,
not counting whatever it delegated to a call or another loop — so a
recursive `fib` shows up as one bucket across every recursion depth,
not one row per call site.

On a real terminal this opens an interactive six-tab TUI: **Time**
and **Memory** (one pie chart each, so both get the full window),
**Stepper** (the tree), **Editor** (opens `nvim` on the file being
debugged as soon as you switch to it — `nvim` behaves normally in
there, so `:w` just saves and keeps you editing; quitting after a
save, e.g. `:wq`, reruns the recording, refreshes the other tabs, and
takes you to the Time tab. Quitting *without* saving, e.g. a plain
`:q`, skips the rerun but still refreshes the Run tab's entry-point
list either way — so adding, removing, or renaming a `store`/
`store_<name>` recipe shows up there the moment you leave the editor,
not only after a save), **Run** (type a path to an input file
and press enter to run the file with it as stdin, showing the raw
output exactly like `crust run day01.crust < input.txt` would — no
tracing, just the program's own stdout/stderr; if the file declares
more than one `store`/`store_<name>` entry point, ↑↓ moves down to a
selector row listing them and ←→ picks one before you hit enter), and
**Files** (every other `.crust` file alongside the one currently
open — the natural "AoC folder full of day01.crust..day25.crust"
layout — ↑↓ to move, enter to switch straight to one without leaving
`crust develop` and relaunching it on a different path; each file
keeps its own remembered store/input/run-all settings, the same as
reopening it fresh from the command line would; `n` starts a new file
right there — type a name, enter creates it (`.crust` appended if you
leave it off) and switches straight to it).
Ctrl+R toggles running *every* entry point in sequence instead of just
the selected one — each against the same input file, one full run
apiece, output concatenated under a `=== part1 ===`-style heading per
store and their timing/debug info merged into one combined Time/
Memory/Stepper recording — handy for checking part 1 and part 2 (or
however many `store_<name>`s a file has) together without switching
the selector and re-running by hand each time.
Time/Memory/Stepper start out empty ("nothing recorded yet") rather
than running the program immediately — that first, automatic run used
to read real process stdin before the TUI had taken over the keyboard,
so any `store`/`store_<name>` recipe that called `unbox()` would
silently consume whatever you typed next as puzzle input instead of it
reaching the TUI at all. Running from the Run tab is what actually
populates those three tabs: it retraces that same entry point and
input file (or, with Ctrl+R's "run all" on, every entry point), so
whichever run you just did is what the rest of the TUI shows — and
remembers your entry point, input path, and run-all toggle for next
time, so reopening the same file later starts the Run tab back where
you left it (though Time/Memory/Stepper still start empty until you
press enter there again). Piped or redirected, or with `--plain`,
there's no Run tab to defer to, so it runs immediately and prints the
same information as text:

```
crust develop day01.crust
# day01.crust — 11 steps · 102µs total
# step                                    out            size   time    self%
# recipe findPair(nums, target) { …       recipe(...)      —  1.4µs     1.3%
# entries = [1721, 979, ...]              [1721, ...]      6  1.5µs     1.5%
# pair = findPair(entries, 2020)          [1721, 299]      2 35.1µs     2.3%
#   findPair(...)
#     ...
#   // end findPair(...)
#
# by self time:
#   name                     calls    self      %  size
#   findPair(...)                1  32.7µs  32.0%     —
```

`--store=<name>` picks the entry point the same way `crust run` does;
`--max-steps N` bounds the recording for a program that loops far more
than a terminal (or a human) wants to read through. Each file's
last-used Run tab settings are remembered in a small JSON file under
your user config directory (`$XDG_CONFIG_HOME/crust/develop_state.json`,
or the platform equivalent) — delete it to forget everything, or an
individual file's entry to forget just that one.

### Documentation site

```
crust bake documentation            # serve on http://localhost:4747, open a browser
crust bake documentation -p 8080    # pick a different port
```

A pizza-themed, browsable reference site — Getting Started, a syntax
cheat-sheet, and full Keywords/Standard Library tables with a search
box on each. The whole site is embedded in the binary
(`internal/docsite`), so this works from any built `crust`, no source
tree required; the two reference tables are read live from the exact
same `internal/lsp` tables `crust lsp` hovers and completes with (kept
honest against `internal/builtins`' real registrations by that
package's own tests), so they can't quietly drift out of date with
what a given build of `crust` actually understands. Ctrl+C stops the
server.

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
outline), completion (keywords, builtins, and every declared name in
the file), and document formatting — all resolved through cRust's real
function-scope nesting (SPEC.md §3), not plain text matching, so two
functions with identically-named parameters correctly resolve to two
different declarations. It speaks plain JSON-RPC 2.0 over stdio, so
any LSP client can launch `crust lsp` as the command; for Neovim
specifically, `vim.lsp.start()` needs no plugin beyond what you likely
already have:

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

-- Format on save, the same way you'd wire up gofmt/rustfmt.
vim.api.nvim_create_autocmd("BufWritePre", {
  pattern = "*.crust",
  callback = function()
    vim.lsp.buf.format({ name = "crust_ls" })
  end,
})
```

Drop that in `lua/config/autocmds.lua` (or wherever your config keeps
general autocommands) and make sure `crust` itself is on `PATH` (see
[With Nix](#with-nix) above). No LSP server registration/Mason install
needed — `crust` *is* the language server.

Prefer a plain CLI over editor integration? `crust fmt <file>` prints
the canonically-formatted source to stdout; `crust fmt -w <file>`
rewrites the file in place (gofmt's convention, not rustfmt's — format
by default is a read, not a write). Both the CLI and the LSP's
formatting request are the same `internal/format` pretty-printer under
the hood, so they always agree.
