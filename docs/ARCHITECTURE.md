# cRust Architecture & Technical Design

This document expands each phase in [TODO.md](../TODO.md) into concrete
technical decisions: what gets built, which Go types and packages are
involved, and why. It's the reference to work from once implementation
starts.

## 1. Pipeline

cRust is a **tree-walking interpreter**. Source text flows through four
stages, each owned by its own package:

```
 .crust source
      │
      ▼
  ┌─────────┐   tokens    ┌────────┐    AST     ┌─────────────┐   object.Object
  │  Lexer  │ ──────────▶ │ Parser │ ─────────▶ │ Interpreter │ ─────────────▶
  └─────────┘             └────────┘            └─────────────┘   (stdout / value)
   internal/lexer         internal/parser        internal/interpreter
   internal/token         internal/ast            internal/object
                                                    internal/builtins
```

No bytecode, no compilation step: the interpreter recursively evaluates
the AST directly against a scoped environment. This is the same shape as
Thorsten Ball's *Writing An Interpreter In Go* (Monkey) — a well-trodden,
easy-to-debug design that's more than fast enough for AoC-sized inputs. A
bytecode VM is listed as a stretch goal if profiling ever shows tree-
walking is the bottleneck (unlikely for AoC; the puzzles themselves are
usually the performance constraint, not the interpreter).

## 2. Package Layout

```
cRust/
├── cmd/
│   └── crust/              # main.go — CLI entrypoint (run + REPL)
├── internal/
│   ├── token/               # TokenType constants + keyword lookup table
│   ├── lexer/                # source text -> token stream
│   ├── ast/                   # AST node definitions (Statement/Expression)
│   ├── parser/                 # token stream -> AST (Pratt parser)
│   ├── object/                  # runtime value types + Environment
│   ├── interpreter/              # AST -> object.Object (the evaluator)
│   └── builtins/                  # standard library functions
├── examples/                       # sample .crust programs, per-AoC-day templates
├── testdata/                        # golden-file integration tests
└── docs/
    ├── SPEC.md                      # language grammar & semantics (source of truth)
    └── ARCHITECTURE.md               # this file
```

`ast` and `object` are split out from `parser` and `interpreter` respectively
so that lower-level packages never import the packages that consume them —
standard practice to avoid import cycles in a Go interpreter.

## 3. Phase-by-Phase Technical Breakdown

### Phase 0 — Project Foundations
- `go.mod` pins a specific Go version (target latest stable at time of
  writing) so CI and local dev can't silently drift.
- CI runs `go build ./...`, `go vet ./...`, `go test ./...` on every push;
  `gofmt -l` fails the build on unformatted files.
- Directory scaffolding above is created up front so every later phase has
  a fixed home — avoids churny "where does this file go" decisions mid-build.

### Phase 1 — Language Design
- **Type system**: dynamically typed at runtime, no static type checker.
  AoC rewards fast iteration over safety rails; the interpreter catches
  type errors at eval time and reports them with source position.
- **Core runtime types** (previewed here, finalized in `object`):
  `Integer (int64)`, `Float (float64)`, `String`, `Boolean`, `List`, `Map`,
  `Function`, `Builtin`, `Null`.
- **Keyword vocabulary**: single source of truth in
  `internal/token/keywords.go` — a `map[string]TokenType` from pizza term
  to token kind (e.g. whatever term is chosen for "declare a variable"
  maps to `TOKEN_LET`). Renaming a keyword is a one-line change; nothing
  else in the lexer/parser references the literal word.
- **Syntax shape**: brace-delimited blocks (`{ }`), not indentation —
  simpler to Pratt-parse and avoids a Python-style whitespace-sensitivity
  layer. Statement terminators: newline-significant like Go (lexer can
  optionally auto-insert a terminator token at line end after certain
  token kinds), avoiding mandatory semicolons.
- **Grammar**: written and kept current as EBNF in `docs/SPEC.md`. The
  parser's structure should map 1:1 onto that grammar so the two never
  drift apart.

### Phase 2 — Lexer (`internal/lexer`)
- Single-pass, rune-by-rune scanner (`Lexer` struct holds input, current
  position, read position, current rune).
- Emits a flat `Token{ Type TokenType, Literal string, Line, Col int }`
  stream; the parser pulls one token at a time via `NextToken()`
  (no pre-materialized slice needed).
- Multi-character operators (`==`, `!=`, `<=`, `>=`, `&&`, `||`) resolved
  by maximal-munch: peek one rune ahead before deciding the token type.
- Identifiers vs. keywords: scan the full identifier, then look it up in
  the `token/keywords.go` table; unmatched falls back to `IDENT`.
- Lexer errors don't panic — an unrecognized character produces an
  `ILLEGAL` token carrying the offending rune and position, which the
  parser turns into a normal parse error.
- **Tests**: table-driven — given an input string, assert the exact
  expected token sequence.

### Phase 3 — Parser (`internal/parser`)
- **Pratt parsing** (top-down operator precedence) for expressions —
  chosen because arithmetic, comparisons, function calls, and indexing
  all need different binding strengths, and Pratt handles that with two
  small dispatch tables instead of a deep grammar-rule hierarchy:
  - `prefixParseFns map[token.TokenType]func() ast.Expression`
  - `infixParseFns  map[token.TokenType]func(ast.Expression) ast.Expression`
- Precedence levels as an ordered enum: `LOWEST, EQUALS, LESSGREATER, SUM,
  PRODUCT, PREFIX, CALL, INDEX`.
- Statements parsed via straightforward recursive descent —
  `parseStatement()` dispatches on the leading token (variable
  declaration, `if`, loop, function definition, `return`, expression
  statement).
- **AST** (`internal/ast`): two marker interfaces, `Statement` and
  `Expression`, both embedding a `Node` interface (`TokenLiteral()`,
  `String()` for debug-printing/round-tripping). Concrete nodes:
  `LetStatement`, `ReturnStatement`, `IfExpression`, `ForStatement`,
  `FunctionLiteral`, `CallExpression`, `InfixExpression`,
  `PrefixExpression`, `Identifier`, literals, `ListLiteral`, `MapLiteral`,
  `IndexExpression`.
- **Error recovery**: parser errors are collected into a slice rather than
  aborting on the first one, so a single run can report multiple problems
  (skip to a synchronization point — next statement boundary — and keep
  going).

### Phase 4 — Interpreter (`internal/interpreter`, `internal/object`)
- One recursive function, `Eval(node ast.Node, env *object.Environment)
  object.Object`, switching on the Go type of `node`. This is the whole
  evaluator — no separate compile step.
- `object.Object` interface: `Type() ObjectType`, `Inspect() string`.
  Concrete types mirror the runtime type list from Phase 1, plus two
  internal control-flow signal types that are never exposed to user code:
  `ReturnValue` (wraps a value bubbling up through nested blocks) and
  `BreakSignal`/`ContinueSignal` for loop control.
- **Environment** (`object.Environment`): `map[string]object.Object` plus
  an `outer *Environment` pointer. Lookups walk outward through enclosing
  scopes. `NewEnclosedEnvironment(outer)` is created on every function
  call and block entry, giving lexical scoping for free.
- **Closures**: a `Function` object captures the `*Environment` active at
  its definition site. Calling it builds a new enclosed environment over
  that captured one (not the caller's), binds parameters to argument
  values, and evaluates the body in it.
- **Control flow** rides on Go's own control flow: `if`/`else` evaluate
  the condition then recurse into the matching branch; loops are native
  Go `for` loops around repeated `Eval` calls. `break`/`continue`/`return`
  are implemented as sentinel objects that propagate up through
  `Eval`'s block-statement handling until something catches them (loop
  body catches break/continue; function call catches return).
- **Errors**: an `Error` object carries a message and source position and
  propagates like any other value instead of using Go panic/recover in
  the hot path. The top-level `Run()` entrypoint wraps a single
  `recover()` as a last-resort safety net for interpreter bugs, not as
  the primary error-handling mechanism.
- **Truthiness / coercion rules** get pinned down explicitly in
  `docs/SPEC.md` once decided (e.g. whether ints auto-widen to floats in
  mixed arithmetic) so the evaluator has one unambiguous rule to follow
  rather than ad hoc per-operator behavior.

### Phase 5 — Standard Library (`internal/builtins`)
- A `Builtin` object wraps a plain Go function:
  `func(args []object.Object) object.Object`.
- All builtins are registered once into a `map[string]*object.Builtin`.
  Identifier evaluation checks the environment chain first, then falls
  back to this table — so builtins behave like predeclared globals that
  user code can still shadow.
- Most builtins are thin adapters over Go's standard library:
  `strings` (split/join/trim/contains/replace), `strconv`
  (string↔number parsing), `math` (abs/pow/sqrt/gcd/lcm), `sort`
  (list sorting). Input builtins wrap `os.ReadFile` / `bufio.Scanner` for
  reading puzzle input files or stdin.

### Phase 6 — Tooling (`cmd/crust`)
- CLI has two modes: `crust run <file>` (parse + eval one file, exit) and
  `crust repl` (interactive loop). Kept intentionally minimal — manual
  flag handling is enough; no need for a CLI framework dependency.
- The REPL reuses the exact same `Lexer` → `Parser` → `Eval` pipeline as
  file execution, holding one persistent `*object.Environment` across
  lines so variables/functions defined earlier stay in scope.
- Error messages throughout stay in the pizza theme (tone, not
  mechanism) — the underlying `Error` object/position reporting is the
  same regardless of wording.

### Phase 7 — Testing & Quality
- `lexer_test.go` / `parser_test.go`: table-driven unit tests (input
  string in, expected tokens/AST shape out).
- `interpreter_test.go`: evaluate a snippet, assert the resulting
  `object.Object` (type + value).
- Integration tests live in `testdata/`: paired `*.crust` source files
  and `*.golden` expected-output files, run through the real CLI pipeline
  end-to-end and diffed.
- A benchmark test (`testing.B`) runs a real prior-year AoC input through
  cRust to catch performance regressions before they show up mid-contest.

### Phase 8 — AoC 2026 Readiness
Process, not architecture: once Phases 2–5 are solid, do a dry run
solving a handful of old AoC days end-to-end in cRust. Anything that's
awkward to express (missing builtin, clunky syntax) becomes a punch-list
item to fix before December — this is the point of building the language
months ahead of the event instead of the week before.

## 4. Key Design Trade-offs

| Decision | Choice | Why |
|---|---|---|
| Execution model | Tree-walking, no bytecode | Faster to build and debug; AoC input sizes don't need VM-level speed |
| Typing | Dynamic, runtime-checked | Faster puzzle iteration beats compile-time safety for this use case |
| Error propagation | Errors as values (`object.Error`), not panics | Predictable control flow through `Eval`; panic/recover reserved for genuine interpreter bugs |
| Expression parsing | Pratt parser | Precedence/associativity handled by two small tables instead of a large grammar-rule cascade |
| Scoping | Environment chain with outer pointers | Simple, well-understood, gives closures for free |
| Blocks | Braces, not indentation | Simpler lexer/parser; avoids whitespace-sensitivity edge cases |
