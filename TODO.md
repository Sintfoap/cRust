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
- [x] Core types: int, float, string, bool, list, map, nil
- [x] Pizza-jargon keyword vocabulary (map every keyword to a pizza term —
      e.g. declare/func/if/else/for/while/return/print/true/false)
- [x] Syntax style: braces vs. indentation, statement terminators, comments
- [x] Write `docs/SPEC.md` with grammar (EBNF) and semantics
- [x] Write a "Hello World" and one AoC-shaped sample program by hand, in
      the target syntax, to sanity-check ergonomics before coding anything

  → See [docs/SPEC.md](./docs/SPEC.md) for the full keyword table, grammar,
  and truthiness/coercion rules, and [`examples/`](./examples) for the
  sample programs.

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
- [ ] Environment & scoping (block scope, closures)
- [ ] Control flow: conditionals, loops, break/continue
- [ ] Functions: declarations, calls, recursion, closures
- [ ] Composite data: lists, maps, indexing, slicing
- [ ] Runtime error handling (panics with source location)

## Phase 5 — Standard Library (AoC-focused)
- [ ] Input: read file / stdin, split into lines
- [ ] Strings: split, join, trim, contains, replace, parse-to-number
- [ ] Math: abs, min, max, pow, gcd, lcm, sqrt
- [ ] Collections: sort, map/filter/reduce (or equivalent loop sugar), len
- [ ] Output: print/println with formatting

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
