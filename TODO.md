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
      **not yet verified end-to-end** (this dev environment can't reach
      nixos.org to install Nix itself); run `nix flake check` /
      `nix run .` locally before relying on it

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

## Phase 4 — Interpreter / Evaluator
- [ ] Tree-walking evaluator
- [x] Environment & scoping (function scope only — `recipe` calls create
      a scope, `order`/`knead`/`bake` blocks don't; walk-and-mutate
      assignment, see SPEC.md §3), closures *(`object.Environment` —
      the closures piece still needs `Function`/`Eval` to exist before
      it means anything, but the scoping rule itself is implemented
      and tested)*
- [ ] Control flow: conditionals, `knead` (counted + for-each), `bake`,
      break/continue
- [ ] Functions: declarations, calls, recursion, closures
- [x] Composite data: lists, maps, sets *(`object.List`/`Map`/`Set` —
      indexing/slicing still need `Eval` to exist to be reachable from
      source, but the underlying types, HashKey scheme, and
      mutate-in-place semantics are implemented and tested)*
- [ ] Runtime error handling (panics with source location)

  → `internal/object` was started ahead of schedule — see
  [ARCHITECTURE.md §5](./docs/ARCHITECTURE.md#5-performance-strategy)
  for the performance decisions baked into it (singleton
  Boolean/Null, small-integer cache, reference-type collections,
  precomputed HashKeys) and the Phase 4 section for what's built vs.
  still pending (`Function`, `Error`/control-flow signal types, and
  `Eval` itself all still need Phase 3's `ast` package and/or a
  decided error-value convention first).

## Phase 5 — Standard Library (AoC-focused)
- [ ] Input: read file / stdin, split into lines
- [ ] Strings: split, join, trim, contains, replace, parse-to-number, `chars`
- [ ] Math: abs, min, max, pow, gcd, lcm, sqrt (standard names, not themed)
- [ ] Collections: sort, map/filter/reduce (or equivalent loop sugar),
      `slices` (length)
- [ ] Sets: `gather`, `sprinkle`, `scrape`, `topped`, `combine`, `shared`, `strip`
- [ ] Nil-handling: `sauce` (fallback-if-nobox)
- [ ] Output: `deliver` with formatting

  → Names for the items above are already locked in — see
  [docs/SPEC.md §7](./docs/SPEC.md#7-standard-library-builtins). This
  phase is about implementing them, not naming them.

## Phase 6 — Tooling
- [ ] CLI: `crust run <file>` (still stubbed — needs Phase 4's interpreter)
- [ ] REPL mode
- [x] Clear, pizza-themed error messages *(started ahead of schedule)*
- [x] Colorized `--help` banner (the pizza, customizable via
      `--toppings`/`--no-banner`/`--no-color`) — see
      [ARCHITECTURE.md](./docs/ARCHITECTURE.md) Phase 0/6 notes
- [x] `crust tokens <file>` — debug command that prints the lexer's
      token stream *(started ahead of schedule; the first real use of
      Phase 2's lexer from the CLI, since `run` doesn't exist yet)*
- [ ] (Stretch) editor syntax highlighting (TextMate grammar / tree-sitter)

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
