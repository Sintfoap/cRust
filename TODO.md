# cRust Roadmap

Milestones for building cRust — a Go-based interpreter with a pizza-jargon
keyword set — in time for Advent of Code 2026 (Dec 1).

## Phase 0 — Project Foundations ✅
- [x] Initialize Go module (`go.mod` — `github.com/Sintfoap/cRust`, Go 1.24)
- [x] `cmd/crust` with a minimal CLI: `--version`, `--help`/`-h`, and
      stubbed `run`/`repl` subcommands that say "not implemented yet"
      rather than doing nothing or panicking
- [x] `.gitignore`, `LICENSE` (MIT)
- [x] GitHub Actions CI: `go build`, `go vet`, `gofmt -l`, `go test ./...`
      on Linux only (see decisions below)
- [x] Smoke tests for the CLI stub (`cmd/crust/main_test.go`) — gives
      Phase 0 something for CI to actually run instead of an empty
      `go test ./...`
- [x] `flake.nix` so `nix run github:Sintfoap/cRust` builds and runs
      the CLI from any Nix-enabled system (NixOS, Nix-on-WSL, etc.) —
      confirmed working end-to-end by a real user on Nix-on-WSL
      (this dev environment still can't reach nixos.org itself to
      install/test Nix directly)
- [x] `devShells.crust` — `nix develop .#crust` (or
      `nix develop github:Sintfoap/cRust#crust`) puts the built `crust`
      CLI on `PATH` for a shell session, no `nix profile add`/global
      `PATH` changes needed. Added after `nix profile add` turned out
      to have a real, non-obvious PATH gotcha in practice — see
      [ARCHITECTURE.md](./docs/ARCHITECTURE.md)'s Phase 0 notes

  → `internal/{token,lexer,ast,parser,object,interpreter,builtins}`
  from ARCHITECTURE.md's package layout aren't created yet — they land
  as stubs-with-real-content when their own phase starts (Phase 2+),
  rather than as empty placeholder packages now.

  → Decisions made explicitly rather than defaulted into: CI targets
  **Linux only** (Go's stdlib is portable and this project has no
  OS-specific code, so a single-platform CI is very unlikely to miss a
  real bug; cross-compiling a one-off binary for another OS later is a
  free `GOOS=... GOARCH=... go build` whenever actually needed) — and
  **no release workflow yet** (build from source via `go build`/`go run`
  until there's something worth distributing).

## Phase 1 — Language Design ✅
- [x] Core types: int, float, string, bool, list, map, set, nil
- [x] Pizza-jargon keyword vocabulary (map every keyword to a pizza term —
      e.g. func/if/else/for/while/return/true/false)
- [x] Syntax style: braces vs. indentation, statement terminators, comments
- [x] Variable model: no `let`/`const` — bare, Python-style assignment
      (`x = 1`), scoped to the nearest enclosing `recipe`
- [x] Full operator list: arithmetic, comparison, logical, assignment
      (incl. compound `+=`/`-=`/etc.), increment/decrement (`++`/`--`),
      ranges (`..`/`.<`), ternary (`(|`/`|)`), Elvis nil-coalescing
      (`?:`), indexing
- [x] Unpacking assignment (`x, y = list`, last target always a List)
- [x] Write `docs/SPEC.md` with grammar (EBNF) and semantics
- [x] Write a "Hello World" and six AoC-shaped/feature sample programs
      by hand, in the target syntax, to sanity-check ergonomics before
      coding anything
- [x] Exhaustive example coverage audit: every keyword, operator, and
      builtin in `SPEC.md` used in at least one `examples/*.crust`
      file — `the_works.crust` (deliberate checklist covering what the
      AoC-shaped examples didn't naturally exercise: `combo`/`burnt`/
      `flip`, `with`/`or`/`hold`, every compound assignment, `!=`/`<=`,
      division, Float/Boolean literals, non-empty Map/Set literals,
      string escapes, string indexing, and the `gather`/`scrape`/
      `topped`/`strip` builtins) and `closures.crust` (the accumulator
      and closure-mutates-captured-variable patterns `SPEC.md` §3
      describes in prose, plus a nested-closure counter as a harder
      case for whenever Phase 4 needs a regression test)

  → See [docs/SPEC.md](./docs/SPEC.md) for the full keyword table, grammar,
  operator list, and truthiness/coercion rules, and [`examples/`](./examples)
  for the sample programs. Open question, deliberately deferred: whether
  for-each `knead` should support unpacking (e.g. `k, v` pairs over a
  Map) — see SPEC.md §3.1.

## Phase 2 — Lexer ✅
- [x] Token type definitions (`internal/token`)
- [x] Tokenizer implementation (`internal/lexer`) — numbers (incl. the
      float-vs-range `.` disambiguation), strings (incl. escapes),
      identifiers, keywords, every operator including `(|`/`|)`/`?:`,
      `//` comments
- [x] Line/column tracking for error messages
- [x] Lexer unit tests — table-driven, 99%+ coverage, plus lexing
      every real `.crust` file under `examples/` end-to-end

  → Two things formalized in `SPEC.md` §2.1 while building this, since
  they were already in use everywhere but never specified: `//` line
  comments and the string escape set (`\" \\ \n \t \r`). Also decided
  (not pre-specified): consecutive blank lines/comments collapse into
  one `NEWLINE` token — see ARCHITECTURE.md's Phase 2 notes for why,
  and the one thing this leaves for Phase 3 (tolerating a leading
  `NEWLINE`).

  → **Fixed post-Phase-3**: a bare `\n` had no line-continuation
  handling at all — any multi-line function call, list literal, or
  grouped expression was a guaranteed parse error, since every physical
  newline became a real terminator regardless of context. Caught by
  running the new `crust parse` (below) against every `examples/*.crust`
  file, where `ternary_elvis.crust`'s multi-line chained ternary failed
  to parse. Fixed by tracking a stack of unclosed `(`/`[`/`{` in the
  lexer and swallowing `\n` while the innermost one is `(` or `[` — see
  `SPEC.md` §8's note and ARCHITECTURE.md's Phase 2 section for why it
  has to be a stack (not a counter) and why `{` is excluded. Purely
  additive/backward-compatible; the one example that needed a multi-line
  layout got reformatted to wrap in explicit parens rather than the
  language growing bare (bracket-free) continuation.

## Phase 3 — Parser ✅
- [x] AST node definitions (`internal/ast`) — 100% coverage, table-driven
      `String()`/`TokenLiteral()` tests per node type
- [x] Recursive-descent / Pratt parser (operator precedence) (`internal/parser`)
- [x] Parse errors with line/col + helpful messages (collected, not
      abort-on-first; `synchronize()` recovers to the next statement
      boundary so one bad statement doesn't cascade)
- [x] Parser unit tests — table-driven, 93%+ coverage, incl. operator
      precedence/associativity round-trips and a broad malformed-input
      error table

  → See [ARCHITECTURE.md](./docs/ARCHITECTURE.md#phase-3--parser-internalparser-)
  for the precedence-table design, the `ELVIS - 1` right-associativity
  derivation, the range anti-chaining check, and the full AST node
  list (including `BakeStatement`, added during implementation — it
  wasn't in the original node list).

  → **Added post-Phase-3**: `Tuple`, a real value type (`SPEC.md`
  §2.3) — `(a, b, ...)`, fixed-size, immutable, and (unlike `List`)
  hashable, so it works as a `Map` key or `Set` element
  (grid-coordinate dedup being the main reason to want one). Unpacking
  a `Tuple` requires exact arity and gives every target, including the
  last, its own bare value, instead of `List`-unpack's "last target
  always gets a List of everything left over" rule — which rule
  applies is decided by the right-hand side's *runtime* type, so it
  works through a ternary/function-call/anything, not just a literal
  directly on the assignment's right. (First shipped as narrower
  unpack-only sugar with no real value type behind it; promoted to a
  real type once that turned out not to survive being produced by a
  ternary branch — see ARCHITECTURE.md's Phase 3 notes for why.)

## Phase 4 — Interpreter / Evaluator ✅
- [x] Tree-walking evaluator (`internal/interpreter`, one recursive
      `Eval` method dispatching on AST node type)
- [x] Environment & scoping (function scope only — `recipe` calls create
      a scope, `order`/`knead`/`bake` blocks don't; walk-and-mutate
      assignment, see SPEC.md §3), closures
- [x] Control flow: conditionals, `knead` (counted + for-each), `bake`,
      break/continue — `burnt` skips a counted loop's `Post` clause
      entirely (C-family `break`), `flip` still runs it (C-family
      `continue`)
- [x] Functions: declarations, calls, recursion, closures
- [x] Composite data: lists, maps, sets — indexing, index assignment,
      map-missing-key-reads-as-nobox, Map's String/Integer-only key
      restriction, Set's not-indexable restriction, all reachable from
      source now
- [x] Runtime error handling — `object.Error` propagates through `Eval`
      as an ordinary value (not Go panic/recover) with source
      line/col; `cmd/crust`'s `runFile` also wraps a single `recover()`
      as a last-resort net for an actual interpreter bug, not the
      primary mechanism
- [x] Entry-point resolution (`store`/`store_<name>` recipes, `crust run
      <file> --store=<name>`) — SPEC.md §9, implemented in
      `cmd/crust/run.go`

  → `internal/object` gained `Function`, `Error`, `ReturnValue`, and
  the `BREAK`/`CONTINUE` singletons to finish what Phase 3-era
  groundwork had deliberately left out. Several truthiness/coercion
  rules got pinned down in SPEC.md §6 while implementing this (int/
  Float as one type for `==` too, not just ordering; division by zero
  always an Error; no negative indexing; missing Map key reads as
  `nobox`; `with`/`or` always produce a strict Boolean) — see
  ARCHITECTURE.md's Phase 4 section for the reasoning behind each.
  93%+ test coverage in `internal/interpreter`, 100% in
  `internal/object`/`internal/builtins`.

## Phase 5 — Standard Library (AoC-focused) 🚧
- [x] Input: `unbox()`/`unbox(path)` (stdin or a file, whole contents
      as a String) + `lines(s)` (split into a List of lines)
- [x] `trim` (leading/trailing whitespace — kept distinct from `strip`,
      which is Set difference, to avoid a name collision)
- [x] `split` (`split(s)` whitespace-collapsing, `split(s, delim)`
      literal delimiter — CSV-style, preserves empty entries)
- [x] `join` (List of Strings + separator → String, `split`'s counterpart)
- [ ] Strings: contains, replace
- [x] `chars` (string → List of one-character strings)
- [x] `ints` (string of digits → List of single-digit Integers; also
      accepts a List of numeric Strings, parsing each as a full
      Integer — `ints(split(line))`)
- [x] Type conversion: `str`/`int`/`float`/`bool` — explicit only, no
      implicit coercion (SPEC.md §6 already rules that out for
      operators: `+` between a String and a number stays a type error,
      not a silent stringify/parse)
- [x] `min`/`max` (2+ direct args or a single List/Tuple; numbers freely
      mixed Integer/Float, or Strings — same ordering as `<`; returns
      the winning element itself, unconverted)
- [x] `combos(list, n)` — every n-element combination (not permutation)
      of list's elements, each as a Tuple; generalizes to any n instead
      of hardcoding pairs/triples
- [x] Grid utilities: originally shipped as plain List-of-List (no
      dedicated type), then promoted to a real `object.Grid`
      (SPEC.md §2.4) once `setAt` needed to auto-expand out-of-range
      writes instead of erroring — a plain List has no room to
      remember a shifted origin between calls, which growing into
      *negative* coordinates needs. `grid(s)` (parse text into a Grid
      at offset (0,0)), `newGrid()` (empty Grid, build one up entirely
      via `setAt`), `at(g, pos)` (bounds-checked read, nobox on
      out-of-range; still accepts a plain List of rows too, for
      backward compatibility), `setAt(g, pos, value)` (Grid-only now;
      grows `g` — any direction, including negative — to include `pos`
      instead of ever erroring), `gridBounds(g)` (`(minRow, minCol,
      maxRow, maxCol)` Tuple, or nobox if empty — the only way to learn
      a Grid's extent, since it's deliberately not directly
      indexable/iterable), `neighbors4(pos)`/`neighbors8(pos)`
      (orthogonal / +diagonal offsets, no bounds checking, unchanged).
      Coordinates are `(row, col)` Tuples throughout, same as before
- [ ] Math: abs, pow, gcd, lcm, sqrt (standard names, not themed)
- [x] `idiv` (integer division — SPEC.md §6, `/` always produces a Float)
- [x] `push` (append to a List in place) and `+` extended to List/List
      and Tuple/Tuple concatenation (always a new value, never
      mutating either operand — `push` is the in-place counterpart)
- [x] `map(iterable, fn)` — applies fn (a recipe or builtin) to every
      element of a List/Tuple, collecting the results. Composition of
      multiple steps is via a lambda (`map(xs, recipe(x) { serve g(f(x)) })`),
      not a List of functions
- [ ] Collections: sort, filter/reduce (or equivalent loop sugar)
- [x] `slices` (length of a String/List/Map/Set)
- [x] Sets: `gather`, `sprinkle`, `scrape`, `topped`, `combine`, `shared`, `strip`
- [x] Nil-handling: `sauce` (fallback-if-nobox)
- [x] Output: `deliver` *(no formatting verbs yet — space-joined
      `Inspect()` output plus a newline)*

  → Names for the items above are already locked in — see
  [docs/SPEC.md §7](./docs/SPEC.md#7-standard-library-builtins). Most
  of what's checked off landed ahead of schedule alongside Phase 4 —
  see ARCHITECTURE.md's Phase 5 section for exactly what's built vs.
  still pending (`internal/builtins`, not yet the general `strings`/
  `math`/`sort` helpers).

## Phase 6 — Tooling
- [x] CLI: `crust run <file> [--store=<name>]` (Phase 4's interpreter
      makes this real; the bare-file shorthand and `--store` entry-point
      selection both work — see SPEC.md §9)
- [ ] REPL mode — natural next step now that `Eval` exists, not yet built
- [x] Clear, pizza-themed error messages *(started ahead of schedule)*
- [x] Colorized `--help` banner (the pizza, customizable via
      `--toppings`/`--no-banner`/`--no-color`) — see
      [ARCHITECTURE.md](./docs/ARCHITECTURE.md) Phase 0/6 notes
- [x] `crust tokens <file>` — debug command that prints the lexer's
      token stream *(started ahead of schedule; the first real use of
      Phase 2's lexer from the CLI, since `run` doesn't exist yet)*
- [x] `crust parse <file>` — debug command that prints the parsed AST
      *(started ahead of schedule; running it against every
      `examples/*.crust` file caught a real lexer bug — see the Phase 2
      line-continuation entry below)*
- [x] (Stretch) editor syntax highlighting — Vim/Neovim (`editors/vim`,
      traditional regex syntax file, works unmodified in both) and
      VSCode (`editors/vscode`, TextMate grammar). Both verified
      against the real tokenizers (a live Vim instance;
      `vscode-textmate`/`vscode-oniguruma`, the same engine VSCode
      itself uses), not just read off the docs — see
      [ARCHITECTURE.md](./docs/ARCHITECTURE.md) Phase 6 notes for two
      real bugs that surfaced only under that testing.
- [x] (Stretch) Tree-sitter grammar for Neovim — `editors/tree-sitter-crust`,
      a real CFG (`grammar.js`) plus a `queries/highlights.scm` highlight
      query, more accurate than the regex-based Vim syntax file above
      (parser-driven, not pattern-matched). 23/23 corpus tests pass,
      every real `examples/*.crust` file parses with zero
      `ERROR`/`MISSING` nodes, and the generated parser was actually
      compiled to a `.so` and confirmed to export the symbol Neovim's
      loader looks for — see
      [ARCHITECTURE.md](./docs/ARCHITECTURE.md) Phase 6 notes for what
      was and wasn't possible to verify without a real Neovim binary in
      the build environment, and for a grammar-vs-grammar priority-
      resolution surprise (opposite direction from the Vim syntax file's
      own bug above) that only showed up once the highlight query was
      tested against the real tokenizer rather than read off the query
      file.
- [x] (Stretch) Language server (`internal/lsp`, `crust lsp`) — hover
      (keyword/builtin/literal-type docs), diagnostics
      (`textDocument/publishDiagnostics` from lexer ILLEGAL tokens and
      `internal/parser`'s structured `ParseError`/`ParseErrors()`),
      and real scope-aware `textDocument/definition` (aliased for
      `typeDefinition` too — cRust has no separate type-declaration
      site to distinguish it from a plain declaration)/`references`/
      `rename`/`documentSymbol`/`completion` (`internal/lsp/symbols.go`
      — a `fileIndex` built by walking the parsed AST once per
      request, resolving names through cRust's actual function-scope
      nesting, not a same-name text search: two functions with
      identically-named parameters correctly resolve to two different
      declarations). Speaks JSON-RPC 2.0 over stdio, hand-rolled
      against the spec rather than built on a third-party LSP library
      (keeps the zero-Go-dependency policy — see ARCHITECTURE.md's Nix
      section — intact). Position-encoding negotiation
      (`utf-8`/`utf-16`/`utf-32`, per
      `capabilities.general.positionEncodings`) implemented and tested
      rather than hardcoding one, since internal/lexer's rune-indexed
      columns don't line up with any encoding but `utf-32` by default.
      Verified against a real subprocess (not just Go unit tests) —
      see ARCHITECTURE.md's Phase 6 notes, including the real-user
      report (Neovim's generic `<leader>D`/`typeDefinition` keymap
      hitting an unimplemented method) that prompted the definition/
      references/rename/documentSymbol/completion work in the first
      place.
- [ ] (Stretch) Further LSP: incremental (as opposed to full) document
      sync, code actions — `internal/lsp` exists now (above) as a base
      to extend rather than something to build from scratch
- [x] (Stretch) debugger: `internal/trace` (a `Tracer` hook —
      `Step`/`PushFrame`/`PopFrame` — checked once per statement and
      once per call/loop-lap frame in `internal/interpreter`, nil when
      untraced so an ordinary `crust run` pays nothing extra; verified
      against a real benchmark, not just reasoned about) +
      `internal/debugger` (a `Recorder` building a bounded step/frame
      tree — recipe calls and knead/bake loop laps are frames, repeated
      loop laps fold after 3 — plus a `Timing` pass giving each
      function/loop its self/total time and self *size*, keyed by
      *family* so every call to one recipe, at any recursion depth, and
      every lap of one loop, lands in one KPI bucket rather than one
      per call site) + `crust debug <file> [--store=<name>] [--plain]
      [--max-steps N]`: a plain-text mode (also what makes the whole
      thing unit-testable and CI-scriptable) and — on a real terminal —
      an interactive bubbletea TUI with two tabs, switched with
      tab/←→: **KPIs** (a pizza-toned ASCII pie chart each for
      self-time and self-memory per function/loop, ranked, plus overall
      step count/total time/slowest single statement) and **Stepper**
      (the recorded tree, ↑↓ to move, enter to open/close a folded run
      of loop laps). Concept and much of the tree-building shape
      borrowed, at the user's explicit request, from a similar tool in
      another interpreter project, adapted throughout for cRust being a
      general imperative language rather than a value pipeline — see
      ARCHITECTURE.md's debugger section for exactly what carried over
      and what had to change, including three real bugs (one a genuine
      performance regression, caught by benchmarking against a
      pre-tracing git-worktree baseline rather than just reasoning about
      the code) found by actually running the finished command rather
      than trusting the unit tests alone.
- [ ] (Stretch) debugger follow-ons: pie chart radius that adapts to
      the terminal's actual size (fixed at 7 today, which a very small
      terminal could clip) and a search/filter over the Stepper tree

## Phase 7 — Testing & Quality
- [ ] Unit tests across lexer/parser/interpreter
- [ ] Integration tests: sample `.crust` programs with expected output
- [ ] Benchmark against a real prior-year AoC puzzle for performance sanity

## Phase 8 — AoC 2026 Ready
- [ ] Confirm day-1 essentials all work end-to-end: file I/O, arithmetic,
      strings, loops, lists, maps
- [ ] Per-day solution template (`examples/day01/`, etc.)
- [ ] Keyword cheat-sheet doc for quick reference during the event
- [ ] Dry run: solve an old AoC day 1-5 in cRust before Dec 1, 2026

## Stretch Goals
- [ ] Module/import system
- [ ] Bytecode VM instead of tree-walking (perf)
- [ ] Web playground
- [ ] Richer stack traces
