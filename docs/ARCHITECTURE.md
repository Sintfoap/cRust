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
  - Two `devShells`, for two different audiences: `devShells.default`
    (`pkgs.go` only) is for hacking on cRust's own source — you'd use
    `go run`/`go build` directly while working in this repo, not the
    packaged binary. `devShells.crust` (`self.packages.${system}.default`)
    is for *using* cRust — writing/running `.crust` files, e.g. AoC
    solutions — without needing a separate workspace flake or a global
    PATH change: `nix develop .#crust` (or
    `nix develop github:Sintfoap/cRust#crust` from outside a clone)
    just puts the built `crust` CLI on PATH for the shell session. Added
    specifically because `nix profile add`'s alternative — installing
    into a persistent profile — turned out to have a real, non-obvious
    failure mode in practice (see the next bullet).
  - **`nix profile add`/`install` needs the profile's `bin/` directory
    on your shell's `PATH`, and that's easy to have silently missing**
    — confirmed against a real user's setup, not hypothetical: `nix
    profile add github:Sintfoap/cRust` completed with no error and the
    binary genuinely existed at the resolved store path, but `crust`
    still weren't found, because neither their shell's `PATH` nor any
    of their shell rc files mentioned `~/.nix-profile/bin` (or the
    newer `~/.local/state/nix/profile/bin` equivalent) at all — running
    `nix profile add` repeatedly while troubleshooting also silently
    created several redundant profile entries (`cRust`, `cRust-1`,
    `cRust-2`, ...) rather than erroring or being a no-op, since Nix
    doesn't dedupe by identical source on `profile add`. None of this
    is a flake bug — it's what motivated adding `devShells.crust` as
    the recommended path for "just let me run `crust`" instead.
  - `nix run`/`nix build`/`nix develop` have since been confirmed
    working end-to-end by a real user on a real Nix install (Nix-on-WSL)
    — this environment's own network policy still blocks `nixos.org`
    directly (confirmed via the proxy status endpoint, `cache.nixos.org`
    itself *is* reachable), so `devShells.crust` above was reviewed
    carefully by hand against the already-working `apps.default`
    pattern in the same file (identical `self.packages.${system}.default`
    reference) rather than independently exercised with a live `nix
    develop .#crust` run here.
  - `flake.lock` is committed (added directly by a user with real Nix
    access, since it couldn't be generated in this environment) —
    `nix flake update` on a real machine is what refreshes it going
    forward; `.gitattributes`' `eol=lf` setting exists specifically
    because of line-ending damage a previous `flake.lock` update
    accidentally introduced repo-wide (see the Phase 0 git history for
    that cleanup).
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
- **`(a, b, ...)` tuple literals** (`SPEC.md` §2.3) parse through the
  same shared `parseGroupedExpression` prefix parselet every `(` in the
  grammar already goes through — not a special case scoped to
  unpacking. One expression's worth of lookahead resolves the
  ambiguity with ordinary grouping: parse the first inner expression
  via the normal `parseExpression`, then check what follows — a `,`
  commits to a `*ast.TupleLiteral` (collect the rest,
  `expectPeek(RPAREN)`); a bare `)` means it was just `(x)`-style
  grouping all along, so the already-parsed expression is returned as
  the grouped value and the surrounding `parseExpression` call's own
  infix loop picks up anything trailing the `)` the normal way (`(x)..y`
  still parses as a Range, not a truncated expression) — no special
  continuation logic needed, since this all happens inside
  `parseGroupedExpression` itself rather than a separate caller trying
  to resume `parseExpression` from outside.
  - This design point is worth calling out because an earlier version
    of this feature took the opposite approach — parsing `(a, b)` only
    at the direct right-hand side of an unpacking assignment, as
    dedicated sugar with no real value type behind it — specifically
    to keep the change small. That fell over the moment real usage
    needed to unpack a Tuple produced by a *ternary* choosing between
    two of them (`x, y = cond (| (a, b) |) (c, d)`): by the time the
    outer assignment sees the ternary's result, it's just a runtime
    value with no memory of having been written with `(...)` syntax,
    so sugar recognized only at one parse-time position structurally
    can't reach it. Promoting `(a, b, ...)` to a real, general
    `TupleLiteral` — parseable (and, at the object level, a real
    `object.Tuple`) anywhere any other expression is — was the only way
    to make unpacking dispatch on what a value *is* at runtime rather
    than how the AST node that produced it happened to be shaped,
    which is what makes it work through a ternary, an `elvis`, a
    function return, or anything else.
  - `internal/interpreter/expressions.go`'s `evalTupleLiteral` requires
    every element to itself be `Hashable` (`internal/object`) — checked
    once here, at construction, specifically so `object.Tuple`'s own
    `HashKey` (a combination of every element's own `HashKey`, via
    `fnv`, folding in each element's `ObjectType` too so e.g. a
    Boolean and a same-`Value` Integer element can't blend together)
    never has to handle a non-`Hashable` element itself. This is the
    same "narrower than `Object` itself" restriction `evalMapLiteral`
    already enforces for map keys (`SPEC.md` §2's String-or-Integer
    rule), applied to every element rather than just keys — and it's
    what makes `Tuple` safely usable as a `Map` key or `Set` element
    (`sprinkle(seen, (x, y))` for grid-coordinate dedup, the single
    biggest practical reason to want this over a `List`, which can
    never be hashed since its contents can change after insertion).
  - `internal/interpreter/statements.go`'s `evalUnpackAssignStatement`
    dispatches on `Value`'s *runtime* type once evaluated, not
    anything visible in the AST: a `*object.Tuple` result unpacks with
    exact arity (every target, including the last, gets its own bare
    value; a count mismatch is a runtime error, not silently
    padded/truncated); a `*object.List` result keeps the classic rule
    (first N-1 targets bare, the last always List-wrapped). Either way
    `Value` is evaluated exactly once before any target is assigned, so
    `a, b = (b, a)` swaps correctly rather than clobbering `b` before
    it's read.
  - `Tuple` otherwise behaves like a read-only `List`: indexable
    (`readIndex`), iterable (`evalForEachLoop`), included in `slices`
    (`internal/builtins`), and compared by contents in
    `objectsEqual`/`listsEqual` (factored to take `[]object.Object`
    directly so `List` and `Tuple` — which only differ in mutability,
    not in what "equal" means — share one implementation). `writeIndex`
    is the one place it diverges: a `Tuple` index-assignment is a
    runtime error (`Tuple is immutable, does not support index
    assignment`) rather than falling through to `List`'s in-place
    mutation — immutability is what makes the hashability above safe in
    the first place, not just a style choice.
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

### Phase 4 — Interpreter (`internal/interpreter`, `internal/object`, `internal/builtins`) ✅
**`internal/object` and `internal/ast`/`internal/parser` were already
done** (built ahead of schedule — see the Performance Strategy notes
and Phase 3, above). This phase filled in the rest: `Function`,
`Error`, `ReturnValue`/`BreakSignal`/`ContinueSignal` in
`internal/object`; the `Eval` dispatcher itself in
`internal/interpreter`; and — built ahead of *its own* schedule, same
reasoning as `crust tokens`/`crust parse` for earlier phases —
`internal/builtins`, since without at least `deliver` there'd be no
way to see the interpreter do anything at all.

- **One recursive method**, `(*Interpreter).Eval(node ast.Node, env
  *object.Environment) object.Object`, switching on the Go type of
  `node` — no separate compile step. A method on an `Interpreter`
  struct (holding the builtin table) rather than a bare function or
  package-level state, specifically so the builtin table isn't global
  mutable state — each `crust run`, REPL session, or test gets its own
  `Interpreter`, bound to its own output writer via
  `interpreter.New(output io.Writer)`.
- `object.Object` interface: `Type() ObjectType`, `Inspect() string`.
  Concrete types mirror the runtime type list from Phase 1, plus
  `Function` (captures `Parameters`, `Body`, and the defining
  `*Environment`), `Builtin` (wraps a Go
  `func(args ...Object) Object`), `Error` (`Message` + `Line`/`Col`),
  and three internal control-flow signals never exposed to user code:
  `ReturnValue` (wraps a value bubbling up through nested blocks) and
  the `BREAK`/`CONTINUE` singletons for loop control.
- **Environment** (`object.Environment`): unchanged from the ahead-of-
  schedule Phase 4 groundwork — `Get`/`Set` implement `SPEC.md` §3's
  scoping rule exactly as designed, no changes needed once `Eval`
  actually started using it.
- **Only `recipe` calls create a new `Environment`** —
  `NewEnclosedEnvironment(outer)` runs on function call (in
  `applyFunction`), *not* on `order`/`combo`/`special`/`knead`/`bake`
  block entry, which all evaluate directly via the shared
  `evalBlockStatement` in whatever `Environment` the caller passed in.
- **Block evaluation is the one place control-flow signals get
  caught.** `evalBlockStatement` runs a block's statements in sequence
  and stops the moment one evaluates to `Error`, `ReturnValue`,
  `BreakSignal`, or `ContinueSignal` — bubbling that signal up
  unchanged rather than continuing. It's every *caller* of
  `evalBlockStatement` that decides whether to catch a signal or keep
  bubbling it: a loop (`evalCountedLoop`/`evalForEachLoop`/
  `evalBakeStatement`) catches `BreakSignal` (and stops, without
  running `knead`'s `Post` clause — matching C-family `break`) and lets
  `ContinueSignal` fall through to `Post` + the next condition check
  (matching C-family `continue`); `applyFunction` catches
  `ReturnValue` and unwraps it back to the plain value inside via
  `unwrapReturnValue`. A block that completes with no signal hit
  evaluates to `NULL` — cRust has no implicit last-statement-as-value
  the way Rust/Ruby do, which is also why a `recipe` that runs off the
  end without `serve` returns `nobox`, and a bare `serve` (no
  expression) wraps `NULL` explicitly rather than needing special-
  casing anywhere else.
- **Closures**: a `Function` object captures the `*Environment` active
  at its definition site (`evalFunctionLiteral`). Calling it
  (`applyFunction`) builds a new enclosed environment over that
  *captured* one (not the caller's) and evaluates the body in it —
  because `Set` walks the chain, a closure can mutate a variable from
  its defining scope just by assigning to it, no `global`/`nonlocal`
  equivalent needed. A *named* `FunctionLiteral` also binds itself into
  `env` under its name as a side effect of being evaluated — this, not
  a separate "recipe declaration" AST node, is what makes `recipe
  add(a, b) {...}` at statement level act like a declaration (see
  Phase 3's note on why there's no such node).
- **Wrong argument count is a runtime `Error`** (`want N, got M`) —
  exact arity is required; `SPEC.md` doesn't propose default/variadic
  parameters, so there's no partial-match case to design around.
- **Assignment/index-assignment/inc-dec all evaluate an index target's
  base and index expressions exactly once**, even for a compound op
  like `list[f()] += 1` — re-evaluating them for a "read current value,
  then write" sequence would run `f()` twice, which would be a real
  correctness bug (not just wasted work) the moment a base/index
  expression has any side effect via a call.
- **`RangeExpression`** evaluates both bounds, type-checks them as
  `Integer` (else an `Error`, `"range bounds must be Integers"`), and
  eagerly builds a `List` — no lazy Range object, matching `SPEC.md`
  §5.1 exactly. `.<` is `..` with the upper bound evaluated as
  `end - 1`; `start > ` the (already-adjusted) upper bound produces an
  empty List rather than an implicit reversal, covering both
  `5..1 → []` and the exclusive edge case `1.<1 → []`.
- **`UnpackAssignStatement`** evaluates the right-hand side once,
  type-checks it as a `List`, verifies it has at least `N - 1` elements
  for `N` targets, binds the first `N - 1` targets positionally, and
  binds the last target to a freshly-allocated `List` of whatever
  remains — even if that's empty, never a bare scalar.
- **`TernaryExpression`/`ElvisExpression`** evaluate lazily: exactly
  one of ternary's `Then`/`Else` runs (never both, and the untaken
  branch's AST subtree is never passed to `Eval` at all); Elvis
  evaluates `Right` only when `Left` evaluates to exactly
  `object.NULL` — a pointer-identity check against the `NULL`
  singleton, not a general truthiness check, so `thin ?: x` stays
  `thin` and `0 ?: x` stays `0`. Both chain correctly (right-
  associative else-if ladders, right-associative `?:`) purely from the
  parser's right-associative AST shape (Phase 3) — no extra evaluator
  logic needed for chaining itself.
- **`with`/`or`** also short-circuit (`evalLogicalExpression` peels
  them off before `InfixExpression`'s normal both-sides-eager path even
  evaluates `Right`), and always produce a strict `Boolean` rather than
  leaking through whichever operand's value decided the result —
  unlike Python's `and`/`or`. Decided this way specifically because
  cRust already has a dedicated "give me back the actual value, falling
  through on `nobox`" operator (Elvis) — `with`/`or` staying in the
  plainer "give me a Boolean" lane keeps the two forms from
  overlapping in confusing ways, and matches the language using
  explicit `stuffed`/`thin` rather than truthy-value passthrough
  anywhere else in its design.
- **Truthiness/coercion rules pinned down while implementing this**
  (now in `SPEC.md` §6, previously either unspecified or only implicit
  in earlier phases' examples): int/Float are one "number" category for
  **both** ordering *and* equality (`1 == 1.0` is `stuffed`, not just
  orderable against each other) — comparing an Integer against a Float
  exactly via `int64` when both happen to be Integer, to avoid
  `float64` precision loss on large values, widening only when at least
  one side is a Float; division by zero (`/`, `%`, or `idiv`, Integer
  or Float divisor) is always a runtime `Error`, not an implicit `Inf`/
  `NaN`; List/Map/Set indices are **not** negative-wrappable (`xs[-1]`
  is `"index out of range"`, matching how `SPEC.md`'s own examples
  compute the last index manually via `slices(list) - 1` instead — that
  phrasing only makes sense if `-1` doesn't already do it); reading a
  **missing Map key evaluates to `nobox`, not an Error** — required for
  `SPEC.md`'s own memoization idiom
  (`cache[n] = cache[n] ?: computeFib(n)`) to work at all, since `?:`
  needs something to fall through *from*; `Function` equality is by
  Go-level pointer identity (`SPEC.md` never proposes structural
  function equality, and there's no sensible definition of it beyond
  "the same closure").
- **Entry points** (`SPEC.md` §9, designed ahead of this phase existing
  to implement it): after evaluating every top-level statement, the
  `run` command looks for a top-level `recipe` named `store` or
  `store_<name>` and, if `--store=<name>` (or its absence, for the bare
  `store` case) resolves to one, calls it with zero arguments via the
  exported `(*Interpreter).Call` — there's no source-level
  `CallExpression` for an entry point invoked this way, so `Call` just
  runs `applyFunction` with a zero-value position. This is a
  `cmd/crust` + entry-point-resolution concern layered on top of
  `Eval`, not a new `ast`/`parser` concept — `store_part1` is just an
  ordinary named `recipe`, so nothing upstream of this phase needs to
  change to support it.

### Phase 5 — Standard Library (`internal/builtins`) 🚧
**Partially built ahead of schedule**, alongside Phase 4 — without at
least `deliver`, there'd be no way to see the interpreter produce
output at all. What exists: `Builtin` wraps
`func(args ...object.Object) object.Object` (variadic, not a slice
parameter — every builtin in `SPEC.md` §7 has a fixed, small arity, so
this reads more naturally at each call site than slicing `args`
manually); `internal/builtins.New(output io.Writer, stdin io.Reader)`
builds a fresh `map[string]*object.Builtin` per `Interpreter` rather
than a package-level table, so `deliver`'s destination and `unbox`'s
source aren't global mutable state — `interpreter.New` and
`cmd/crust`'s `run`/`runFile` thread both through explicitly (the same
`stdin io.Reader` `main()`'s `run()` already gained for `crust lsp`,
reused here rather than a second plumbing path). Identifier evaluation
checks the environment chain first, then falls back to this table —
builtins behave like predeclared globals that user code can still
shadow (confirmed by a test: `deliver = recipe(x) {...}` works). A
`Builtin`'s `Error` return has no position of its own
(`internal/builtins` doesn't know about source positions at all);
`applyFunction` patches the call site's position in after the fact if
one comes back unset, which is the one place that distinction matters.

Implemented now: `deliver`, `slices`, `sauce`, `chars`, `ints`, `idiv`
(mentioned in `SPEC.md` §6 as backing `/`'s "integer division is a
builtin" note, so it landed with the rest even though it's not yet in
§7's table), the full Set family
`gather`/`sprinkle`/`scrape`/`topped`/`combine`/`shared`/`strip`, input
(`unbox`/`lines`/`trim`), and type conversion
(`str`/`int`/`float`/`bool`). **Still not built**: general `strings`
helpers (split/join/contains/replace) and `math`/`sort` adapters
(abs/pow/sqrt/gcd/lcm, list sorting) — the rest of what this phase's
own section below describes.

- Most builtins are thin adapters over Go's standard library:
  `strings` (split/join/contains/replace, not yet built), `math`
  (abs/pow/sqrt/gcd/lcm, not yet built), `sort` (list sorting, not yet
  built). `unbox` wraps `os.ReadFile` (with a path argument) or reads
  the injected `stdin io.Reader` directly (no argument) — same-shaped
  `String` result either way, since a puzzle solution shouldn't care
  which source it came from. `lines` wraps `bufio.Scanner` with its
  default `ScanLines` split function specifically for its "no trailing
  blank entry for a string that ends in `\n`" behavior, rather than
  hand-rolling that edge case with `strings.Split`. `trim` wraps
  `strings.TrimSpace` — added right after input, once it became obvious
  cleaning up `unbox()`'s trailing newline was the actual first thing
  most callers would need to do with its output. Deliberately named
  `trim`, not `strip`: `strip(a, b)` (Set difference) already existed
  in §7, and overloading one name across two unrelated operations
  (whitespace trimming vs. Set difference) would have been confusing
  regardless of which one got there first.
- **Type conversion is explicit-only, by design, not by omission.**
  `SPEC.md` §6 already states operators never implicitly convert
  between types (`+` between a String and a number is a type error);
  `str`/`int`/`float`/`bool` exist so a conversion is still possible,
  just always as a visible function call rather than something that
  could happen silently inside `+` or an `order` condition. `int`
  parsing a non-integer string (`int("3.5")`) is a runtime error rather
  than silently truncating — `float(x)` first, then `int(...)` that, if
  truncation genuinely is what's wanted; `int(aFloat)` truncates toward
  zero (a real, deliberate cast), which is a different, intentional
  operation from parsing malformed input. `bool(x)` doesn't add a new
  truthiness rule — it exposes the exact one `order`/`bake`/ternary
  already use (`SPEC.md` §6) as a value instead of only a branch
  decision, so nothing about "what counts as falsy" needed two separate
  definitions to keep in sync.
- **`ints(s)` is `chars(s)`'s digit-grid counterpart**, added once real
  usage made clear `chars` alone doesn't cover the common AoC shape of
  a line of digits meant to be summed/compared/walked as numbers
  (heightmaps, calorie-digit puzzles) — splitting into single-character
  Strings and then having to `int()` each one individually was the
  alternative, and `ints` just does that in one call. Same one-rune-
  at-a-time split as `chars`, but each rune must be `'0'`-`'9'` — a
  non-digit character is a runtime error, not silently skipped or
  mapped to its raw code point, matching `int(x)`'s own "malformed
  input is an error, not a guess" stance above.
- The names already locked in — see `SPEC.md` §7 — are `deliver` (print),
  `slices` (length, replacing a generic `len`), `sauce` (nil-coalesce:
  `value` or a `fallback` if `value` is `nobox`), `chars`/`ints`
  (string → List of characters/digits), the Set builtins
  `gather`/`sprinkle`/`scrape`/`topped`/`combine`/`shared`/`strip`,
  `unbox`/`lines`/`trim` (input), and `str`/`int`/`float`/`bool`
  (conversion). These names were chosen specifically because dropping
  `topping`/`sauce` as declaration
  keywords (Phase 1 revision) freed them up to mean something more
  useful as functions — `sauce` in particular reuses the "base layer
  under everything else" metaphor for a fallback value; `unbox` extends
  the same "pizza box" metaphor `nobox` already established (§4) to
  "here's what's actually inside," which read better than forcing a
  second, unrelated pun onto plain file/stdin reading. Math and
  conversion builtins keep their standard names on purpose; see
  `SPEC.md` §7 for why.

### Phase 6 — Tooling (`cmd/crust`)
- CLI has two modes: `crust run <file>` (parse + eval one file, exit,
  now real — see Phase 4) and `crust repl` (interactive loop, still a
  stub). Kept intentionally minimal — manual flag handling is enough;
  no need for a CLI framework dependency.
- `crust run <file> [--store=<name>]` (and the bare-file shorthand,
  `crust <file> [--store=<name>]`) selects which `store`/`store_<name>`
  recipe the file's entry point resolves to (`SPEC.md` §9) — omitted,
  it selects the bare `store`, if the file has one; a file with no
  `store`-family recipe at all just runs top-to-bottom, unaffected by
  the flag. `parseRunArgs` hand-parses this instead of using
  `flag.FlagSet`, specifically so the flag can appear before *or* after
  the file path — Go's stdlib flag parsing stops at the first
  non-flag-looking argument, which would silently fail to parse
  `crust run day01.crust --store=part1` (flag after the positional).
- The REPL reuses the exact same `Lexer` → `Parser` → `Eval` pipeline as
  file execution, holding one persistent `*object.Environment` across
  lines so variables/functions defined earlier stay in scope. Still not
  built — the natural next step now that `Eval` exists, but out of
  scope for the same turn that built `Eval` itself; the main open
  design question is how much (if any) multi-line construct support
  (a `recipe`/`order`/`knead` body spanning several typed lines) a v1
  needs versus deferring to single-line-only for now.
- Error messages throughout stay in the pizza theme (tone, not
  mechanism) — the underlying `Error` object/position reporting is the
  same regardless of wording.
- **Editor syntax highlighting** (`editors/`) — Vim/Neovim
  (`editors/vim`: `syntax/crust.vim`, `ftdetect/crust.vim`,
  `ftplugin/crust.vim`) and VSCode (`editors/vscode`: a TextMate
  grammar plus a minimal extension manifest). Both are regex-based
  (Vim's own syntax matching; TextMate scope patterns for VSCode) —
  neither editor's *default* highlighting path needs a real parser, and
  writing one (a Tree-sitter grammar, which Neovim can also consume) is
  substantial, separate-project-sized work tracked as a further stretch
  in `TODO.md` rather than attempted here. Both files were verified
  against the real tokenizer, not just read off the docs: the Vim
  syntax file against an actual headless Vim instance
  (`vim -Nu NONE -es`, checking `synID()` at specific columns of a test
  file exercising every token category); the VSCode grammar against
  `vscode-textmate` + `vscode-oniguruma` — the exact same tokenizer
  engine VSCode itself runs, driven from a small Node harness rather
  than eyeballing the grammar JSON.
  - **Two real bugs only surfaced under that testing**, both about
    match-priority ordering, and in *opposite* directions between the
    two engines — worth recording since the "obvious" fix for one is
    wrong for the other:
    1. Vim resolves two syntax items that could both start matching at
       the same buffer position by picking whichever was **defined
       last** in the syntax file (`:help syn-priority`). The `//` line
       comment rule was defined *before* the arithmetic-operator rule
       (which includes bare `/` for division), so `crustOperator`'s
       single-character `/` match silently won at a comment's opening
       `//`, and the comment span never got claimed at all — not just
       miscolored, entirely unrecognized. Fixed by moving the comment
       rule to be the *last* one defined in the file, specifically so
       it wins that tie-break. The same "last-defined-wins" rule also
       bit the number literals: `crustFloat` needs to be defined
       *after* `crustInteger`, or `5.5` shows only its leading `5` as
       an Integer instead of the whole thing as a Float.
    2. TextMate/VSCode grammars go the other way: the tokenizer tries
       each pattern in a `patterns` array **in list order** at the
       current scan position and takes the first one that matches, so
       the equivalent fix there is to list `comments` *first* — which
       is what this grammar already did, and testing confirmed it
       needed no change. Both `syntax/crust.vim` and
       `syntaxes/crust.tmLanguage.json` now carry an inline comment
       explaining which direction their own engine's priority rule
       runs, specifically so this doesn't get silently un-fixed by
       someone applying the other engine's mental model while editing.
  - `deliver`/`slices`/and the rest of the builtins (`SPEC.md` §7) only
    highlight as builtins when actually called (a `(?=\s*\()`
    lookahead in the VSCode grammar; ordinary `syn keyword` in Vim,
    which doesn't have an equivalent lookahead concept but doesn't need
    one — an identifier just being *named* `deliver` elsewhere, e.g.
    `deliver = recipe(x) {...}` shadowing it per `SPEC.md` §4, isn't
    something Vim's simpler keyword matching would get wrong either
    way). Confirmed the VSCode side specifically, since that grammar's
    lookahead is the part that could plausibly have been written wrong.
  - `(|`/`|)`'s literal-paren cosmetic limitation (`SPEC.md` §5.3's own
    note) applies to both editors' *generic* bracket-matching the same
    way it would to any editor — nothing in either syntax file
    resolves it, since doing so would need real grammar awareness
    neither engine has by default; both READMEs call this out
    explicitly rather than let someone discover it and assume it's a
    bug in the highlighting.
- **Tree-sitter grammar for Neovim** (`editors/tree-sitter-crust`) — a
  real CFG (`grammar.js`), not a token-pattern list like the two
  regex-based grammars above; this is what gives Neovim
  parser-driven highlighting (also the foundation `nvim-treesitter`
  builds incremental selection and structural text objects on, though
  only highlighting is provided here). Written for **accurate
  highlighting**, not as a second validating implementation of the
  language — it deliberately doesn't model `SPEC.md` §8's
  `terminator = NEWLINE | ";"` precisely, treating newlines as
  insignificant whitespace instead of a real statement terminator
  (modeling that exactly needs an external scanner tracking
  significance, the way `tree-sitter-python` tracks indentation — real
  work, out of proportion to what highlighting needs). `;` is still
  handled explicitly as an optional separator, so semicolon-joined
  one-liners still parse correctly; only the newline half of the rule
  is simplified away, and it doesn't cost anything in practice — every
  real file under `examples/` still parses with zero `ERROR`/`MISSING`
  nodes, one statement per line, same as always.
  - Verified in layers, same testing discipline as the two regex
    grammars above: `tree-sitter generate` produces no unresolved
    conflicts; `tree-sitter test` passes all 23 cases in
    `test/corpus` (every operator's precedence/associativity, both
    `knead` header forms, the block-vs-map-literal disambiguation,
    `serve` followed by a map literal vs. a separate following block,
    semicolon one-liners); every real `examples/*.crust` file parses
    clean; the highlight query's pattern-matching was checked with
    `tree-sitter query`, and its *resolved* (post-override) output
    with `tree-sitter highlight --html` against genuinely overlapping
    cases (a declared function's name correctly overrides the generic
    `@variable` fallback, a builtin call overrides both `@variable`
    and `@function.call`); the generated `src/parser.c` was actually
    compiled to a `.so` (`cc -shared -fPIC`) and confirmed via `nm -D`
    to export `tree_sitter_crust`, the exact symbol Neovim's built-in
    loader looks up via `dlsym`. **Not verified**: an actual Neovim
    instance loading that `.so` and rendering colors — there's no
    Neovim binary in the environment this was built in, so that one
    link in the chain rests on every earlier, independently-verified
    link being right rather than being exercised directly itself.
  - **A second match-priority surprise, in the opposite direction from
    the Vim syntax file's bug above** — worth recording next to that
    one specifically because the "obvious" fix for one is wrong for the
    other, and this project already got bitten by exactly that
    confusion once in the same session: tree-sitter query files (like
    TextMate grammars, unlike Vim's `syn match`) resolve two patterns
    matching the *same node* by letting the *later*-listed pattern win.
    `queries/highlights.scm` lists the generic `(identifier) @variable`
    fallback *first* and every more specific override (`@function` for
    a declared name, `@function.builtin` for a builtin call,
    `@variable.parameter` for a parameter) *after* it, for exactly that
    reason — confirmed correct via the `tree-sitter highlight`
    resolved-output check above, not assumed from having just fixed the
    Vim file's opposite-direction version of this same category of bug.
  - `grammar.js` needed a handful of explicit `conflicts` declarations
    tree-sitter's GLR engine couldn't resolve on its own — found by
    running `tree-sitter generate` and reading its conflict reports,
    not anticipated in advance: `block` vs. a bare `map_literal`
    statement (the same statement-position ambiguity the Vim/VSCode
    grammars don't have to resolve, since they're not real parsers);
    `serve` followed immediately by `{` (is that `serve <map-literal>`,
    or a bare `serve` followed by a separate block statement? — resolved
    toward the former, matching how the real interpreter treats
    anything after `serve` other than a terminator as its value); and
    `_lvalue` vs. a bare identifier expression at `identifier ++`/
    `identifier =` (same "is this the start of an assignment target or
    an expression statement" ambiguity `internal/parser` needs explicit
    lookahead for, which tree-sitter's GLR parsing handles by exploring
    both and pruning instead). One redundant grammar rule (`recipe`
    declarations were reachable both as a direct top-level `_statement`
    alternative and, separately, via `_simple_statement → _expression`,
    since `function_literal` was already one of `_expression`'s
    alternatives) caused a conflict that was fixed by deleting the
    redundant alternative rather than adding another `conflicts` entry
    — not every conflict tree-sitter reports needs a resolution rule;
    some mean the grammar itself has an actual redundancy worth
    removing.
- **Language server** (`internal/lsp`, wired up as `crust lsp`) — hover
  and diagnostics over JSON-RPC 2.0 on stdio, hand-rolled against the
  spec rather than built on a third-party LSP library. That's a
  deliberate consequence of this project's zero-Go-dependency policy
  (see §3's Nix section — `vendorHash = null` in `flake.nix` only stays
  valid with no third-party modules to vendor), not a rejection of
  existing LSP libraries on their own merits.
  - **Subcommand, not a separate binary.** `crust lsp` reuses the
    existing `cmd/crust` build/distribution path (the Nix flake's
    `packages.default`, `nix develop .#crust`, etc.) with zero changes
    — a `crust-lsp` binary would need its own build target, its own
    place in the flake, and its own install story for exactly the same
    functionality.
  - **Transport** (`transport.go`): `Content-Length`-framed JSON-RPC
    over `io.Reader`/`io.Writer`, matching LSP's base protocol exactly.
    `Server.Run` reads until EOF or an `"exit"` notification; one
    malformed message logs to `stderr` (LSP reserves `stdout` for
    protocol traffic only) and the session keeps going, rather than
    the whole server dying on one bad message.
  - **Diagnostics** (`diagnostics.go`): lexes and parses the document
    fresh on every `didOpen`/`didChange` — no incremental
    re-lex/re-parse, the same "start over each time" approach
    `crust tokens`/`crust parse` already use, since AoC-sized `.crust`
    files make that cheap enough not to matter. Sources errors from two
    places: every lexer `ILLEGAL` token, and `internal/parser`'s new
    structured `ParseError`/`ParseErrors()` (added specifically for
    this — `parser.Errors() []string` already existed for the CLI's
    `crust parse`, but a pre-formatted `"line %d:%d: msg"` string is
    useless to a caller that needs a real `Range`). A lexer `ILLEGAL`
    token almost always cascades into a parser error at that exact same
    position too (the parser has no prefix-parse function for
    `ILLEGAL`); `computeDiagnostics` drops any `ParseError` landing on a
    position already reported as `ILLEGAL`, so a single bad character
    doesn't show up as two confusing, redundant diagnostics.
  - **Hover** (`hover.go`): deliberately shallow — static lookup tables
    mirroring `SPEC.md` §4 (keywords) and §7 (builtins), plus a
    one-line type note for `INT`/`FLOAT`/`STRING` literal tokens.
    Finds the token under the cursor by lexing fresh and walking
    forward to the last token starting at or before the requested
    column on that line (cRust tokens never span multiple lines, which
    is what makes this a single-line walk instead of needing a real
    token index). No type inference over the surrounding expression, no
    evaluating user code as a side effect of hovering — both explicitly
    out of scope, since either would need a much deeper static-analysis
    pass this project doesn't have a use case to justify yet.
  - **Position encoding** (`position.go`): negotiated in `initialize`
    rather than hardcoded, because `internal/lexer`'s `Col` field is a
    rune (code point) index — which lines up exactly with LSP's
    `"utf-32"` `PositionEncodingKind` and *not* with `"utf-16"` (LSP's
    documented default absent negotiation) or `"utf-8"`, both of which
    need a real per-rune width calculation (`unicode/utf16`'s
    `RuneLen`, or `unicode/utf8`'s `RuneLen`) to convert correctly for
    any line containing non-ASCII text — plausible in a cRust string
    literal even if never in the keyword/operator vocabulary itself.
    `negotiateEncoding` follows the spec precisely: the client lists
    `capabilities.general.positionEncodings` in its own preference
    order, and the server picks the first one it supports, rather than
    always preferring whichever encoding is cheapest for the server to
    produce.
  - **Verified against a real subprocess**, not just Go's own test
    suite: `go build -o crust ./cmd/crust`, then a real Python harness
    spawned `crust lsp` and drove it through
    `initialize`/`initialized`/`didOpen` (a file with a deliberate
    illegal `@` character)/`hover`/`shutdown`/`exit` over actual
    stdin/stdout pipes, confirming byte-for-byte correct
    `Content-Length` framing, a `publishDiagnostics` notification for
    the bad character, and a correct hover response for `deliver` —
    the same "don't just trust the code, run it against something
    real" discipline this project applied to the Vim/VSCode/tree-sitter
    highlighting work above. `internal/lsp`'s own Go tests
    (`lsp_test.go`) build on the same idea in-process: real
    `bytes.Buffer`s framed exactly like wire traffic, decoded back with
    the package's own `readMessage`, rather than calling handler
    methods directly and skipping the transport layer entirely.
  - **Definition, references, rename, documentSymbol, and completion**
    (`symbols.go`, `definition.go`) extend the same server with real
    scope-aware name resolution, added after a real user hit
    Neovim's generic `<leader>D` (`vim.lsp.buf.type_definition`)
    keymap against `crust_ls` and got "method not supported" — the
    right fix wasn't `typeDefinition` (cRust is dynamically typed, so
    there's no static type-declaration site to jump to) but a genuine
    `textDocument/definition`, which then made the rest of this group
    a natural, low-incremental-cost extension. `symbols.go` walks the
    parsed AST once per request into a `fileIndex`: every identifier
    occurrence, tagged as a declaration (recipe name, parameter,
    assignment/unpack target, for-each loop variable) or a plain
    reference, each carrying which lexical scope it belongs to.
    Scoping mirrors `internal/interpreter`'s real model exactly
    (SPEC.md §3 — only `recipe` calls get their own scope;
    `order`/`knead`/`bake` blocks share the enclosing one, and a
    recipe's own name is declared in its *enclosing* scope, not
    inside itself, matching how the interpreter's `Environment.Set`
    actually binds it) — scopes form a parent chain for closures, so
    `resolve(name, scope)` walks outward exactly the way a real
    closure lookup would, converging on the nearest enclosing
    declaration rather than a same-name text match. This is real
    lexical resolution, not the "deliberately shallow" scope hover.go
    and completion below stay within — a same-named parameter in two
    unrelated functions correctly resolves to two different
    declarations, and `textDocument/references`/`rename` only touch
    the one actually being asked about.
    - **`textDocument/definition`**/**`references`**: find the
      identifier token exactly under the cursor (`identTokenAt` — a
      stricter span-containment match than hover's lenient
      "nearest token at or before the cursor," since jumping
      somewhere unintended is worse here than hover simply doing
      nothing), resolve it via the scope chain, and for references
      return every occurrence whose *own* resolution lands on that
      same declaration.
    - **`textDocument/rename`**: identical resolution to references,
      turned into a `WorkspaceEdit` — one `TextEdit` per occurrence,
      always including the declaration site (a rename that left the
      declaration untouched wouldn't be a rename).
    - **`textDocument/documentSymbol`**: every recipe declaration,
      `SymbolKind.Function`. `Range` and `SelectionRange` are both
      just the name's own token span rather than the whole
      declaration — `internal/ast`'s `BlockStatement` doesn't carry
      its closing `}`'s position, so there's no cheap way to report a
      full body span; outline/go-to-symbol behavior doesn't need more
      than the name span to work correctly.
    - **`textDocument/completion`**: keywords (from
      `internal/token`'s own keyword table, now exported as
      `Keywords()` specifically so this didn't need a second
      hand-maintained copy of the spelling list) + builtins (the same
      `builtinDocs` table hover.go already had) + every declared name
      in the whole document. Deliberately **not** resolved against the
      cursor's actual lexical scope, unlike definition/references/
      rename above — `internal/ast` has no block-end position to
      determine "which scope is this blank cursor position inside,"
      the way it does for an *existing* identifier's own scope
      (recorded naturally while walking the tree). Whole-document
      scoping is occasionally over-inclusive (offering a name that's
      technically out of scope at the cursor) but never hides a real
      completion, which is the safer direction to be wrong in for a
      suggestion list.
    - Verified against the real subprocess harness described above,
      extended to definition/references/documentSymbol/completion/
      rename in the same run: confirmed byte-for-byte correct
      `Location`/`WorkspaceEdit`/`DocumentSymbol`/`CompletionItem`
      JSON shapes, not just that Go's own JSON marshaling round-trips
      — plus `internal/lsp`'s Go test suite gained real closure- and
      shadowing-specific cases (two functions with identically-named
      parameters; a nested recipe's body correctly resolving a name
      declared only in its enclosing recipe) that a plain
      same-name-string search would get wrong.
    - **`textDocument/typeDefinition`** is aliased straight to the
      same handler as `textDocument/definition` rather than left
      unhandled. This is the direct fix for the bug report that
      started this whole group of features: Neovim's stock
      `LspAttach` keymaps bind `<leader>D` to
      `vim.lsp.buf.type_definition` unconditionally for every
      attaching client, and a server with no handler at all for that
      method makes Neovim report "method ... not supported by any of
      the servers registered for the current buffer" the moment
      someone presses it. cRust has no separate type-declaration
      syntax to point at in the first place (no classes/structs —
      just recipes and the builtin value kinds), so there's no
      meaningful distinction between "where was this declared" and
      "where was this value's type declared" to preserve by
      implementing the two differently; aliasing is the same
      practical choice a number of real servers for dynamically-typed
      languages make, rather than either leaving the method
      unhandled or building a second, redundant resolution path.
  - Not built: incremental (as opposed to full) document sync, and
    code actions — listed as possible follow-on work in `TODO.md`.

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
| CLI shape | `main()` → `run(args, stdin, stdout, stderr) (code int)`, not `os.Exit`/`os.Stdout`/`os.Stdin` sprinkled through the logic | Makes the CLI unit-testable (`main_test.go`) without subprocess spawning; `stdin` was added specifically for `crust lsp`, the first subcommand that actually reads it |
| Nix packaging | `flake.nix` via `buildGoModule`, no `flake.lock` committed yet | Builds from source, so it's consistent with "no release workflow" rather than a separate distribution channel; the lock file needs a real Nix install (network access this dev environment doesn't have) to generate correctly |
| LSP implementation | Hand-rolled JSON-RPC/LSP in `internal/lsp`, no third-party LSP library | Keeps the zero-Go-dependency policy intact, which is what keeps `flake.nix`'s `vendorHash = null` valid; the protocol subset `crust lsp` actually needs (lifecycle, hover, diagnostics) is small enough that this doesn't cost much |
| LSP distribution | A `crust lsp` subcommand, not a separate `crust-lsp` binary | Reuses the existing build/package/Nix-flake path entirely — no new binary to build, version, or install |
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
