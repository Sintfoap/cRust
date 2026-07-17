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
  │  Lexer  │ ─────────▶ │ Parser │ ─────────▶ │ Interpreter │ ─────────────▶
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

### Phase 1 — Language Design ✅
Fully specified in [`docs/SPEC.md`](./SPEC.md) — this section covers the
*implementation* implications of those decisions, not the decisions
themselves.

- **Type system**: dynamically typed at runtime, no static type checker.
  AoC rewards fast iteration over safety rails; the interpreter catches
  type errors at eval time and reports them with source position.
- **Core runtime types** (previewed here, finalized in `object`):
  `Integer (int64)`, `Float (float64)`, `String`, `Boolean`, `List`, `Map`,
  `Set`, `Function`, `Builtin`, `Null`.
- **No declaration keyword**: cRust dropped `let`/`const` in favor of
  bare, Python-style assignment (`x = 1`) — see `SPEC.md` §3 for the
  scoping rule this implies. There's no `TOKEN_LET`; assignment is
  recognized structurally (an lvalue followed by `=`/`+=`/...), not by a
  keyword.
- **Keyword vocabulary**: single source of truth in
  `internal/token/keywords.go` — a `map[string]TokenType` from pizza term
  to token kind (e.g. `"order"` maps to `TOKEN_IF`, `"toppings"` to
  `TOKEN_SET_LIT`; the full mapping is the table in `SPEC.md` §4).
  Renaming a keyword is a one-line change; nothing else in the
  lexer/parser references the literal word.
- **Syntax shape**: brace-delimited blocks (`{ }`), not indentation —
  simpler to Pratt-parse and avoids a Python-style whitespace-sensitivity
  layer. Statement terminators: newline-significant like Go (lexer can
  optionally auto-insert a terminator token at line end after certain
  token kinds), avoiding mandatory semicolons.
- **Operators**: full list (arithmetic, comparison, logical, assignment,
  ranges, increment/decrement, ternary, Elvis, indexing) is `SPEC.md`
  §5. Set union/intersection/difference are builtins, not operators —
  see Phase 5 below. Several additions worth flagging here because they
  touch the lexer directly, not just the parser: range operators
  `..`/`.<` (`SPEC.md` §5.1) and `++`/`--` (§5.2), both statement-level
  like `=` with no prefix/postfix value distinction; the ternary pair
  `(|`/`|)` (§5.3), a single expression-level construct split across
  two tokens (`cond (| then |) else`), lowest precedence of anything in
  the grammar; and Elvis `?:` (§5.4), which sits one level above
  ternary and short-circuits — its right-hand side is only evaluated
  when the left-hand side is `nobox`.
- **Unpacking assignment** (`x, y = list`, `SPEC.md` §3.1): the last
  target always receives a List, even when only one item remains —
  chosen deliberately so consuming code never has to guess whether
  `rest` is a scalar or a list depending on runtime input length.
- **Truthiness/coercion**: pinned down in `SPEC.md` §6 — only `thin` and
  `nobox` are falsy, `/` always produces a float, `+` on mixed
  string/number is a type error. The evaluator should never need a
  per-operator special case beyond what's written there.
- **Grammar**: written and kept current as EBNF in `docs/SPEC.md` §8. The
  parser's structure should map 1:1 onto that grammar so the two never
  drift apart.

### Phase 2 — Lexer (`internal/lexer`)
- Single-pass, rune-by-rune scanner (`Lexer` struct holds input, current
  position, read position, current rune).
- Emits a flat `Token{ Type TokenType, Literal string, Line, Col int }`
  stream; the parser pulls one token at a time via `NextToken()`
  (no pre-materialized slice needed).
- Multi-character operators (`==`, `!=`, `<=`, `>=`, `+=`, `-=`, `*=`,
  `/=`, `%=`) resolved by maximal-munch: peek one rune ahead before
  deciding the token type. `+` peeks for a second `+` (→ `TOKEN_INC`)
  before falling back to `+=`/`+`; `-` does the same for `TOKEN_DEC`.
  `- -x` (two separate unary minuses, double negation) needs an
  explicit space to avoid being lexed as `TOKEN_DEC` — same trade-off
  C makes, and just as rare in practice.
- **Number vs. range disambiguation**: `.` is overloaded between float
  literals (`1.5`) and the range operators (`1..5`, `1.<5`). While
  scanning a number, on hitting `.` the lexer peeks *two* runes ahead:
  digit → part of a float literal; a second `.` → stop the number,
  emit `TOKEN_INT`, then `TOKEN_DOTDOT`; `<` → stop the number, emit
  `TOKEN_INT`, then `TOKEN_DOTLT`. cRust deliberately has no
  trailing-dot float syntax (`5.` is illegal), which is what keeps this
  unambiguous — there's no case where a bare `.` at that position could
  legitimately mean anything else.
- **`(|` / `|)` (ternary)**: `(` peeks for a following `|` (→
  `TOKEN_TERN_THEN`) before falling back to plain `TOKEN_LPAREN`; `|`
  peeks for a following `)` (→ `TOKEN_TERN_ELSE`). `|` has no other
  meaning anywhere in cRust (Set operations are builtins, not `|`/`&`
  operators — `SPEC.md` §5), so a bare `|` not immediately followed by
  `)` is always `ILLEGAL`, and there's no case where `(` or `)` could
  be legitimately confused with these two tokens. Note for editor/
  syntax-highlighting tooling: these tokens contain a real `(` or `)`
  character, so naive bracket-matching will flag them as unbalanced —
  a cosmetic cost the language accepts for the visual pun (`SPEC.md`
  §5.3).
- **`?:` (Elvis)**: `?` peeks for a following `:` (→ `TOKEN_ELVIS`); a
  bare `?` is currently `ILLEGAL` (nothing else uses it yet). `:` on
  its own is still the map-literal pair separator (`SPEC.md` §8,
  `pair`) — maximal-munch only combines it into `TOKEN_ELVIS` when a
  `?` immediately precedes it, so `{"a": 1}` is unaffected. No literal
  parens in this token, so unlike `(|`/`|)` it doesn't trip up generic
  bracket-matching.
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
- Precedence levels as an ordered enum: `LOWEST, TERNARY, ELVIS, OR,
  AND, EQUALS, LESSGREATER, RANGE, SUM, PRODUCT, PREFIX, CALL, INDEX` —
  `TERNARY` sits below everything (§5's ladder puts `(| |)` last),
  `ELVIS` one level above that, `RANGE` slots in between `LESSGREATER`
  and `SUM`. `?:`'s infix parse function recurses back into `ELVIS`
  (not the level above it) for its right-hand side, which is what
  makes it right-associative and chainable (`a ?: b ?: c`); `(|`'s
  parse function is a dedicated three-part production (condition,
  `then`, `else`) rather than a normal infix slot, since it needs to
  consume the matching `|)` and a trailing `else` expression, not just
  one right-hand operand.
- Statements parsed via straightforward recursive descent —
  `parseStatement()` dispatches on the leading token (`recipe`, `order`,
  loop, `serve`, `burnt`, `flip`, block) and otherwise falls through to
  `parseSimpleStatement()`.
- **Assignment has no leading keyword to dispatch on.** `parseSimpleStatement()`
  parses an expression first, then checks what follows it:
  - a `,` → re-enter as an unpacking target list, consume more
    identifiers up to `=`, and build an `UnpackAssignStatement`
    (`SPEC.md` §3.1) — this is the one place the parser needs
    lookahead past a single token, since `x` alone is ambiguous between
    "the start of `x, y = ...`" and "the whole expression statement `x`"
    until the `,` shows up;
  - an assignment operator (`=`, `+=`, ...) → re-interpret what was
    just parsed as an lvalue and build an `AssignStatement`, rejecting
    anything that isn't a bare identifier or index expression;
  - `++`/`--` → build an `IncDecStatement` with the same lvalue check;
  - otherwise → it's an ordinary expression statement (this is also
    the path a bare `(| |)` ternary or `?:` Elvis expression takes,
    since neither is a dedicated statement form).

  This is the same trick Go's own parser uses for assignment — targets
  are parsed as ordinary expressions and validated after the fact,
  rather than given their own grammar branch.
- **AST** (`internal/ast`): two marker interfaces, `Statement` and
  `Expression`, both embedding a `Node` interface (`TokenLiteral()`,
  `String()` for debug-printing/round-tripping). Concrete nodes:
  `AssignStatement`, `UnpackAssignStatement`, `IncDecStatement`,
  `ReturnStatement`, `IfExpression`, `CountedLoop`, `ForEachLoop`,
  `FunctionLiteral`, `CallExpression`, `InfixExpression`,
  `RangeExpression`, `TernaryExpression`, `ElvisExpression`,
  `PrefixExpression`, `Identifier`, literals, `ListLiteral`,
  `MapLiteral`, `SetLiteral`, `IndexExpression`. `TernaryExpression`
  holds three children (`Cond`, `Then`, `Else`) rather than the two an
  `InfixExpression` has. `CountedLoop` and `ForEachLoop` are both
  produced by the same `knead` keyword — the parser picks which one to
  build based on whether it sees a `(` or a bare identifier followed by
  `in` right after `knead`.
- **Error recovery**: parser errors are collected into a slice rather than
  aborting on the first one, so a single run can report multiple problems
  (skip to a synchronization point — next statement boundary — and keep
  going).

### Phase 4 — Interpreter (`internal/interpreter`, `internal/object`)
- One recursive function, `Eval(node ast.Node, env *object.Environment)
  object.Object`, switching on the Go type of `node`. This is the whole
  evaluator — no separate compile step.
- `object.Object` interface: `Type() ObjectType`, `Inspect() string`.
  Concrete types mirror the runtime type list from Phase 1 — including
  `Set`, backed by a Go `map[HashKey]Object` the same way `Map` is —
  plus two internal control-flow signal types that are never exposed to
  user code: `ReturnValue` (wraps a value bubbling up through nested
  blocks) and `BreakSignal`/`ContinueSignal` for loop control.
- **Environment** (`object.Environment`): `map[string]object.Object` plus
  an `outer *Environment` pointer, with two methods reflecting the
  scoping rule in `SPEC.md` §3: `Get(name)` walks outward through
  enclosing scopes; `Set(name, value)` also walks outward looking for an
  *existing* binding to mutate, and only creates a new one in the
  current environment if none is found anywhere in the chain. This one
  method is what implements "assignment with no declare keyword" —
  there's no separate `Define` vs `Assign` split.
- **Only `recipe` calls create a new `Environment`** —
  `NewEnclosedEnvironment(outer)` runs on function call, *not* on
  `order`/`combo`/`special`/`knead`/`bake` block entry. Those blocks
  evaluate their statements directly in the enclosing function (or
  global) scope. This is deliberate and mirrors Python's function-scoped
  (not block-scoped) model: it's what makes `total = total + x` inside a
  `knead` loop update the right variable without a declare keyword to
  pin it to an outer scope.
- **Closures**: a `Function` object captures the `*Environment` active at
  its definition site. Calling it builds a new enclosed environment over
  that captured one (not the caller's), binds parameters to argument
  values, and evaluates the body in it. Because `Set` walks the chain,
  a closure can mutate a variable from its defining scope just by
  assigning to it — no `global`/`nonlocal` equivalent needed.
- **Control flow** rides on Go's own control flow: `if`/`else` evaluate
  the condition then recurse into the matching branch; loops are native
  Go `for` loops around repeated `Eval` calls. `knead`'s for-each form
  (`knead item in collection`) dispatches on the collection's runtime
  type — List and Set iterate their elements, Map iterates its keys —
  binding `item` via the same `Environment.Set` as any other assignment.
  `break`/`continue`/`return` are implemented as sentinel objects that
  propagate up through `Eval`'s block-statement handling until something
  catches them (loop body catches break/continue; function call catches
  return).
- **Errors**: an `Error` object carries a message and source position and
  propagates like any other value instead of using Go panic/recover in
  the hot path. The top-level `Run()` entrypoint wraps a single
  `recover()` as a last-resort safety net for interpreter bugs, not as
  the primary error-handling mechanism.
- **`RangeExpression`** evaluates both bounds, type-checks them as
  `Integer` (else an `Error`), and eagerly builds a `List` — no separate
  lazy Range object, matching `SPEC.md` §5.1 exactly. `.<` is just `..`
  with the upper bound evaluated as `end - 1`.
- **`IncDecStatement`** evaluates to `Environment.Set(name, current + 1)`
  (or `- 1`), reusing the exact same walk-and-mutate `Set` as ordinary
  assignment — it's sugar at the AST level, not a distinct runtime
  mechanism.
- **`UnpackAssignStatement`** evaluates the right-hand side once,
  type-checks it as a `List`, verifies it has at least `N - 1` elements
  for `N` targets, binds the first `N - 1` targets to elements
  positionally via `Environment.Set`, and binds the last target to a
  freshly-allocated `List` of whatever remains (`[]` if nothing does).
- **`TernaryExpression`** (`cond (| then |) else`) evaluates `Cond`
  first, applies the standard truthiness rule (§6, same helper `if`/
  `while` already use), then evaluates and returns *only* `Then` or
  only `Else` — never both, and the unevaluated branch's AST subtree
  is never passed to `Eval`. Chained else-if-ladders
  (`a (| b |) c (| d |) e`) fall out of the parser's right-associative
  `Else` shape (Phase 3) without any special evaluator logic — it's
  just a `TernaryExpression` nested in the `Else` slot of another.
- **`ElvisExpression`** (`a ?: b`) evaluates `a` first; if the result
  isn't `object.NULL`, that's the value and `b`'s AST subtree is never
  passed to `Eval` at all. Only on `nobox` does it evaluate and return
  `b`. Chained `a ?: b ?: c` falls out of the parser's right-associative
  shape (Phase 3) the same way ternary chains do — nested
  `ElvisExpression` nodes, no extra evaluator logic.
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
- The names already locked in — see `SPEC.md` §7 — are `deliver` (print),
  `slices` (length, replacing a generic `len`), `sauce` (nil-coalesce:
  `value` or a `fallback` if `value` is `nobox`), `chars` (string → List
  of characters), and the Set builtins `gather`/`sprinkle`/`scrape`/
  `topped`/`combine`/`shared`/`strip`. These names were chosen
  specifically because dropping `topping`/`sauce` as declaration
  keywords (Phase 1 revision) freed them up to mean something more
  useful as functions — `sauce` in particular reuses the "base layer
  under everything else" metaphor for a fallback value. Math builtins
  keep their standard names on purpose; see `SPEC.md` §7 for why.

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
| Scoping | Environment chain with outer pointers; only `recipe` calls create a new scope | Simple, well-understood, gives closures for free; matches Python's function-scoped model so accumulator patterns work with no declare keyword |
| Variable declaration | None — bare assignment (`x = 1`) creates or updates | Removes `let`/`const` ceremony; walk-and-mutate `Environment.Set` needs no `global`/`nonlocal` equivalent |
| Blocks | Braces, not indentation | Simpler lexer/parser; avoids whitespace-sensitivity edge cases |
| Ranges | Eager `List`, not a lazy Range type | Matches exactly what `1..5` looks like it should produce; avoids a second "sequence" abstraction alongside List this early |
| Increment/decrement | Statement only, `i++` and `++i` identical | Removes C's prefix/postfix return-value distinction entirely — consistent with `=` also being statement-level, not an expression |
| Unpacking's last target | Always a `List`, never sometimes-scalar | One predictable type regardless of input length; avoids call sites needing a runtime check on what they got back |
| Half-pizza glyphs' job | `(|`/`|)` as one matched-pair ternary, not a split coalesce | A two-branch conditional is a genuinely two-part structure, unlike coalescing (one fallback) — the shape fits the job better; accepted cosmetic cost is that generic editor bracket-matching sees an unmatched paren inside each token |
| Nil-coalescing spelling | Conventional Elvis `?:`, `sauce()` builtin unchanged | Frees the half-pizza pair for ternary; `?:` is a well-known convention so it needs no introduction, and has no bracket-matching downside |
