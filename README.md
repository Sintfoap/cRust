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
`crust bake playground` serves an in-browser sandbox that runs cRust
client-side via WebAssembly, nothing sent anywhere — see
[Web playground](#web-playground) below. `crust repl` starts an
interactive session — see
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

A runtime error (`crust run`, `crust develop`'s Run tab, and `crust
repl` alike) prints the failing line first, same as always, plus —
once it unwound through more than one recipe call — the call chain
underneath it, innermost call first, so "which recipe, called from
where" is visible without reaching for the debugger:

```
crust run: day06.crust:14:5: division by zero
    in divide(...), called from 9:12
    in process(...), called from 3:5
    in store(...)
```

A single level of call wrapping (an error happening directly inside
`store()`'s own body, nothing nested beneath it) prints exactly as it
always has — just the one line — since a one-entry chain wouldn't add
anything the failing line doesn't already say.

Starting a new day: `crust new <day>` stamps out `dayNN.crust` from
the same `store_part1`/`store_part2` skeleton
[`examples/dayNN_template.crust`](./examples/dayNN_template.crust)
documents for manual copying — reading its input via `unbox()`, with
the day-to-day workflow (`crust fetch`, `crust develop`/`crust run`)
in its own header comment — with the day number already filled in
rather than left as a `dayNN` placeholder to edit by hand. Refuses to
overwrite an existing file unless you pass `--force`, the same
protection `crust fetch` gives an existing input file:

```
crust new 6
# Created day06.crust. `crust fetch 6` to grab its input, `crust develop day06.crust` to start writing.
crust fetch 6
crust develop day06.crust
```

The new file is immediately visible in `crust develop`'s own Files/Nav
tab, described below — "every other `.crust` file alongside the one
currently open" already includes it, no extra step needed.

Sharing logic across files (a Grid helper, a parsing routine) instead
of copy-pasting it into every day's file: `delivery "path.crust"`
brings that file's recipes and top-level variables directly into
scope, resolved relative to whichever file is actually running —
[`examples/module_utils.crust`](./examples/module_utils.crust) /
[`module_demo.crust`](./examples/module_demo.crust) are a real,
runnable pair showing the shape:

```
// grid_utils.crust
recipe manhattan(a, b) {
    serve abs(a[0] - b[0]) + abs(a[1] - b[1])
}
```
```
// day15.crust
delivery "grid_utils.crust"

deliver(manhattan((0, 0), (3, 4)))   // 7
```

No namespacing, no exports list — a delivered file's top level runs
directly into the importing file's own scope, the same as pasting its
text in at that line, and delivery is fully transitive (deliver a file
that itself delivers another, and its names come along too). Delivered
exactly once even if reached from more than one place, and safe
against a circular `delivery` — see [SPEC.md
§10](./docs/SPEC.md#10-modules) for the full semantics.

Fetching that day's puzzle input and timing how long it takes is
optional, and entirely inert until you opt in with `crust login`:
paste the `session` cookie from a browser logged into
[adventofcode.com](https://adventofcode.com) (dev tools ->
Application/Storage -> Cookies), and it's saved to a private
(`0600`) file under your user config directory — or set `$AOC_SESSION`
instead, if you'd rather not have crust write one. Once that's in
place, `crust fetch <day>` downloads that day's input as
`dayNN_input.txt` and starts a timer for it; `crust done <day>` stops
the timer and prints the total. `crust develop dayNN.crust` does the
fetch (and starts the timer) automatically the moment it opens a
`dayNN.crust` file whose input isn't already sitting there — and shows
a live-ticking stopwatch in its own header for as long as that day's
timer is running:

```
crust login
# Paste your adventofcode.com session cookie (from a logged-in browser's
# dev tools -> Application/Storage -> Cookies -> "session").
# Note: this will be visible as you type/paste it — there's no input masking.
# session: ...

crust fetch 6
# Saved day 6, 2026 input to day06_input.txt (1234 bytes). Timer started — `crust done 6` when you've got the star.

crust done 6
# Day 6, 2026: 14m32s
```

`crust fetch`/`crust develop`'s auto-fetch both refuse to overwrite an
existing `dayNN_input.txt` (pass `--force` to `crust fetch` if you
really want to redownload it) — a hand-edited or already-fetched input
file is never silently replaced. `--year <n>` targets a different AoC
event (default: 2026) on `crust fetch`/`crust done`/`crust submit`.

Working a past year (not just 2026) through `crust develop` itself:
pass `--year` there too, and it sticks — remembered per file, the
same way the entry point/input path already are, so it only needs
saying once:

```
crust new 7 --year 2020        # stamps day07.crust's header with 2020 too
crust develop day07.crust      # already knows 2020 from here on, no --year needed
# day07.crust — 0 steps · 0s total  ⏱ day 7, 2020: 11s (running)
```

`crust develop day07.crust --year <other>` on a later invocation
overrides and re-remembers the new year, the same explicit-flag-wins
precedence `--store` already follows. Switching to a file that's never
had `--year` set falls back to 2026, same as always — this only
matters for files actually set up for a different event.

Same session cookie, one step further: `crust submit <day> <answer>
--part=<1|2>` posts an answer to adventofcode.com and prints its
verdict, exiting 0 only on a correct answer — so it composes with `&&`
in a shell one-liner. Leave the answer off and it reads the first line
of stdin instead, which is what makes piping a run's own output
straight in worthwhile:

```
crust run day06.crust --store=part1 < day06_input.txt | crust submit 6 --part=1
# Correct! Day 6 part 1, 2026: That's the right answer! [Return to Day 6]
```

A wrong answer, AoC's own rate-limit cooldown, or "you already have
that star" all print AoC's own response text and exit 1 — `crust
submit` never touches the timer `crust fetch` starts/`crust done`
stops, since a correct part 1 isn't "done" for the day and only the
solver knows when they actually are.

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

On a real terminal this opens an interactive eight-tab TUI: **Time**
and **Memory** (one pie chart each, so both get the full window),
**Stepper** (the tree, each row showing its source line number; `/`
searches by the visible label/output text with `n`/`N` repeating the
same search forward/backward, and `f`/`F` jump straight to the next/
previous failed step — all three auto-expand any folded loop lap
standing in the way, so a match or failure buried inside a collapsed
"N iterations" row is never missed; `v` toggles a watch panel showing
every variable visible at the selected step — its own scope plus every
enclosing one, shadowed the same way a real lookup resolves — read from
a snapshot taken at that exact moment, not live state, so browsing an
earlier step after the run finished still shows what that variable was
*then*; `o` toggles a panel showing that step's full, untruncated
result — the table's own "out" column caps a long value with an
ellipsis to keep the row a fixed width), **Editor** (opens `nvim` on the file being
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
selector row listing them and ←→ picks one before you hit enter;
output taller than the panel scrolls with pgup/pgdn, with "N more
line(s) above/below" hints and the scroll position resetting to the
top on every fresh run),
**Bench** (type a run count and press enter to run the file that many
times, back to back, with whichever entry point and input file the
Run tab currently has selected — plain, untraced runs straight through
the same `runFile` `crust run` itself uses, so tracing overhead never
skews the numbers — then charts wall-clock time and memory allocated
across those runs as two small ASCII line charts, one per metric,
each on its own scale; five lines per chart — the raw per-run values,
average, median, max, min — toggle independently with `r`/`a`/`m`/`x`/
`n` so you can isolate exactly what you're comparing, shared between
both charts since "average" means the same thing on either one; press
`i` to inspect an individual run instead of only the aggregates — `h`/
`l` steps to the previous/next run, `g`/`G` jumps to the first/last,
and a caret under both charts plus a readout line show that exact
run's own duration/memory (and whether it failed), not blended into
any average; `e` exports the current batch to a CSV file next to the
debugged file — one row per run, its raw duration in nanoseconds and
bytes allocated, for spreadsheet analysis or comparing across days;
`b` saves the current batch's average runtime/memory as a named
baseline, remembered per file — every batch after that shows how far
its own average has moved from it, as a signed percentage, until `b`
is pressed again to replace it; `c` captures the current batch (which
entry point, how many runs, its average runtime/memory) for a
two-entry-point diff — switch the Run tab's entry-point selector to
whatever you want to compare against (a brute-force `store_part2`
against an optimized rewrite, or `part1` against `part2`) and run a
fresh batch on the Bench tab, and it shows both sides labeled side by
side with the same signed-percentage diff the baseline uses.
Session-only, unlike the baseline — it's for A/B-ing two things both
still open right now, not tracking drift across sessions), and
**Files** (every other `.crust` file alongside the one currently
open — the natural "AoC folder full of day01.crust..day25.crust"
layout — ↑↓ to move, enter to switch straight to one without leaving
`crust develop` and relaunching it on a different path; each file
keeps its own remembered store/input/run-all/bench-count settings, the
same as reopening it fresh from the command line would; `n` starts a
new file right there — type a name, enter creates it (`.crust`
appended if you leave it off) and switches straight to it; `Ctrl+N`
does the same for a fresh AoC day specifically — type a day number
and, optionally, a year (`7` or `7 2020`), enter creates
`day07.crust` from the same starter template `crust new` writes and
switches straight to it, without leaving `crust develop` to run a
separate command; a file at
least one other file's own top-level `delivery "..."` targets shows
"(used by N other files)" next to its name — the easy way to spot
which shared helper is safe to edit without checking every day by
hand, or which one is still worth keeping around), and
**Live** (a genuinely *live* debugger, unlike the Stepper's
after-the-fact recording: `b` toggles a breakpoint on the source line
under the cursor, remembered per file the same as Bench's baseline;
`r` runs (or restarts) the file with the Run tab's current entry point
and input file; the very first statement always pauses, so you're
never left wondering whether anything happened yet. Once paused, `s`
steps to the next statement regardless of breakpoints, `c` continues
until the next breakpoint (or the program ends), and `x` stops the run
early — even mid-flight between breakpoints, not only while actually
paused. A watch panel below the source shows every variable in scope
at the current pause, and an output panel shows the program's own
`deliver()` output as it happens, not only at the end. `w` toggles
focus onto the watch panel when there are more variables than fit —
↑↓ then scroll through them instead of moving the source cursor,
with "N more above/below" hints and a "(1-6 of 13)"-style position
readout in the panel's title so browsing a large scope never leaves
you guessing how much is left; `w` again returns focus to the source).
Ctrl+R toggles running *every* entry point in sequence instead of just
the selected one — each against the same input file, one full run
apiece, output concatenated under a `=== part1 ===`-style heading per
store and their timing/debug info merged into one combined Time/
Memory/Stepper recording — handy for checking part 1 and part 2 (or
however many `store_<name>`s a file has) together without switching
the selector and re-running by hand each time.
The Run tab also has manual triggers for the AoC workflow itself,
never bare letters (the input-file field needs every printable key for
a real path), so they're all `Ctrl+<letter>`: `Ctrl+F` fetches this
day's input on demand — a retry if auto-fetch failed, or the first
fetch for a file `crust develop` created before a session was saved —
and either downloads it or, if it's already on disk, just adopts it,
either way wiring the result straight into the input-file field above
so there's nothing left to type by hand; `Ctrl+S` submits the last
run's own final output line as this file's answer (day/year from the
filename, part from whichever entry point is selected) and shows AoC's
verdict right there on the Run tab, refusing up front if nothing's
been run yet or the last run failed; `Ctrl+L` suspends `crust develop`
the same way the Editor tab hands off to nvim and runs `crust login`
in its place, so a session can be started (or replaced) without ever
leaving the debugger — once it exits, the Run tab reports whether a
session actually got saved.
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
than a terminal (or a human) wants to read through; `--year <n>`
targets a past AoC event instead of 2026 (see "Starting a new day"
above), and sticks for this file once given. Each file's last-used
Run tab settings — store, input path, and year — are remembered in a
small JSON file under your user config directory
(`$XDG_CONFIG_HOME/crust/develop_state.json`, or the platform
equivalent) — delete it to forget everything, or an individual file's
entry to forget just that one.

If `crust login` has a session saved, opening `dayNN.crust` here also
fetches its input (unless `dayNN_input.txt` already exists) and starts
its timer — see "Starting a new day" above — using whichever year this
file resolves to (2026, unless `--year` set something else for it),
and the header shows a live-ticking `⏱ day N, YYYY: 3m12s (running)`
for as long as that timer's going, whichever tab you're on. Either
way — auto-fetch at startup or a manual `Ctrl+F` later — a successful
fetch always overwrites the Run tab's input-file field with the
downloaded path, the same as typing it in and pressing enter would, so
the very next run already reads the real puzzle input.

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

### Web playground

```
crust bake playground            # serve on http://localhost:4748, open a browser
crust bake playground -p 8080    # pick a different port
```

An in-browser cRust sandbox — an editor pane, a stdin box, a `--store`
field, and a Run button (or Ctrl/Cmd+Enter). The language itself runs
entirely client-side via WebAssembly (`cmd/wasm`, built to
`crust.wasm` by `scripts/build-wasm.sh` and embedded into the binary
alongside `index.html`/`wasm_exec.js`), so nothing typed into it is
ever sent anywhere, and — like the documentation site — this works
from any built `crust` with no source tree or Go toolchain present.
Both the CLI's `run` subcommand and the playground call the exact same
`internal/runner.Run` — lex, parse, Eval, resolve-and-call a
`store`/`store_<name>` entry point — so the playground can't drift
from what `crust run` actually does. One caveat inherent to running
synchronously in the browser's main thread: a program with an infinite
loop hangs the tab until reloaded, same as pasting one into any other
in-browser code sandbox. Ctrl+C stops the server.

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
the file), document formatting, and a "Format document" code action —
all resolved through cRust's real function-scope nesting (SPEC.md §3),
not plain text matching, so two functions with identically-named
parameters correctly resolve to two different declarations. Document
sync is incremental (`TextDocumentSyncKind.Incremental`): an editing
client sends just the changed range on every keystroke rather than the
whole file, cheaper over the wire on a long file. It speaks plain
JSON-RPC 2.0 over stdio, so any LSP client can launch `crust lsp` as
the command; for Neovim specifically, `vim.lsp.start()` needs no
plugin beyond what you likely already have:

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
