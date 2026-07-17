# cRust Roadmap

Milestones for building cRust — a Go-based interpreter with a pizza-jargon
keyword set — in time for Advent of Code 2026 (Dec 1).

## Phase 0 — Project Foundations
- [ ] Initialize Go module (`go.mod`)
- [ ] Repo layout: `cmd/crust`, `internal/lexer`, `internal/parser`,
      `internal/interpreter`, `examples/`, `docs/`
- [ ] `.gitignore`, `LICENSE`
- [ ] GitHub Actions CI: `go build`, `go vet`, `go test ./...`

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
- [x] Write a "Hello World" and four AoC-shaped sample programs by hand,
      in the target syntax, to sanity-check ergonomics before coding
      anything

  → See [docs/SPEC.md](./docs/SPEC.md) for the full keyword table, grammar,
  operator list, and truthiness/coercion rules, and [`examples/`](./examples)
  for the sample programs. Open question, deliberately deferred: whether
  for-each `knead` should support unpacking (e.g. `k, v` pairs over a
  Map) — see SPEC.md §3.1.

## Phase 2 — Lexer
- [ ] Token type definitions
- [ ] Tokenizer implementation (numbers, strings, identifiers, keywords,
      operators, comments)
- [ ] Line/column tracking for error messages
- [ ] Lexer unit tests

## Phase 3 — Parser
- [ ] AST node definitions
- [ ] Recursive-descent / Pratt parser (operator precedence)
- [ ] Parse errors with line/col + helpful messages
- [ ] Parser unit tests

## Phase 4 — Interpreter / Evaluator
- [ ] Tree-walking evaluator
- [ ] Environment & scoping (function scope only — `recipe` calls create
      a scope, `order`/`knead`/`bake` blocks don't; walk-and-mutate
      assignment, see SPEC.md §3), closures
- [ ] Control flow: conditionals, `knead` (counted + for-each), `bake`,
      break/continue
- [ ] Functions: declarations, calls, recursion, closures
- [ ] Composite data: lists, maps, sets, indexing, slicing
- [ ] Runtime error handling (panics with source location)

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
- [ ] CLI: `crust run <file>`
- [ ] REPL mode
- [ ] Clear, pizza-themed error messages
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
