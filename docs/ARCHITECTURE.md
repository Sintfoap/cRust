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

### Phase 0 — Project Foundations ✅
- `go.mod` pins Go 1.24 (`github.com/Sintfoap/cRust`) so CI and local
  dev can't silently drift.
- CI (`.github/workflows/ci.yml`) runs `go build ./...`, `go vet ./...`,
  `gofmt -l`, and `go test ./...` on every push, **Linux only**. Go's
  standard library is highly portable and nothing in this project
  touches OS-specific behavior (no syscalls, no path/filesystem
  quirks beyond a plain file read), so a single-platform CI is very
  unlikely to miss a real bug. If a one-off macOS/Windows binary is
  ever needed, that's a free `GOOS=darwin GOARCH=arm64 go build` — no
  CI investment required to unlock it.
- **No release workflow yet, deliberately.** There's nothing worth
  distributing at Phase 0/1; `go build ./cmd/crust` or
  `go run ./cmd/crust` from source is the only supported path for now.
  A GitHub Releases workflow (goreleaser or a manual build matrix) is
  easy to bolt on later once the language actually does something.
- **`flake.nix` is not a release workflow and doesn't contradict the
  point above** — it's a reproducible *build recipe* (`pkgs.buildGoModule`
  compiling from source), not a prebuilt-binary distribution channel,
  so it fits the "build from source" decision rather than working
  around it. It's what makes `nix run github:Sintfoap/cRust` work from
  any Nix-enabled system (NixOS, Nix-on-WSL, Nix on macOS/Linux)
  without the repo needing to publish anything itself.
  - `vendorHash = null` since `go.mod` has no `require`s yet (stdlib
    only) — nothing to vendor. This needs to become a real hash the
    moment a third-party Go dependency is added; `nix build` prints the
    correct value on a mismatch.
  - `subPackages = [ "cmd/crust" ]` restricts the build to the CLI
    binary specifically, independent of whatever exists under
    `internal/` at any given phase.
  - `meta.mainProgram = "crust"` plus an explicit `apps.default` (via
    `flake-utils.lib.mkApp`) covers both older and newer Nix versions'
    ways of resolving what `nix run` should execute.
  - A `devShells.default` with `pkgs.go` is included as the standard,
    low-cost companion to a Go flake — lets `nix develop` give a
    hacking environment without needing Go installed globally.
  - **Not yet verified end-to-end**: this dev environment's network
    policy blocks `nixos.org` (confirmed via the proxy status endpoint),
    so Nix itself couldn't be installed here to test `nix build`/
    `nix run` against the real thing. The flake follows well-established
    `buildGoModule` + `flake-utils` conventions, but it should be run
    through `nix flake check` on a real Nix install before being
    trusted blindly.
  - No `flake.lock` is committed yet, for the same reason: locking
    requires Nix to actually resolve and hash `nixpkgs`/`flake-utils`
    against real network access. `nix run`/`nix build` still work
    without one (Nix resolves an ephemeral lock on the fly), but running
    `nix flake lock` once on a real machine and committing the result
    is what makes future runs reproducible instead of floating on
    whatever `nixos-unstable` currently points to.
- `cmd/crust/main.go` exists now, ahead of Phase 6, with a genuine but
  minimal CLI surface: `--version`, `--help`/`-h`, and `run`/`repl`
  subcommands that print "not implemented yet" and exit 1 rather than
  doing nothing. This is worth having from day one — it gives CI a
  concrete smoke test (`go build && ./crust --version`) and means the
  binary never silently no-ops before Phase 6 fills in the real
  interpreter. The version string is a package-level `var version =
  "dev"`, meant to be overridden at build time via
  `-ldflags "-X main.version=1.2.3"` once there's a tagging scheme.
- `main()` is a thin wrapper around `run(args []string, stdout, stderr
  io.Writer, colorDefault bool) int` — pushing the actual logic into a
  function that takes writers and returns an exit code (rather than
  calling `os.Exit`/writing to `os.Stdout` directly throughout) is what
  makes `cmd/crust/main_test.go` able to assert on output and exit
  codes without subprocess spawning. `colorDefault` is computed once in
  `main()` from real process state (`os.Stdout` + `$NO_COLOR`) and
  passed in as a plain bool specifically so tests can force either path
  deterministically without needing a real TTY. Same shape will likely
  make sense for the eventual `run`/`repl` implementations in Phase 6.
- **`cmd/crust/banner.go`**: the `--help` screen (and bare/no-arg
  invocation) prints the pizza banner from the README, redrawn as
  colored terminal text, ahead of Phase 6's "pizza-themed error
  messages" item:
  - The pizza is one Go raw string (`pizzaArt`) — plain characters,
    no markup. Coloring is entirely **character-identity-based**:
    `@`/`#` is crust, `o` is pepperoni, `*` is basil, everything else
    printable is cheese. This is the same trick the original HTML/CSS
    banner used (`SPEC.md`-adjacent, see the README banner's own
    history) — no separate "layout map" to keep in sync with the art.
  - Colors are **24-bit ANSI true-color** escapes (`\x1b[38;2;R;G;Bm`),
    with R/G/B taken directly from `assets/banner.png`'s hex palette
    (`#c88a3a`/`#f2c744`/`#e05a45`/`#7cbf58`), so the terminal banner
    and the README image agree exactly. True color isn't universal but
    is widely supported by anything a WSL/Linux/macOS user is likely
    running; there's no fallback tier (16/256-color) since the
    graceful failure mode (a modern terminal ignoring/approximating an
    unsupported escape) is acceptable for a decorative banner.
  - `--toppings <list>` (`pepperoni,basil` default, or `all`/`plain`)
    controls which of the two topping layers render in their real
    color vs. collapse to plain cheese (`.`) — implemented by
    `parseToppings` returning a `map[string]bool` that `renderPizza`
    consults per-character. The pizza's *shape* (crust outline, cheese
    texture) never changes; only whether the `o`/`*` positions show
    their topping or blend into the cheese.
  - `--no-color` and `$NO_COLOR` (checked in `main()`, not `run()`,
    since it's real process state) both suppress the ANSI codes;
    `--no-banner` skips the pizza entirely and prints just the usage
    text. All three are independent of `--toppings`/etc. being
    otherwise irrelevant to non-banner invocations.
  - Below the pizza, `crustLogo` is a block-letter "cRust" wordmark
    (figlet's `ansi_shadow` font — generated once via `pyfiglet` and
    pasted into the Go source as a raw string, not generated at
    runtime) plus a smaller "language baked better" tagline, both
    centered under the pizza by `centeredBlock`/`centeredLine`. Colored
    as one flat color per line (not character-identity-based like the
    pizza) since block-font glyphs don't carry topping semantics.
    figlet fonts at this size don't distinguish letter case, so it
    renders as CRUST rather than mixed-case "cRust" — matching the
    reference banner's own all-caps convention read better here than
    hand-forcing a visually-smaller "c" into an otherwise uniform font.
  - **No `golang.org/x/term` dependency for TTY detection** — `isTerminal`
    uses `os.Stdout.Stat()` and checks `os.ModeCharDevice` directly, a
    stdlib-only heuristic that works on Linux/macOS/Windows consoles.
    This is deliberate: the project has zero third-party Go
    dependencies right now, which is exactly what keeps the Nix
    flake's `vendorHash = null` valid (§ above) — pulling in a real
    dependency just for isatty detection would break that for a
    cosmetic feature.
  - Bare-file invocation (`crust day01.crust`, no `run` keyword) is
    now equivalent to `crust run day01.crust` — a small UX borrow from
    a similar tool a friend of the project's author built, adopted
    because it's a genuine ergonomics win independent of where the
    idea came from, not because the surrounding command surface was
    copied (it wasn't — no `build`/`check`/`lsp`/"expansion" commands
    exist, because cRust doesn't compile and has no diagnostics engine;
    inventing those to mimic the shape would misrepresent what the
    tool actually does).
- Directory scaffolding (`internal/{token,lexer,ast,parser,object,
  interpreter,builtins}`) from §2's package layout is **not** created
  yet as empty stub packages — those get created with real content
  when their own phase starts (Phase 2 creates `token`+`lexer`, Phase 3
  creates `ast`+`parser`, etc.). `examples/` and `docs/` already exist
  from Phase 1.

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

### Phase 2 — Lexer (`internal/lexer`) ✅
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
  the keyword table in `token/token.go` (`token.LookupIdent`); unmatched
  falls back to `IDENT`. (This ended up living alongside the `Type`
  constants in one small file rather than a separate `keywords.go` —
  the package is small enough that splitting it further didn't earn
  its keep.)
- **Newline collapsing**: a run of blank lines, whitespace, and
  comments between two real tokens collapses into a single `NEWLINE`
  token rather than one per physical line. This wasn't pinned down in
  `SPEC.md` ahead of time; it's a lexer-level choice that keeps the
  token stream — and therefore Phase 3's parser — from needing to
  special-case runs of empty statements. **Phase 3 still needs to
  tolerate a *leading* `NEWLINE`** (blank lines before the first
  statement in a file or block resolve to one token the grammar's
  `program`/`block` rules don't have an explicit slot for) — the
  straightforward fix is skipping leading `NEWLINE` tokens where a
  statement sequence begins.
- **Line continuation inside `(` / `[`** (added after Phase 3 shipped,
  found via `crust parse` on the example files rather than during
  Phase 2 itself — see the note below): a `Lexer.brackets` stack tracks
  every currently-unclosed `(`, `[`, or `{`, innermost last. A raw
  `\n` is swallowed as insignificant whitespace instead of becoming a
  `NEWLINE` token whenever the *innermost* entry is `(` or `[` — the
  same rule Python and most C-family languages use for wrapping call
  arguments and list literals across lines. This has to be a stack, not
  a single open/close counter: what governs newline significance is the
  innermost delimiter specifically, not "is anything open at all," so
  a `{` block nested inside a still-open `(` — e.g. an anonymous
  `recipe` literal passed as a call argument,
  `apply(recipe(x) { serve x * 2 })` — keeps its own body's newlines
  meaningful even though the outer `(` hasn't closed yet. `{` itself
  never enables continuation (regardless of what's below it on the
  stack), which — as a side effect, not a targeted decision — also
  means map/set literals (also `{`-delimited) stay single-line for now,
  since the lexer has no way to tell a block's `{` from a
  `mapLiteral`/`setLiteral`'s at this context-free stage; see `SPEC.md`
  §8's note on this.

  → This was a real gap, not a deliberate limitation discovered and
  then accepted: it surfaced when `crust parse` (added alongside this
  fix, see Phase 6 below) was run against every file in `examples/` for
  the first time and `ternary_elvis.crust`'s multi-line chained-ternary
  formatting failed to parse — a bare, un-bracketed line break with no
  syntax around it, which even this fix can't (and isn't meant to)
  solve, so that one construct got reformatted to wrap in an explicit
  `(...)` instead, rather than the language growing bare multi-line
  expression continuation. The fix itself is purely additive: every
  input that lexed successfully before still lexes to the identical
  token stream (nothing that used to produce a `NEWLINE` inside a
  balanced `{...}`, or outside any bracket, changed), so this shipped
  without a version bump or a compatibility note beyond this one.
- **Comments and string escapes** are formalized in `SPEC.md` §2.1
  now that they're implemented, since both were already in constant use
  throughout every example file without ever being written down: `//`
  line comments (stripped by the lexer, no grammar production, no
  block-comment form), and a small string escape set (`\" \\ \n \t
  \r`) — an unrecognized `\x` is a lexer error rather than a silent
  passthrough, since that's far more likely a typo.
- Lexer errors don't panic — an unrecognized character produces an
  `ILLEGAL` token carrying a message and position, which the parser
  will turn into a normal parse error in Phase 3.
- **Tests** (`internal/token`, `internal/lexer`): table-driven per
  `SPEC.md` feature (operators, keywords, numbers incl. the range/float
  disambiguation, strings incl. every error path, comments, newline
  collapsing, line/col tracking), plus one test that lexes every real
  `.crust` file under `examples/` end-to-end and asserts no `ILLEGAL`
  token turns up — a cheap way to catch gaps the hand-written cases
  miss, since those files exercise every feature together rather than
  in isolation. 99%+ statement coverage on `internal/lexer`.
- **`crust tokens <file>`** (`cmd/crust/tokens.go`, Phase 6 work done
  ahead of schedule): reads a file, runs it through `lexer.New`, and
  prints one line per token (`line:col  TYPE  literal`) until `EOF` or
  the process would otherwise have nothing to show for `run`/`repl`
  still being stubs. It's the only way to see the lexer act on real
  input from the CLI right now, and doubles as a debugging aid for
  Phase 3 once the parser exists to consume this same stream. String
  and `ILLEGAL` literals print `%q`-quoted so embedded newlines/tabs
  (from escape sequences, or an error message) can't break the
  one-line-per-token output. Exit code is 1 if any `ILLEGAL` token
  turns up, 0 otherwise — but every token still gets printed either
  way, since seeing what came *before* the bad token is usually the
  point of reaching for this in the first place.

### Phase 3 — Parser (`internal/parser`) ✅
- **Pratt parsing** (top-down operator precedence) for expressions —
  chosen because arithmetic, comparisons, function calls, and indexing
  all need different binding strengths, and Pratt handles that with two
  small dispatch tables instead of a deep grammar-rule hierarchy:
  - `prefixParseFns map[token.Type]func() ast.Expression`
  - `infixParseFns  map[token.Type]func(ast.Expression) ast.Expression`
- Precedence levels as an ordered `iota` enum: `LOWEST, TERNARY, ELVIS,
  OR, AND, EQUALS, LESSGREATER, RANGE, SUM, PRODUCT, PREFIX, CALL,
  INDEX` — `TERNARY` sits just above `LOWEST` (§5's ladder puts `(| |)`
  last), `ELVIS` one level above that, `RANGE` slots in between
  `LESSGREATER` and `SUM`.
  - `?:`'s infix parse function recurses at `ELVIS - 1` (i.e. at
    `TERNARY`'s numeric level) for its right-hand side, *not* at
    `ELVIS` itself. This is the standard Pratt right-associativity
    trick, and the reason it has to be `-1` rather than the same level
    is worth spelling out since it's easy to get backwards: recursing
    at precedence `ELVIS` would make the loop condition
    `ELVIS < peekPrecedence()` false for a second `?:` (equal
    precedences don't satisfy strict `<`), so that second `?:` would be
    left for the *outer* loop to consume instead — producing
    left-associativity (`(a ?: b) ?: c`), the opposite of what's
    wanted. Dropping to `ELVIS - 1` makes a follow-on `?:` satisfy the
    recursive call's own loop condition, so it gets folded into the
    right-hand side instead — right-associative, chainable
    (`a ?: (b ?: c)`) — while still being high enough to exclude a
    trailing ternary (`TERNARY`, one level further down), which
    correctly isn't part of `elvis`'s own grammar production.
  - `(|`'s parse function is a dedicated three-part production
    (condition, `then`, `else`) rather than a normal infix slot, since
    it needs to consume the matching `|)` and a trailing `else`
    expression, not just one right-hand operand. Both `then` (between
    the delimiters) and `else` (after `|)`) parse at `LOWEST` — for
    `else` specifically, that's equivalent to parsing a full nested
    ternary again, since `TERNARY` is the lowest real operator
    precedence above `LOWEST`, which is exactly what `ternary`'s own
    grammar production needs (`ternary = elvis [ "(|" expression "|)"
    ternary ]` — the `else` branch is itself a `ternary`).
  - Ranges (`start..end` / `start.<end`) need an explicit anti-chaining
    check that precedence alone can't provide: parsing `end` at `SUM`
    (`term`) precedence stops the *recursive* call from swallowing a
    second range operator, but doesn't stop the *outer* Pratt loop —
    which was entered at whatever precedence the caller used, often
    `LOWEST` — from re-entering the range parse function with the
    already-built range as its new left operand. `1..5..10` would
    otherwise silently parse as `(1..5)..10`. `parseRangeExpression`
    checks the peek token after building the range and records a parse
    error if another `..`/`.<` immediately follows, rather than letting
    the chain form.
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
  are parsed as ordinary expressions and validated after the fact
  (`isValidLvalue`: only `*Identifier` and `*IndexExpression` qualify),
  rather than given their own grammar branch. The same validation is
  reused for both forms of `incDecStmt` (`x++` and `++x`).
- **`knead` dispatch** needs only one token of lookahead, not the
  general expression backtracking `parseSimpleStatement` uses: a `(`
  immediately after `knead` always means `countedHeader`, a bare
  identifier always means `forEachHeader` — the two productions can't
  start with the same token, so `parseKneadStatement` just checks
  `peekToken` once.
- **Block vs. map-literal ambiguity at `{`**: `{` starting a *statement*
  is always parsed as a `BlockStatement`; `{` reached while parsing an
  *expression* (assignment RHS, call argument, etc.) is always a
  `MapLiteral`, via `prefixParseFns[LBRACE]`. `parseStatement`'s switch
  branches on `LBRACE` before expression parsing ever gets a chance to
  run, so there's no runtime ambiguity to resolve — just two code paths
  that never see the same `{`.
- **Terminators**: `expectTerminator()` consumes a trailing `NEWLINE` or
  `;` if present, but also silently accepts a following `}` or `EOF` as
  an *implicit* terminator without consuming it — so the last statement
  in a block never needs a trailing newline before its closing brace.
  Only `simpleStmt` (assignment/unpack/inc-dec/expression) and
  `serve`/`burnt`/`flip` call it; `recipe`/`order`/`knead`/`bake` all
  end in a `block` per their own grammar productions, with no
  terminator token to consume.
- **AST** (`internal/ast`): two marker interfaces, `Statement` and
  `Expression`, both embedding a `Node` interface (`TokenLiteral()`,
  `String()` for debug-printing/round-tripping). Concrete nodes:
  `Program`, `BlockStatement`, `ExpressionStatement`, `AssignStatement`,
  `UnpackAssignStatement`, `IncDecStatement`, `ReturnStatement`,
  `BurntStatement`, `FlipStatement`, `IfStatement` (+ `ComboClause`),
  `CountedLoop`, `ForEachLoop`, `BakeStatement`, `FunctionLiteral`,
  `Identifier`, `IntegerLiteral`, `FloatLiteral`, `StringLiteral`,
  `BooleanLiteral`, `NilLiteral`, `ListLiteral`, `MapLiteral` (+
  `MapPair`), `SetLiteral`, `PrefixExpression`, `InfixExpression`,
  `RangeExpression`, `TernaryExpression`, `ElvisExpression`,
  `CallExpression`, `IndexExpression`. `TernaryExpression` holds three
  children (`Cond`, `Then`, `Else`) rather than the two an
  `InfixExpression` has. `BakeStatement` (`bake (cond) { block }`,
  SPEC.md §8 `bakeStmt`) wasn't in this list in an earlier draft of
  this doc — added alongside the rest of the loop nodes once the AST
  was actually written, since a plain conditional loop needs its own
  node the same way `CountedLoop`/`ForEachLoop` do. `CountedLoop` and
  `ForEachLoop` are both produced by the same `knead` keyword — see the
  dispatch note above. (Corrected from an earlier draft's
  `IfExpression`: `order`/`combo`/`special` are statements, not
  expressions — SPEC.md §5.3 draws that line specifically to justify
  the ternary's own existence, so the AST node name needs to agree.)
  `FunctionLiteral`'s `Name` field is a `*Identifier` that's `nil` for
  an anonymous literal (SPEC.md's `recipeStmt` grammar note); there's
  no separate "recipe declaration" node — `parseRecipeStatement` just
  wraps a named `FunctionLiteral` in an `ExpressionStatement`, and
  deciding what a named one *means* (binding it in the environment) is
  left to `Eval` in Phase 4, not resolved at parse time.
- **Error recovery**: parser errors are collected into a slice
  (`Parser.Errors()`) rather than aborting on the first one, so a
  single run can report multiple problems. A statement that fails to
  parse triggers `synchronize()`, which skips tokens until it finds a
  terminator, a block's closing `}`, or a keyword that starts a new
  statement — so one malformed statement doesn't cascade into a wall of
  unrelated follow-on errors.
- **Parser unit tests** (`internal/parser/*_test.go`) are table-driven,
  93%+ coverage: operator-precedence round-trips via `String()`
  (matching the classic Pratt-parser test style), every statement form,
  associativity checks for `?:` and chained `(| |)`, the range
  anti-chaining check, and a broad table of malformed inputs
  (`errors_test.go`) asserting each one produces at least one recorded
  error rather than a panic or a silently-wrong tree.
- **`crust parse <file>`** (`cmd/crust/parse.go`, Phase 6 work done
  ahead of schedule, same rationale as `crust tokens` for Phase 2):
  reads a file, runs it through `Lexer` → `Parser`, and prints one
  numbered line per top-level statement using that node's `String()`
  — which fully parenthesizes every expression, so precedence and
  associativity are visible directly in the output without a separate
  tree-printer. Whatever did parse is printed to stdout first, then any
  `Parser.Errors()` go to stderr and the exit code goes non-zero — same
  "show what you got, then flag the problem" shape as `crust tokens`.
  Running this against every file in `examples/` for the first time is
  what caught the line-continuation gap described in Phase 2 above —
  exactly the kind of thing this tool exists to surface before Phase 4
  has to debug it blind.

### Phase 4 — Interpreter (`internal/interpreter`, `internal/object`) 🚧
**`internal/object` exists already** (built ahead of schedule, alongside
the Performance Strategy work below) — `Object`, `ObjectType`,
`Integer`/`Float`/`String`/`Boolean`/`Null`/`List`/`Map`/`Set`,
`HashKey`/`Hashable`, and `Environment` are real, tested code, not just
this plan. **`internal/ast` and `internal/parser` are also done now**
(Phase 3, above), so `Function` no longer has a blocker on the AST
side. **Still not built**: `Function` itself, `Error`/`ReturnValue`/
`BreakSignal`/`ContinueSignal` (the control-flow signal types —
deferred because they need a decided error-value convention, which is
naturally a Phase 4 question once there's an `Eval` to design it
around), and all of `internal/interpreter` (`Eval` itself). The
bullets below describe that remaining, still-planned work.

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
- **Entry points** (`SPEC.md` §9, designed ahead of this phase existing
  to implement it): after evaluating every top-level statement, the
  `run` command looks for a top-level `recipe` named `store` or
  `store_<name>` and, if `--store=<name>` (or its absence, for the bare
  `store` case) resolves to one, calls it with zero arguments. This is
  a `cmd/crust` + entry-point-resolution concern layered on top of
  `Eval`, not a new `ast`/`parser` concept — `store_part1` is just an
  ordinary named `recipe`, so nothing upstream of this phase needs to
  change to support it. Still fully unimplemented, since it needs `Eval`
  to exist first.

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
- `crust run <file> --store=<name>` selects which `store`/`store_<name>`
  recipe the file's entry point resolves to (`SPEC.md` §9) — omitted,
  it selects the bare `store`. Still unimplemented pending Phase 4.
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
| CI platform coverage | Linux only, no matrix | Go's stdlib is portable and this project has no OS-specific code; a full macOS/Windows CI matrix would cost real minutes to catch a bug that's very unlikely to exist |
| Distribution | Build from source (`go build`/`go run`); no release workflow yet | Nothing worth shipping to non-developers at Phase 0/1; a release workflow is cheap to add later and premature now |
| CLI shape | `main()` → `run(args, stdout, stderr) (code int)`, not `os.Exit`/`os.Stdout` sprinkled through the logic | Makes the CLI unit-testable (`main_test.go`) without subprocess spawning; the same shape carries forward into Phase 6's real `run`/`repl` |
| Nix packaging | `flake.nix` via `buildGoModule`, no `flake.lock` committed yet | Builds from source, so it's consistent with "no release workflow" rather than a separate distribution channel; the lock file needs a real Nix install (network access this dev environment doesn't have) to generate correctly |
| Banner colors | 24-bit true-color ANSI, no 256-color fallback tier | Matches `assets/banner.png`'s hex palette exactly; a decorative help-screen banner degrading ungracefully on an ancient terminal isn't worth a second color-rendering path |
| TTY detection | `os.Stdout.Stat()` + `os.ModeCharDevice`, not `golang.org/x/term` | Keeps the project's dependency count at zero, which is what keeps the Nix flake's `vendorHash = null` valid — not worth breaking for isatty detection |

## 5. Performance Strategy

A tree-walking interpreter's cost is lopsided: the lexer and parser run
**once** per program (even a chunky AoC solution is a few hundred lines
of source), while `Eval` and `Environment` lookups run **potentially
millions of times** in a tight `knead` loop over a big grid or input
set. That asymmetry is what decides where optimization effort actually
pays for itself — the list below is split accordingly, and the second
half is intentionally *not implemented*, because guessing past what's
proven necessary is how simple interpreters grow accidental complexity.

### Committed to now (implemented in `internal/object`, cheap, no design cost)

- **Singleton `TRUE`/`FALSE`/`NULL`.** `stuffed`/`thin`/`nobox` get
  evaluated constantly. One shared `*Boolean`/`*Boolean`/`*Null`
  instance each, reused everywhere, instead of allocating on every
  evaluation — safe because these types carry no mutable state.
- **Small-integer cache** (`NewInteger`, `-256..256`). AoC loops are
  full of small counters, indices, and grid-neighbor deltas (`-1, 0,
  1` turn up constantly in "check 4/8 neighbors" code). Returning a
  cached `*Integer` instead of allocating one is the single
  highest-value change here — it lands directly on the hot path
  (arithmetic inside loops), for near-zero implementation cost. Safe
  for the same reason as the singletons: cRust Integers are immutable,
  so two equal values sharing a pointer is unobservable.
- **`List`/`Map`/`Set` as reference types** (pointer receivers over a
  slice/map, never a value type). This one is a correctness
  requirement first, performance win second: SPEC.md's `sprinkle`/
  `scrape`/etc. mutate a Set in place, which only works if passing one
  around doesn't copy it. The same representation choice happens to
  mean a 10,000-element list argument to a `recipe` call doesn't get
  deep-copied either.
- **Precomputed `HashKey`** (`Type ObjectType; Value uint64`) for
  Map/Set keys, computed once via a small `Hashable` interface
  (`Integer`/`Float`/`String`/`Boolean`) rather than reflection-based
  hashing on every lookup. `HashKey.Type` matters as much as `.Value`
  here — `Integer(1)` and `TRUE` both naturally hash their `.Value` to
  `1`, and only the `Type` field keeps them from colliding as Map/Set
  keys.
- **`strings.Builder` for anything building a string in a loop**
  (already the convention in `cmd/crust/banner.go`; locked in as a
  requirement for Phase 5's string-processing builtins —
  `chars`/joins/`deliver` formatting — before any of them are
  written). Repeated `+=` concatenation is the classic O(n²) trap and
  there's no reason to ever write it here.

### Deliberately deferred until a profile says otherwise

- **Slot-resolved variable access instead of map-based `Environment`.**
  The "real" way fast interpreters do variable lookup: resolve each
  local to a fixed array index at parse time, so runtime access is
  `frame.slots[3]` instead of a map lookup. It's a genuine speedup —
  but it sits in real tension with cRust's own design. SPEC.md §3's
  whole variable model is "no declarations; assignment walks the scope
  chain *at runtime* and creates bindings dynamically." A static slot
  resolver assumes every variable's home scope is knowable ahead of
  time, which is a much easier guarantee in a language with mandatory
  `let` than in one where `total = total + x` might create `total` or
  might mutate an existing one three scopes up, decided only by
  walking the chain at the moment it runs. Plain `map[string]Object` +
  outer pointer (what's actually built) is simpler, correct, and very
  likely fast enough for AoC-scale inputs. Not touching this without a
  benchmark specifically pointing at `Environment.Get`/`Set`.
- **Arena/bump allocation** for AST or Object nodes, to cut GC
  pressure. Real technique, real implementation cost (lifetime
  management stops being "let Go's GC handle it"). Speculative until
  proven necessary.
- **A bytecode VM.** Already a stretch goal in `TODO.md`, already
  reasoned about in §4's trade-off table (execution model row) — not
  reopening that here.

### When to revisit this list

`TODO.md`'s Phase 7 already has "Benchmark against a real prior-year
AoC puzzle for performance sanity" as a checklist item. That's the
right moment to profile (`go test -bench` + `pprof`) rather than
extending the deferred list above by guesswork — if `Environment`
lookups or GC pressure actually show up as the bottleneck there, that's
the point to reconsider slot resolution or arenas, backed by a number
instead of a hunch.
