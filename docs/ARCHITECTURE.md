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
    correct value on a mismatch. *(That moment arrived in Phase 6 —
    `crust develop`'s TUI needed bubbletea/lipgloss; see the Go
    dependency policy row in §4's design-decisions table.)*
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
    never has to handle a non-`Hashable` element itself. This is what
    makes `Tuple` safely usable as a `Map` key or `Set` element
    (`sprinkle(seen, (x, y))` for grid-coordinate dedup, the single
    biggest practical reason to want this over a `List`, which can
    never be hashed since its contents can change after insertion).
    - **Real bug, live for a while: `Tuple`-as-`Map`-key didn't
      actually work**, despite this doc and `SPEC.md` §2.3 both
      claiming it did from the moment `Tuple` landed. `isValidMapKey`
      (`expressions.go`) — the gate `evalMapLiteral`, map-literal
      index-reads, and map index-*assignment* all call independently —
      still special-cased `String`/`Integer` only, a rule written
      before `Tuple` (or `Float`/`Boolean` `HashKey`) existed and never
      revisited when they were added. `Set` never had this problem:
      `evalSetLiteral` and `sprinkle`/`gather` all just check
      `object.Hashable` directly, no allowlist, so `toppings{(x, y)}`
      worked from day one — only `Map` quietly lagged behind its own
      documentation. Found via the grid-utilities session, the first
      time anyone actually tried `m[(row, col)] = value` for a sparse
      "infinite" grid (the standard AoC pattern for simulations that
      grow past a fixed `grid()`/`setAt()` array's bounds). Fixed by
      making `isValidMapKey` just check `object.Hashable`, matching
      `Set`'s rule exactly — `Map` and `Set` are both backed by the
      same interface, so a Map accepting a narrower set of key types
      than Set accepts as elements was never a design anyone chose on
      purpose, just documentation that outran an early implementation
      detail nobody went back to fix. `String, Integer, Float, Boolean,
      Tuple` — precisely `Hashable`'s implementers — can all be Map
      keys now, end to end (literal, read, assignment), with
      regression tests at the interpreter layer covering all three
      access paths plus a same-shape `Float`/`Boolean` key check.
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
    `object.Equal`/`equalSlices` (factored to take `[]object.Object`
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
- **Environment** (`object.Environment`): `Get`/`Set` are unchanged from
  the ahead-of-schedule Phase 4 groundwork and implement `SPEC.md` §3's
  scoping rule exactly as designed — `Set` walks outer scopes looking
  for an existing binding to mutate, which is what makes closures
  mutate a captured variable and loop accumulators work with no
  `global`/`nonlocal` keyword. A third method, `Declare`, was added
  later (found via a real bug — see below) specifically for the one
  case that must *never* walk outward: binding a recipe's parameters.
- **Only `recipe` calls create a new `Environment`** —
  `NewEnclosedEnvironment(outer)` runs on function call (in
  `applyFunction`), *not* on `order`/`combo`/`special`/`knead`/`bake`
  block entry, which all evaluate directly via the shared
  `evalBlockStatement` in whatever `Environment` the caller passed in.
  - **Real bug, found while writing the grid-simulation example
    (`examples/grid_life.crust`)**: `applyFunction` originally bound
    each parameter with `extEnv.Set(param.Value, args[idx])`. `Set`'s
    whole job is walking outer scopes to find and mutate an existing
    binding — exactly right for a closure reassigning a variable it
    captured, but for a *parameter* it meant a recipe whose parameter
    happened to share a name with an outer (often global) variable
    would silently overwrite that outer variable the moment the
    parameter got reassigned inside the function body. Minimal repro:
    `recipe touch(g) { g = 999 }` `g = 1` `touch(g)` `deliver(g)`
    printed `999`, not `1` — but only when `g` lived in global scope;
    the same call from inside another recipe's own local scope
    (nothing named `g` anywhere in the outer chain) worked correctly by
    accident, since `Set` had nothing existing to find and fell through
    to creating a fresh local. That inconsistency (works when nested,
    breaks at global scope) is what made it a real, surprising bug
    rather than a documented restriction. Fixed by adding
    `Environment.Declare(name, value)` — an unconditional local bind,
    no outer walk, ever — and switching `applyFunction`'s parameter
    loop to use it; every other `Set` call site (`knead`'s loop
    variable, plain assignment, unpack targets) is untouched, since
    those all correctly operate on an existing, non-fresh scope where
    the walk-and-mutate rule is exactly what SPEC.md §3 asks for.
    Regression-tested at both layers: `object.TestEnvironmentDeclareShadowsExistingOuterBinding`
    isolates `Declare` itself, and
    `interpreter.TestParameterShadowsSameNamedOuterVariable` reproduces
    the original global-scope failure end-to-end.
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

Implemented now: `deliver`, `slices`, `sauce`, `chars`, `ints`, `push`,
`map`, `idiv` (mentioned in `SPEC.md` §6 as backing `/`'s "integer
division is a builtin" note, so it landed with the rest even though
it's not yet in §7's table), the full Set family
`gather`/`sprinkle`/`scrape`/`topped`/`combine`/`shared`/`strip`, input
(`unbox`/`lines`/`split`/`join`/`trim`), type conversion
(`str`/`int`/`float`/`bool`), and `+`-as-concatenation extended from
strings to Lists and Tuples. **Still not built**: `contains`/`replace`,
`filter`/`reduce` (`map`'s siblings), and `math`/`sort` adapters
(abs/pow/sqrt/gcd/lcm, list sorting) — the rest of what this phase's
own section below describes.

- Most builtins are thin adapters over Go's standard library:
  `strings` (contains/replace, not yet built), `math`
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
- **`split` overloads on arity rather than needing two names** — the
  same `unbox()`/`unbox(path)` pattern. `split(s)` wraps
  `strings.Fields` (runs of whitespace, no empty entries — the "just
  give me the words" behavior most languages' no-argument split has);
  `split(s, delim)` wraps `strings.Split` (literal delimiter,
  preserving empty entries between consecutive delimiters, so
  `"a,,b"` on `","` is 3 elements, not 2 — real CSV-style parsing needs
  that, unlike the whitespace form). An empty `delim` is a runtime
  error rather than falling through to `strings.Split`'s own
  per-rune-boundary behavior for that case, since `chars(s)` already
  owns "split into individual characters" and having two builtins
  quietly do the same thing under different names would be confusing,
  not convenient.
- **`join(list, sep)` is `split`'s counterpart**, wrapping
  `strings.Join`, and deliberately doesn't call `str()` on non-String
  elements itself — `join(someInts, ", ")` is a runtime error naming
  the offending index and type, not a silent stringify. Same
  "conversion is always explicit" stance as the `str`/`int`/`float`/
  `bool` note below, just enforced at a second call site instead of
  only inside `+`.
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
  - **`ints(list)` is a second, deliberately different overload**,
    added once real usage showed `ints(split(line))` — a line of
    whitespace-separated, possibly multi-digit numbers — was just as
    common a shape as a bare digit string. Each element is parsed as a
    *whole* Integer (`strconv.ParseInt`, the same as `int(x)`'s own
    String case), not digit-by-digit — `ints(["12", "345"])` is
    `[12, 345]`, not five single digits. The two overloads answer "this
    string IS a sequence of digits" vs. "this list holds separate
    numerals" — different enough questions that giving them different
    parsing rules under one name is the right call, not an
    inconsistency to paper over.
- **`push(list, item)` mutates in place; `+` never does.** Both exist
  because they answer different questions — "add this one item to the
  List I already have" (`push`, the List counterpart to `sprinkle`'s
  in-place Set insert) vs. "give me a new List/Tuple that's the
  concatenation of these two" (`+`, extended in
  `internal/interpreter/expressions.go`'s `evalArithmetic` from
  strings — already concatenation-on-`+` — to Lists and Tuples the
  same way, same-type-only). Keeping `+` non-mutating matters
  specifically because compound assignment (`a += b`) is implemented
  generically as `a = evalInfixOperator("+", a, b)`
  (`internal/interpreter/statements.go`): if `+` mutated its left
  operand in place, `a += b` would double-apply the change (once via
  the mutation, once via the rebind) for a List the way it never would
  for a String. Tuple concatenation needs no extra hashability check
  the way `evalTupleLiteral` does for a fresh literal — both operands
  are already-constructed Tuples, so every element was already proven
  `Hashable` when *they* were built.
- **`map(iterable, fn)` is the first builtin that needs to call back
  into the interpreter** — every builtin before it only ever produced
  or inspected `object.Object` values, never *invoked* one, so nothing
  before this needed a way to call a user-defined `*object.Function`
  (that needs `internal/interpreter`'s environment/`Eval` machinery,
  not just the `object` package). Direct import isn't possible —
  `internal/interpreter` already imports `internal/builtins`, and Go
  disallows the reverse — so the fix is dependency injection: a new
  `builtins.Call` type (`func(fn object.Object, args []object.Object)
  object.Object`) is threaded through `builtins.New`'s new third
  parameter, and `interpreter.New` supplies its own `i.Call` method
  value as that argument. This does mean `interpreter.New` has to
  construct the `*Interpreter` before it can finish building the
  builtin table (`i := &Interpreter{}` first, *then*
  `i.Builtins = builtins.New(output, stdin, i.Call)`), since the method
  value needs a real receiver to bind to — safe because `Call` only
  ever reads `i.Builtins` when something actually invokes `map` at
  *program* runtime, long after this one-time constructor call
  returns, never during construction itself. `mapFn` itself is
  otherwise a plain "iterate a List/Tuple, call fn on each element,
  collect into a new List, short-circuit on the first error" loop — no
  special-casing beyond that; a `*object.Builtin` argument (`map(xs,
  str)`) and a `*object.Function` argument (`map(xs, recipe(x)
  {...})`) both just flow through the same injected `call`. Only one
  function at a time, deliberately: chaining more than one transform
  per element is already possible by passing a lambda that does both
  (`map(xs, recipe(x) { serve g(f(x)) })`), so `map` doesn't also need
  to accept a List of functions to pipeline — that would just be a
  second, redundant spelling of something closures already do.
  `internal/builtins`' own tests can't easily exercise the
  `*object.Function` half of this (constructing one needs
  `internal/ast`/`internal/object.Environment` wiring that's really
  `internal/interpreter`'s job) — a `fakeCall` test helper that can
  still invoke a `*object.Builtin` directly covers `map`'s own
  iterate-and-collect logic in isolation; the real closures-through-`map`
  behavior (including a lambda that closes over an outer variable, and
  a lambda composing two steps like `ints(split(line))`) is covered by
  `internal/interpreter`'s own test suite instead, where a real `Call`
  is naturally available.
- **`find(collection, value)` is `contains`'s positional counterpart —
  "is `value` here" vs. "where is `value`."** First built as a
  predicate search (`find(iterable, fn)`, JS's `Array.find` shape,
  reusing `map`'s injected `Call` to invoke a user function per
  element and returning the first element it accepted) on the
  reasonable-sounding original request "a library function that
  searches for an element in a list and returns the one at the lowest
  index that matches." That wording turned out to mean something more
  literal: a plain value to search for (not a predicate function) and
  the *index* as the answer (not the element) — corrected immediately
  once the actual call shape was spelled out (`find(collection,
  value_of_element_to_find)`), before the predicate version had
  anything built on top of it. The corrected `find` no longer calls
  back into user code at all — it's a linear scan compared with
  `object.Equal`, sharing `indexOfElement` with `contains`'s own
  List/Tuple case (one definition of "where does this value live in
  this collection," not two that could quietly disagree) — and only
  accepts List/Tuple, the same restriction `contains` doesn't share
  (a Set has no position to report, and a Map's iteration order isn't
  meaningful the way an index promises it is). `nobox` on no match or
  an empty collection, matching every other "nothing here" result in
  the language (`at`'s out-of-range read, a missing Map key).
  `object.IsTruthy`'s relocation out of `internal/interpreter` (from
  the predicate version's needs) stayed even after `find` itself
  stopped needing it — `order`/`bake`/ternary/`hold`/with/or's call
  sites are already on the relocated version, and there was no reason
  to move them back.
- **`min`/`max` accept two shapes** — `min(a, b, ...)` (2+ direct
  arguments) or `min(list)` (a single List/Tuple) — the same
  "iterable-or-its-unpacked-elements" convention `map`/`ints` already
  established, rather than picking just one. Ordering reuses exactly
  the categories `internal/interpreter/expressions.go`'s `evalComparison`
  already defines for `<`/`>` (SPEC.md §6): numbers with Integer/Float
  freely mixed, or String-vs-String, never a cross-category comparison.
  That logic can't be imported directly — same one-way dependency
  restriction as `map`'s `Call` type above — so `numericValue` and a
  local `compareTwo` helper duplicate just enough of it by hand inside
  `internal/builtins`, rather than pulling in the whole operator
  dispatch. The winning element is returned as-is, not converted, so
  `max(1, 2.5)` gives back the actual `Float` value `2.5` rather than a
  widened copy — same "no implicit coercion" stance as everywhere else
  (SPEC.md §6). An empty List/Tuple is a runtime error (`min([])` has no
  answer), and comparing across categories (`min(1, "x")`) is too, the
  same way `1 < "x"` already is.
- **`pizzasort(list)` delegates the actual sorting to Go's own
  `slices.SortFunc` rather than a hand-rolled algorithm** — the request
  was for "whatever sorting method is smartest," and three approaches
  were pitched before building anything: hand-roll an introsort
  (quicksort + insertion sort for small partitions + a heapsort
  fallback to bound the worst case); detect input shape up front and
  pick a specialized algorithm per shape (e.g. counting sort for a
  narrow range of integers); or reuse Go's own sort, which — since Go
  1.19 — already *is* pattern-defeating quicksort (pdqsort): insertion
  sort for small partitions, a heapsort fallback bounding the worst
  case at O(n log n), and cheap detection of already-sorted/
  reverse-sorted/many-duplicate-key input. The third option won,
  specifically because it delivers everything "smartest" implies
  without cRust owning and debugging a sort implementation of its own —
  reusing well-tested machinery over hand-rolling it is the same
  instinct that already shaped `min`/`max` (below) and the equality/
  truthiness relocation two builtins ago. `slices.SortFunc` isn't
  guaranteed stable, but stability only matters when equal-comparing
  elements are still distinguishable by something else (a secondary
  key) — natural order has none: two equal numbers or two equal Strings
  are genuinely interchangeable, so there's nothing for stability to
  preserve here, and the faster unstable sort costs nothing observable.
  A follow-up question ("should sorting take a custom comparator or key
  function, for descending order or sorting by a derived value?") was
  raised and explicitly deferred — natural-order-only shipped first,
  with a comparator-function variant noted as a real, larger follow-on
  if it's ever asked for (it would need the same `Call`-injection
  `map`/`find` already use, not just `compareTwo`).
  - **Reuses `compareTwo` directly rather than adding a second
    comparison helper** — the exact ordering rule (numbers freely
    mixed, Strings lexicographic, cross-category is an error) `min`/
    `max` already needed is the same one sorting needs, and it was
    already sitting in `internal/builtins` with nowhere else to import
    it from. Every element is checked against a single shared reference
    (`elements[0]`) once, before sorting starts, rather than trusting
    `slices.SortFunc`'s own comparator closure to surface a
    mid-algorithm error — `compareTwo` only ever succeeds within one of
    exactly two categories, so each element comparing successfully
    against one reference transitively proves every element shares a
    category with every other, and the actual sort pass never needs to
    re-check or handle an error path at all.
  - Always returns a new List, whether the input was a List or a
    Tuple — the same convention `map`/`combos`/`enumerate` already
    settled on, and the only one that even makes sense here, since a
    Tuple can't be sorted in place (it's immutable) and a List
    shouldn't have its identity/contents silently mutated by a function
    that reads like it's just answering a question, not performing an
    action.
- **`combos(list, n)` generalizes to any n instead of shipping separate
  `pairs`/`triples` functions** — the request that motivated it was
  specifically "every combination of n elements," and hardcoding a
  couple of small cases would just mean writing this same
  odometer-style index-advance loop again the day someone needs `n=4`.
  Combinations, not permutations: `combos([1,2,3], 2)` is `[(1,2),
  (1,3), (2,3)]`, never `(2,1)` as well — matching `itertools.combinations`
  in Python, a well-known-enough shape that it didn't need its own
  design discussion beyond confirming "every element used at most once,
  order within a group doesn't matter." Each combination comes back as
  a Tuple (fixed-size, hashable — useful as a Set element or Map key,
  e.g. deduplicating pairs already seen), which means every source
  element has to satisfy the same Hashable requirement
  `evalTupleLiteral` already enforces for a literal `(a, b)` — checked
  once against the source List/Tuple up front, not per generated
  combination, since the same elements recur across every group. `n`
  bigger than the source's length isn't an error, just an empty result
  (there aren't that many elements to pick from); `n < 0` is, since
  there's no such thing as a negative-size group. `n == 0` is the one
  edge worth calling out explicitly: it's not an error and not an empty
  List either — it's a List containing exactly one element, the empty
  Tuple `()`, matching the standard math convention that there's
  exactly one way to choose nothing.
- **`enumerate(list)` exists so an enumerated loop doesn't need its own
  syntax** — the request was "an easy way to make an enumerated loop,"
  and cRust's `knead item in collection` only ever binds one loop
  variable (`ast.ForEachLoop.Identifier` is a single `*Identifier`, not
  a list of them), so `knead i, x in enumerate(xs)` isn't something the
  parser accepts today. Extending the loop header to bind multiple
  names would be the more ambitious fix; pairing a new builtin with the
  tuple-unpack assignment sugar that already exists (§3.1) was the
  smaller, immediately-available one, and got picked for exactly that
  reason — `knead pair in enumerate(xs) { i, x = pair; ... }` is one
  extra line, not a grammar change. Each pair comes back as a Tuple, not
  a two-element List, and that choice isn't arbitrary: List-unpack's
  rule (SPEC.md §3.1) is "the last target catches everything left over
  as its own List," which exists for a variable-length remainder
  (`a, *rest`-style) — apply that same rule to a fixed 2-element pair
  and `i, x = [0, "a"]` binds `x` to `["a"]`, not the bare `"a"` a
  reader would expect. A Tuple's exact-arity unpack doesn't have that
  trap. The cost is the same Hashable requirement `combos` above
  already has to enforce, and for the identical underlying reason:
  `Tuple.HashKey()` does an unchecked type assertion on each element
  (`internal/object/tuple.go`), so an unhashable one wouldn't error
  gracefully, it would panic — but only the moment the pair actually
  got used as a Set element or Map key, which could be far from where
  the bad Tuple was built. Checking eagerly, right where the Tuple is
  constructed, turns a delayed Go panic into an immediate, accurately-
  located cRust runtime error — worth the restriction that
  `enumerate()` can't pair positions with a List/Map/Grid value
  directly (an ordinary counted loop, `knead i in 0.<slices(xs)`,
  already covers indexing into one of those).
- **`contains(collection, item)` generalizes `topped` from "Set
  membership" to "membership in whatever you've got"** — the request
  was specifically "an `in` keyword for dictionaries and lists and
  sets," and a full `in` infix expression (`item in collection ->
  Boolean`) was the first option offered; a builtin function was picked
  instead once asked, since it needed no parser changes at all
  (`in` stays exactly where it already was, `knead item in
  collection`'s loop header, rather than gaining a second grammatical
  role). List/Tuple membership is a linear scan compared with
  `object.Equal` — the same notion of "equal" `==` uses, so
  `contains(xs, y)` agrees with `xs[i] == y` for whichever `i`; Set
  reuses `topped`'s own `Set.Has` (O(1) via `Hashable`, not
  re-implemented); Map membership means "is `item` a *key*", not a
  value, matching Python's `k in dict` convention and reusing `Map.Get`
  the same way `sauce`/nobox indexing already does.
  - **This is what moved the interpreter's `==`/`!=` equality logic
    (`objectsEqual` and its `gridsEqual`/`listsEqual`/`mapsEqual`/
    `setsEqual` helpers) out of `internal/interpreter` and into
    `internal/object` as an exported `object.Equal`.** `contains()`
    needed the exact same value-equality rule `==` already implements
    (Lists/Tuples/Maps/Sets/Grids compare contents, Integer/Float are
    one number category, cross-category is always false) — and
    `internal/builtins` can't import `internal/interpreter` to reach
    the existing private function (the dependency runs the other way:
    the interpreter imports builtins for the `Call` callback `map()`
    needs, so the reverse import would cycle). Reimplementing the same
    tree of cases a second time in `internal/builtins` was the
    alternative, and got rejected specifically because it's exactly the
    kind of duplication that drifts silently — a future fix to how,
    say, Grid equality handles offsets would need to land in both
    copies, and nothing would fail loudly if it only landed in one.
    Moving the real implementation to `internal/object` (both
    interpreter and builtins already depend on it) and having
    `evalInfixOperator`'s `==`/`!=` cases call `object.Equal` instead
    means there is exactly one implementation of "equal" in the whole
    interpreter, not two that happen to agree today. No behavior
    changed — every existing `==`/`!=` test still passes unmodified —
    this was a pure relocation, verified by keeping the full existing
    equality test suite green throughout.
- **`copy(value)` only recurses into List elements, Map values, and
  Grid cells — Set and Tuple copies stop at one level, and Integer/
  Float/String/Boolean/Null/Tuple pass through unchanged.** The
  request was "a copy function so items don't continue being modified
  by reference," motivated by List/Map/Set/Grid all being Go pointer
  types with reference semantics (assignment/passing shares the same
  underlying object; `push`, `setAt`, index assignment, `sprinkle`/
  `scrape` all mutate through every existing reference). The design
  fell out of an invariant already established for a different reason:
  every Tuple element (and, by the same rule, every Set element and
  every Map key) must be `Hashable` (§Phase 3 above, `combos`/
  `enumerate`'s design notes) — and List/Map/Set/Grid don't implement
  `Hashable`. That means a Tuple, or a Set's members, or a Map's keys,
  can never hold a mutable container in the first place, so they're
  already as independent as a copy could make them; only List
  elements, Map *values*, and Grid cells can hold arbitrary Objects
  (including another List/Map/Set/Grid) and are where `copy` actually
  has to recurse. `object.DeepCopy` lives in `internal/object` (not
  `internal/builtins`) for the same one-way-import reason `object.Equal`
  and `object.IsTruthy` do — nothing about `copy` itself needed that
  move this time, it's just where the previous two builtins already
  established the pattern lives, and `internal/builtins`' `copyFn` is a
  thin one-line wrapper over it. A naive recursive copy would loop
  forever on a self-referential structure (`xs = [1]; push(xs, xs)`),
  so `deepCopy` carries a `seen map[Object]Object` from original to
  its already-built copy, registering each new container immediately
  after allocating it and before recursing into its contents — the
  same memo technique as Python's `copy.deepcopy`. That memo has a
  second effect beyond cycle-safety: two references to the same
  original sub-container inside one copy operation end up pointing at
  the same new copy, so whatever internal aliasing the original had is
  preserved rather than silently duplicated. Verified via a real
  `crust run` subprocess: independence for a plain List, a List nested
  inside a List, a List value inside a Map, and a List cell inside a
  Grid; a self-referential List copies without hanging; a Tuple and a
  scalar both come back unchanged.
- **`list[start..end]` slice notation reuses the existing `..`/`.<`
  range operators inside `[...]` instead of adding new grammar** — the
  request offered two shapes explicitly ("cRust range-flavored" vs.
  Python's `:`/`::step`), and the range-reuse option won without
  needing to ask, since it's a strictly smaller change that fits an
  instinct already established by `pizzasort`, `find`/`contains`, and
  the equality/truthiness relocations: prefer what the language
  already has over growing the grammar. It also turned out to need
  *zero* parser/AST changes at all — `index = "[" expression "]"`
  (§8) already accepts any expression, `..`/`.<` were already
  registered as infix operators at `RANGE` precedence (above `LOWEST`,
  which is what `index`'s inner expression parses at), so
  `xs[0..4]` was already parsing as `IndexExpression{Index:
  RangeExpression{...}}` before this feature existed — the only gap
  was that `evalIndexExpression` had no idea what to do with a
  `RangeExpression` as an index, and would eagerly materialize it into
  a List of Integers via the *existing* `evalRangeExpression` (meant
  for a bare `1..5` range statement) and then reject that List as "not
  an Integer" the same way any other non-Integer index would. The fix
  was entirely in `internal/interpreter`: detect `*ast.RangeExpression`
  in `evalIndexExpression` before evaluating it, and route to a new
  `evalSliceExpression` that evaluates Start/End itself rather than
  letting `evalRangeExpression` build a throwaway List first.
  - **Negative bounds and reversed direction are a deliberate
    departure from how a bare range statement behaves, not an
    oversight.** A negative slice bound resolves against the
    container's length (`-1` is the last element, Python's
    convention) before anything else, so the rest of the logic only
    ever sees plain non-negative positions. Direction then follows
    whichever way the *resolved* bounds point: `start <= end` walks
    forward exactly like `evalRangeExpression` already does, but
    `start > end` walks backward instead of coming back empty the way
    an out-of-order bare range does (`5..1` is `[]`, but
    `xs[-1..0]` is asked to mean "last element back to the first") —
    the request was explicit about this being the point of adding
    negative bounds at all, so an empty result would have missed what
    was actually asked for. `.<` drops whichever end the walk is
    currently heading toward (the upper end going forward, the lower
    end going backward), which is the direction-agnostic reading of
    "exclusive of end" a plain range's own `.<` already has.
  - **Out-of-range bounds are a runtime error, not a silent clamp** —
    Python slicing famously never errors (`[1,2,3][0:100]` just comes
    back `[1,2,3]`), but every other cRust indexing operation
    (`readIndex`'s plain `list[i]`) already errors loudly on
    out-of-bounds, and introducing the one indexing operation that
    fails silently would be a surprising, hard-to-spot exception to
    that rule rather than a feature worth having.
  - **List/Tuple/String are the only sliceable types, matching
    `readIndex`'s own set of position-indexable types** — Set was
    already unindexable (`readIndex` rejects it) and stays that way;
    Map indexes by key, not position, so "a Map slice" isn't a
    coherent idea; Grid indexes by `(row, col)` Tuple via `at`/`setAt`,
    a different enough shape that folding it into linear start/end
    slicing wouldn't actually mean anything.
  - **A sliced Tuple skips re-validating the Hashable-element
    invariant** `evalTupleLiteral` enforces on construction — a subset
    of an already-all-Hashable Tuple's elements is still all-Hashable
    by construction, so `object.NewTuple` is called directly on the
    sliced elements rather than routing back through whatever
    Hashable-checking a literal would do.
  - **No step component** (`list[start..end:step]` or similar) — the
    request's own examples were start/end only, and a step is a
    materially bigger feature (it needs its own operator or syntax
    slot, and interacts with negative-direction reversal in a way that
    would need its own design pass) that wasn't asked for. Deferred
    as a stretch, the same way `pizzasort`'s comparator-function
    variant was.
  - Verified via a real `crust run` subprocess across List/Tuple/String:
    inclusive and exclusive forward slices, a negative-bound reversed
    slice, a negative-bound forward slice, a single-element slice both
    inclusive (one element) and exclusive (empty), a backward-exclusive
    slice, that slicing doesn't mutate or alias the original, an
    out-of-range bound erroring cleanly, and a Set correctly rejected
    with "does not support slicing."
- **`list(x)`/`tuple(x)`/`set(x)` are the general collection-conversion
  builtins, filling the gap `gather` deliberately left** — the request
  was "a way to convert collections to other collection types," and
  the survey that answered it first turned up partial coverage that
  already existed by accident: `gather(list)` only ever went List →
  Set (never Tuple → Set or Set → anything), and `map`/`pizzasort`
  happen to always return a List regardless of whether a List or
  Tuple went in, which is a usable Tuple → List trick but not a named,
  discoverable one. The three new builtins round that out
  symmetrically — each accepts any of List/Tuple/Set and normalizes to
  its own target type — named to match the existing `str`/`int`/
  `float`/`bool` conversion builtins' convention of keeping standard
  names rather than a pizza pun (SPEC.md §7's own rationale for those
  four applies here without change).
  - **A shared `asElements(x) ([]Object, bool)` helper extracts a
    List/Tuple/Set's elements once**, used by all three, rather than
    each repeating its own three-case type switch — a deliberate step
    up from the two-case (List/Tuple only) switch duplicated inline
    across `map`/`pizzasort`/`combos` elsewhere in this file: three
    near-identical copies of a three-armed switch (List/Tuple/Set,
    where the Set arm also has to drain a Go map into a slice) crossed
    the line from "small enough to just repeat" to "worth naming."
    Always returns a fresh slice, never the input's own backing
    storage, so `list(xs)`/`tuple(xs)`/`set(xs)` can hand that slice
    straight to a new container without the new container quietly
    aliasing the old one's memory.
  - **A Set's element order is whatever Go's own map iteration
    produces** — arbitrary, and not guaranteed stable between two
    calls on the same Set — which is a faithful reflection of Set
    being unordered (SPEC.md §2.2) rather than a limitation; there was
    never a "correct" order to preserve converting *out* of a Set.
  - **`list`/`set` on their own type still builds a new container,
    not the same object back** — `list(xs)` for a List `xs`, or
    `set(s)` for a Set `s`, is a shallow copy (new backing
    storage/map, same element references), the same convention
    Python's own `list()`/`set()` conversion functions use. This is
    deliberately *not* what `copy()` (above) does — `copy()` goes
    deep (recursing into nested containers) specifically to break
    reference-sharing all the way down, while `list()`/`set()` just
    need to guarantee the *outer* container is independent, which is
    all a same-type "conversion" ever promised.
  - **`tuple`/`set` both reject an unhashable element with the same
    message shape `gather` already used** (`"<name>: unhashable
    element of type %s cannot go in a <Type>"`) rather than each
    inventing its own wording — matches how a `(a, b)` Tuple literal
    and a `toppings{...}` Set literal already fail on the same input.
  - Verified via a real `crust run` subprocess: List↔Tuple↔Set in
    every direction (including Set→List and Set→Tuple, the two paths
    that had no route at all before this), duplicate-dropping on
    `set()`, `list()`'s shallow-copy-not-mutation behavior, an
    unhashable-element error from `tuple()`, and a wrong-type error
    for a non-collection argument.
- **`freq(x)` reuses `asElements` a fourth time** — the request was
  literally "a `freq(l)` function that gives me a dictionary of the
  frequency of items in a list," Python's `collections.Counter` by
  another name. Built on the same `asElements(x) ([]Object, bool)`
  helper `list`/`tuple`/`set` already share (above), so it accepts any
  of List/Tuple/Set the same way they do rather than being List-only —
  a Set argument is a legal, if not especially interesting, case
  (every count comes back 1, since a Set's own members are already
  unique) rather than one worth rejecting just because it's unlikely
  to be what anyone actually calls `freq` on. Every element must be
  Hashable to become a Map key, same requirement (and the same
  `"freq: unhashable element of type %s cannot be a Map key"` message
  shape) as `gather`/`tuple`/`set` already use for their own unhashable
  case. The count itself is tracked by reading a key's current value
  back out with `Map.Get` before each `Map.Set` (0 if the key hasn't
  been seen yet) rather than a separate Go-side counting map kept
  alongside the result — one map, no second data structure to keep in
  sync with it. Verified via a real `crust run` subprocess: counts
  across repeated Strings and Integers, a Tuple argument, a Set
  argument (all 1s), an empty List (empty Map back), and an
  unhashable-element error.
- **`keys(m)`/`values(m)` sort by each key's `Inspect()` text instead
  of just handing back whatever order Map's own backing Go map
  happens to visit** — the request was literally "a keys and values
  function for dictionaries," and the obvious first implementation
  (each function doing its own `for _, pair := range m.Pairs`) has a
  real correctness trap Go's own map semantics create: iteration order
  over a `map[K]V` is randomized *per range statement*, not fixed per
  map, so two separate range loops over the very same map — which is
  exactly what `keys(m)` and a later `values(m)` call would each be —
  aren't guaranteed to visit entries in the same relative order.
  Silently, `keys(m)[i]` and `values(m)[i]` could end up as two
  different original pairs, which wouldn't be obvious from either call
  alone; it would only surface once someone actually zipped the two
  Lists together by index (`knead i in 0.<slices(ks) { deliver(ks[i],
  vs[i]) }`, the obvious reason to want both functions in the first
  place) and got nonsense. Fixed by routing both through a shared
  `sortedMapPairs(m) []MapPair` helper that sorts by
  `pair.Key.Inspect()` — a plain function of `m`'s *current contents*,
  not of when or how many times something ranges over it, so the two
  calls agree by construction rather than by luck. The sort order
  itself is arbitrary (lexicographic over each key's text form, so
  Integer keys `2`/`10` sort as strings, not numerically) and not
  claimed to be meaningful on its own — the only property that
  actually matters is that `keys`/`values` agree with each other every
  time. `knead k in m` (already existing, SPEC.md §8) still iterates a
  Map's keys directly for an ordinary loop; `keys`/`values` are for the
  specific case of wanting real Lists back, e.g. to zip by index or
  pass to `map`/`pizzasort`. Verified via a real `crust run`
  subprocess, including running the same program 3 times in a row
  (to catch exactly the kind of flake `sortedMapPairs` exists to
  prevent) confirming `keys(m)[i]`/`values(m)[i]` always agreed with
  direct `m[keys(m)[i]]` lookups; a Go test
  (`TestKeysAndValuesCorrespondAcrossSeparateCalls`) makes the same
  check part of the permanent suite rather than relying on manual
  reruns alone.
- **Grid support started as five composable functions over plain
  List-of-List, not a dedicated Grid type** — until `setAt` needed to
  auto-expand instead of erroring (below), at which point it *became*
  a dedicated type after all. Both stages are kept here rather than
  overwriting the first with the second, since the reasoning that led
  from one to the other is the useful part. The request that motivated
  the original version was "grid simulation functionality" — broad
  enough that it was worth asking what shape was actually wanted (a
  full cellular-automaton stepper vs. just the pieces to write one)
  before building anything; the answer was utilities only, so a
  `step(grid, rule)`-style function stayed out of scope then and still
  hasn't been built. `grid(s)` parsed a String into a row-major List of
  row-Lists of one-character Strings (`lines` + `chars`, combined into
  one call); `at(g, pos)`/`setAt(g, pos, value)` were bounds-checked
  read/write; `neighbors4(pos)`/`neighbors8(pos)` give the 4 or 8
  neighbor positions of a cell. Every position is a `(row, col)` Tuple
  — chosen specifically because it's hashable (SPEC.md §2.3), so a
  position can go straight into a `Set` (visited cells, already how
  `gather`/`sprinkle`/`topped` work) or a `Map` key (distances, costs)
  with no extra packing, and it composes cleanly with `combos`/`map`.
  Tuple positions and `neighbors4`/`neighbors8` are entirely unchanged
  by everything below — only the grid value itself and `at`/`setAt`
  changed.
  - **`at` reads out-of-range as `nobox`, deliberately breaking from
    plain `g[row][col]` indexing (which errors via `readIndex`)** — the
    same reasoning `readIndex`'s own doc comment already gives for a
    missing Map key reading as `nobox` instead of erroring. Grid code
    constantly asks "is there a cell here" for a candidate neighbor
    near an edge; `nobox` turns that into a plain equality/`?:` check
    instead of a hand-written bounds check before every lookup — e.g.
    `knead n in neighbors8(pos) { v = at(g, n); order (v == "#") {
    count += 1 } }` never needs to check `n` is in range first, since
    `at` already answers "no" as `nobox`, which just isn't `"#"`.
  - **`setAt` originally went the other way: out-of-range was a runtime
    error**, matching plain `g[row][col] = value` rather than `at`'s
    nobox-on-miss, on the reasoning that writing off the edge of a grid
    is a bug to surface immediately. That held until a user actually
    hit it mid-simulation (their own neighbor-walk output included
    negative coordinates like `(0, -1)`) and asked for `setAt` to
    expand the grid instead of erroring — see the dedicated Grid-type
    note directly below for what changed and why "expand" turned out
    to need more than a one-line fix.
  - **Why "expand" couldn't just mean "grow the List a bit": a plain
    List-of-List has nowhere to remember a shifted origin.** Growing
    positively (appending rows/cols) is easy with plain Lists, but the
    motivating case was negative — the user's own neighbor-walk output
    included coordinates like `(0, -1)`, i.e. `setAt` needs to succeed
    when the new cell is *before* row/col 0. That only works if
    `(0, -1)` keeps meaning the same physical cell on every later call,
    which means somewhere has to remember "this grid's logical row 0 is
    now at backing-storage row 1" — state a bare `[][]Object` has no
    field for. That gap is what promoted Grid from "five functions over
    List-of-List" to `object.Grid` (`internal/object/grid.go`): a struct
    holding the backing rows plus `RowOffset`/`ColOffset`, so
    `Set`/`Get` can translate a logical `(row, col)` into a backing-array
    index (`row + RowOffset`, `col + ColOffset`) and the offset survives
    across calls.
  - **`Grid.Set` on an out-of-range position rebuilds the whole grid into
    a new bounding box rather than prepending/appending incrementally**
    — it computes the union of the current bounds and the new position,
    allocates a fresh `[][]Object` sized to that box, copies every
    existing cell to its shifted position, writes the new value, and
    updates `RowOffset`/`ColOffset` to match. That's O(new area) per
    expanding write instead of O(1), but grid expansions are rare
    relative to in-bounds writes in a typical simulation loop (most
    `setAt` calls land inside the existing box once a simulation is
    running), and AoC-sized grids make the difference unmeasurable in
    practice — consistent with this project's standing preference for
    the simplest-correct approach at this input scale (see the general
    performance philosophy note elsewhere in this doc) over a fancier
    incremental-resize scheme.
  - **Grid is deliberately not directly indexable (`g[row]`) or iterable
    (`knead row in g`)** — both were considered and rejected. A raw
    index would have to mean either "position in backing storage" (which
    silently changes meaning after any expansion shifts the offset —
    exactly the bug the offset exists to prevent) or would have to
    redo the same `row + RowOffset`/`col + ColOffset` translation `at`
    and `setAt` already do, just duplicated at a second call site.
    `at`/`setAt`/`gridBounds` are the entire interface on purpose.
    `gridBounds(g)` fills the resulting gap — since nothing else exposes
    a Grid's current extent, it returns `(minRow, minCol, maxRow,
    maxCol)` as a Tuple (or `nobox` for an empty grid), which is what
    lets `examples/grid_life.crust`'s `step()` sweep every cell without
    knowing the grid's dimensions ahead of time.
  - **`setAt` now requires an actual `*object.Grid`, not a plain List**
    — the old List-based `setAt` path was deleted outright rather than
    kept as a fallback, since a plain List has no offset field for
    `setAt` to update on an expanding write. `at`, by contrast, still
    accepts either a `*object.Grid` or a plain List-of-List/Tuple-rows
    (checking for Grid first, falling back to the legacy path) — reading
    doesn't need persistent offset state, so old code that built a grid
    by hand (e.g. via `map()`) without going through `grid()`/`newGrid()`
    still works with `at`.
  - **Grid equality (`==`/`!=`) compares offset *and* contents, not just
    visible layout** — two grids with identical cell values but
    different `RowOffset`/`ColOffset` are *not* equal, because the same
    coordinate Tuple would resolve to a different cell on each of them;
    treating them as equal would make `==` lie about what a subsequent
    `at(g, pos)` returns. This follows the same "compare full internal
    state" convention already used for Map/List equality elsewhere in
    the interpreter. Grid is also deliberately **not** `Hashable` (like
    List/Map/Set, and for the same reason — it's mutable), so it can't
    be a Map key or Set element.
  - `newGrid()` (0-arg, returns an empty `&object.Grid{}`) exists for
    simulations that start from a handful of live cells rather than a
    block of text — e.g. `grid_life.crust`'s `step()` builds each
    generation by calling `setAt` into a fresh `newGrid()`, letting the
    grid grow to fit exactly the cells that get written instead of
    pre-sizing it.
  - Re-verified the whole Grid type end-to-end via real `crust run`
    subprocesses, not just Go unit tests: expansion in the positive
    direction, the negative-direction case that motivated all of this
    (`setAt` at `(0, -1)` on an empty grid), a grid that expands in both
    directions across several calls with earlier cells checked to still
    read back correctly afterward, and the full rewritten
    `grid_life.crust` (vertical blinker → horizontal blinker, same
    result as before the type existed).
  - **`neighbors4`/`neighbors8` do zero bounds checking against any
    particular grid** — they're pure `(row, col)` arithmetic, so a
    neighbor of `(0, 0)` can come back as `(-1, 0)`. That's
    deliberate: baking a grid parameter into these functions just to
    filter would make them less composable (what if you're computing
    neighbors for a Set of visited positions instead of a grid at
    all?), and the nobox-on-miss behavior above already gives a clean
    way to filter against a real grid when that's what's wanted. Both
    functions list their offsets in the same row-major order over the
    3×3 neighborhood (`neighbors4` is exactly `neighbors8`'s four
    non-diagonal entries in the same relative order), so the two agree
    on "which direction comes first" wherever they overlap — arbitrary
    but worth pinning down once so it's actually documented/testable
    behavior rather than "whatever the loop happens to produce."
  - Verified this whole set actually composes into a real simulation,
    not just individually: a from-scratch Game-of-Life single-step
    (`countLiveNeighbors` using `neighbors8`+`at`, then a `knead`-based
    row/col sweep using `setAt`) run against a 3-cell vertical blinker
    produced the correct horizontal-blinker output in one generation,
    via a real `crust run` subprocess — not just Go unit tests.
- The names already locked in — see `SPEC.md` §7 — are `deliver` (print),
  `slices` (length, replacing a generic `len`), `sauce` (nil-coalesce:
  `value` or a `fallback` if `value` is `nobox`), `chars`/`ints`
  (string → List of characters/digits), `push` (in-place List append),
  `map` (apply a function across a List/Tuple), `find` (lowest matching
  index), `copy` (independent deep copy), `min`/`max`, `pizzasort` (natural-order sort),
  `combos` (n-element combinations), `enumerate` (index/value pairs),
  `grid`/`newGrid`/`at`/`setAt`/`gridBounds`/`neighbors4`/`neighbors8`
  (2D grid support), the Set builtins
  `gather`/`sprinkle`/`scrape`/`topped`/`combine`/`shared`/`strip`,
  `list`/`tuple`/`set` (collection conversion), `freq` (occurrence counts),
  `keys`/`values` (Map keys/values as Lists),
  `contains` (general List/Tuple/Set/Map membership, `topped`'s
  broader counterpart), `unbox`/`lines`/`split`/`join`/`trim` (input),
  and `str`/`int`/`float`/`bool` (conversion). These names were chosen specifically because dropping
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
    - **Real bug, caught by a real user**: `builtinDocs` is a
      hand-maintained copy of `internal/builtins`' own table, and nine
      builtins added to `internal/builtins` after this package was
      first built (`ints`, `push`, `unbox`, `lines`, `split`, `join`,
      `trim`, `str`/`int`/`float`/`bool`) had silently gone
      undocumented in both hover and completion (`completionsAt`
      reuses the same map) — `crust` itself worked fine, but hovering
      or autocompleting any of them in an editor showed nothing. Fixed
      by syncing the table, and by a new test
      (`TestBuiltinDocsCoversEveryRealBuiltin`, `hover_test.go`) that
      diffs `builtinDocs`'s keys against `builtins.New()`'s real
      key set directly — not another hand-maintained list, which
      would only move the staleness risk rather than remove it — so
      the next builtin added anywhere in `internal/builtins` fails
      this test immediately instead of silently shipping undocumented.
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

#### Debugger (`internal/trace`, `internal/debugger`, `crust develop`)

Concept and much of the tree-building shape borrowed from a similar
step-by-step debugger in another interpreter project
(RFuller25/domainlang's `visualize` command), at the user's explicit
request to "borrow some concepts." What carried over unchanged, and
what had to be redesigned, both come down to one underlying
difference: that project's language is a value *pipeline* — every
construct is a stage with one input value and one output value, so
"watch the data change shape" is a direct, uniform description of
every step. cRust is a general imperative language — a `knead` loop
has no value of its own, an `order` picks a branch rather than
producing one, and an assignment's interesting fact is *which name*
got a new value, not an anonymous output. That's why the design here
is *statement*-shaped rather than value-shaped throughout, discussed
with the user (a real design conversation, not assumed) before any
code was written.

- **`internal/trace`** defines the hook: a `Tracer` interface
  (`Step(StepEvent)`, `PushFrame(label)`, `PopFrame()`) and
  `StepEvent{Node ast.Statement, Out object.Object, Dur time.Duration}`.
  No `Depth`/`Frame` field on `StepEvent` — a `Tracer` receives
  `Step`/`PushFrame`/`PopFrame` calls in the actual order they happen,
  so a tree-building `Tracer` already knows its own nesting from that
  call sequence alone, and carrying the same fact again on every event
  would be a second, redundant way to ask "how deep am I." No `Err`
  field either: cRust represents a runtime failure as an ordinary
  `object.Object` value (`*object.Error`, never Go's `error`
  interface — see `internal/interpreter`'s `isError`), so a failed
  step's `Out` already *is* its error.
  - `internal/object.Environment` gained no changes for this; the
    `Interpreter.Trace trace.Tracer` field is nil by default, and every
    call site checks it directly rather than through a wrapper —
    `evalTracedStatement` (per-statement `Step` reports, wraps
    `evalBlockStatement`'s and `evalProgram`'s loops) and `evalFramed`
    (per-call/per-lap `PushFrame`/`PopFrame`, wraps `applyFunction`'s
    body eval and the three loop forms' body eval) both short-circuit
    to a plain `i.Eval(...)` when `i.Trace == nil`.
  - **A real zero-cost claim, checked, not just asserted.** The first
    version of the loop/call frame-label code built each label
    (`fmt.Sprintf("knead (...) lap %d", lap)`, `ce.Function.String()+"(...)"`)
    *unconditionally*, before ever checking whether tracing was even
    on — so an untraced `crust run` was still paying for a
    `Sprintf` allocation on every single loop lap and function call.
    Caught by actually benchmarking against a git-worktree checkout of
    the pre-tracing commit (`BenchmarkUntraced` vs. a throwaway
    baseline benchmark in a `git worktree add` of the prior commit),
    not by reasoning about the code: allocs/op were 19100 vs. baseline
    15892 — a real, measurable regression the doc comments claimed
    didn't exist. Fixed by guarding each label computation behind its
    own `if i.Trace != nil` at the call site, after which allocs/op
    matched the baseline exactly (15892 = 15892) across three runs
    each. `BenchmarkTraced`/`BenchmarkUntraced` in
    `internal/interpreter/trace_test.go` keep this checkable going
    forward (`go test -bench . -benchmem ./internal/interpreter`) —
    there's no automated pass/fail threshold (that would be flaky,
    keyed to whatever machine runs it), just numbers a human can read.
  - **`ast.Statement` gained a `Pos() token.Token` method** (added to
    the interface itself, not a type-switch helper) so the recorder can
    attach a line number to any statement kind uniformly. A type switch
    would silently return a zero-value position for any statement kind
    added later without remembering to update it; putting `Pos()` on
    the interface makes the Go compiler enforce it — a new `Statement`
    type that forgets `Pos()` fails to build, not fails silently at
    runtime. All 12 concrete statement types implement it as a one-line
    `return x.Token`.
- **`internal/debugger`** is the consumer: `Recorder` (implements
  `trace.Tracer`, builds a bounded tree — recipe calls and loop laps
  are frames a reader can step into) and `Timing` (a self/total-time
  pass over that tree, plus KPI aggregation).
  - **The tree-building algorithm is close to a direct port**: a
    frame's `PushFrame`/`PopFrame` calls both happen *during* the step
    that opened it, so a closed frame is held in a pending list at the
    level that opened it until the step that produced it reports, at
    which point it becomes that step's child. Loop laps fold into one
    collapsed-but-still-explorable row past `foldFrom` (3) consecutive
    same-shaped frames, so a thousand-lap loop doesn't bury the program
    around it. What didn't carry over: the source project's `adopt()`
    step (collapsing a child frame that merely restates its owner's own
    label — needed there because a pipeline stage's body frame is
    always named identically to the stage itself) and `number()`
    (numbering same-labeled sibling frames after the fact) — cRust's
    loop-lap labels already embed their own lap number at the point
    they're built (`evalForEachLoop` etc.), and a step's label is never
    equal to a frame's label in the first place, so neither
    post-processing step has anything to do here.
  - **A real bug, found by actually running `crust develop` against a
    file**, not just from unit tests: `Roots()` originally treated
    *any* frame left in the pending list as evidence of an incomplete
    recording (a step cap hit mid-body, or — in the source project — an
    uncaught panic) and wrapped it in a misleading
    `"(incomplete — ...)"` row. But `crust develop`'s own entry-point call
    (`interp.Call`, invoked directly from `cmd/crust/debug.go`'s Go
    code, never through a *traced statement*) opens exactly this kind
    of never-adopted frame on every single ordinary, successful run —
    a program's whole logic almost always lives inside its `store()`
    entry point. Fixed by keying the distinction on `r.truncated`
    instead of "is anything pending at all": a pending frame is only
    genuinely incomplete when the step cap was actually hit; otherwise
    it's just a frame that happened to open outside the traced-
    statement machinery, and belongs as an ordinary root.
    Regression-tested (`TestRecorderUnclaimedFrameIsAnOrdinaryRootWhenNotTruncated`)
    alongside the still-real truncation case
    (`TestRecorderIncompleteRunKeepsOrphanedFrames`).
  - **A second real display bug, same discovery method**: `Step.Label()`
    originally returned a statement's raw `String()` — correct for
    round-tripping source, wrong for a one-line table row, since a
    recipe declaration's or a loop's `String()` renders its *entire*
    multi-line body. `crust develop`'s plain-text table broke badly on
    this (a "row" that was actually eleven lines of source text, with
    every column after it shifted). Fixed by keeping only the first
    line and marking that there's more
    (`"recipe fib(n) { …"`) — `TestStepLabelCollapsesMultiLineStatementsToOneLine`
    pins it.
  - **KPIs bucket by frame *family*, not by call site.** The source
    project's own hotspot ranking keys by the call-site AST node
    (right for "which line is slow" — a legitimate question, and the
    one a profiler usually answers), but the user's actual ask was
    "time spent **per function**." A recursive `fib(n-1)`/`fib(n-2)`
    is two different call-site nodes; bucketing by node identity would
    split one recipe's cost across as many rows as it has call sites
    in the source, not the one row "time spent per function" implies.
    `family()` (`recorder.go`) collapses a frame's display label down
    to its recurring-thing key — stripping the trailing `" lap N"`
    that every loop-lap label embeds (a recipe-call label never
    contains it, so it passes through unchanged) — and `Timing.measure`
    switches to that family when descending into a frame's children,
    so every call to one recipe, at any recursion depth, and every lap
    of one loop, lands in a single `KPI` bucket. Verified with an
    actual recursive `fact`/`fib`: `TestTimingKPIsGroupRecursiveCallsIntoOneBucket`
    asserts a `fact(5)`'s five recursion depths produce `Calls == 5`
    in one bucket, and the real `crust develop` demo run against a
    `fib(0..7)` summed-in-a-loop program produced exactly 100 calls in
    one `fib(...)` bucket — the correct closed-form sum
    (`fib(0)+fib(1)+...+fib(7)` call counts), a number worth checking
    by hand precisely because getting the bucketing wrong would still
    produce *a* plausible-looking number, just the wrong one.
  - **Self vs. total time**, and KPI `SelfSize`/`SelfTime`, follow the
    same "self time is what a profile should rank by" reasoning as the
    source project (a `knead` loop at 98% total isn't a slow loop, it's
    a hundred laps of whatever's inside it) — `NodeTiming.Self` is
    `Total` minus the summed `Total` of a row's own child frames,
    floored at zero for clock-granularity noise. Unlike the source
    project, `SelfSize` also exists: `trace.SizeOf` (mirroring
    `slices(x)`'s rune/element-count rule, not byte length) feeds a
    second KPI dimension, since the user explicitly asked for "memory
    used per function" alongside time.
- **`crust develop <file.crust>`** (`cmd/crust/debug.go`,
  `debug_view.go`) mirrors `runFile`'s two-phase execution (top-level
  eval, then resolve/call `store`/`store_<name>` per `SPEC.md` §9) but
  under a `debugger.Recorder`, and deliberately does *not* exit
  non-zero on a runtime `Error` the way `crust run` does — the recorded
  failure, in place, at the step that produced it, is the whole point
  of looking at the trace, not something to suppress by failing the
  command first. Plain-text output (`--plain`, or automatically
  whenever stdout isn't a real terminal — `isColorTerminal`, generalizing
  `main.go`'s `isTerminal` from a concrete `*os.File` to the `io.Writer`
  every command here is actually handed) is both a legitimate mode on
  its own (scriptable — grep it, diff it, assert on it in CI) and what
  makes the whole command unit-testable without a pty.
- **The interactive TUI** (`cmd/crust/debug_tui.go`, `debug_style.go`,
  `debug_editor.go`, `debug_run.go`) — bubbletea + lipgloss, cRust's
  first non-stdlib Go dependency (added specifically for this;
  everything else in the project stayed zero-dependency, see the Nix
  section). Five tabs, switched with tab/←→/shift+tab: **Time** and
  **Memory** (one pie chart each — self-time or self-memory per
  function/loop, `Timing.KPIs()`'s own ranked order — plus overall
  numbers specific to that dimension: step count/total time/slowest
  statement for Time, step count/largest single value for Memory; split
  into two tabs, see the dedicated note below, from the original single
  KPI tab that showed both charts at once), **Stepper** (the recorded
  tree, ↑↓ to move a highlighted cursor with the visible window
  scrolling to follow it, enter/space to open or close a *folded* run
  of loop laps — the only rows that ever need toggling, since an
  ordinary frame or step already shows everything under it;
  ​`recorder.go`'s fold mechanism is what keeps a huge run from flooding
  the view in the first place, so a second general-purpose
  collapse-everything UI on top of that would just be more interface
  for the same job), **Editor** (hands the terminal to a real `nvim` on
  the file being debugged; see the dedicated note below), and **Run**
  (type a path to an input file and press enter to run the debugged
  file with it as stdin, command-line style; see the dedicated note
  below).
  - **Tested the way `repl_tty.go` is: by driving the model directly**
    (`newDebugModel`, then `Update(tea.KeyMsg{...})`/`Update(tea.WindowSizeMsg{...})`,
    asserting on the returned model and `View()`'s text) rather than
    through a real pty — a bubbletea `Model` is a plain Go value with
    `Init`/`Update`/`View` methods, so nothing about testing it needs a
    terminal at all. Tests this way cover tab switching (all five,
    both directions, including the wrap), cursor movement and clamping
    at both ends, scroll-window following the cursor, fold toggling
    (open, close, and confirming a non-folded row is a no-op), view
    content for every tab including the zero-KPI/empty-recording edge
    case, the closing-marker rows `buildRows` inserts, `clampHeight`'s
    truncation, and the Editor tab's reload/nvim-exit state machine
    (`handleNvimExit`/`handleReload`/`reloadCmd`) driven directly with
    synthetic `nvimExitMsg`/`reloadMsg` values — real subprocess
    spawning is left to the pty pass below, the same split every other
    debugger real-vs-simulated test already follows.
  - **Also verified against the real compiled binary in an actual pty**
    (Python's `pty.openpty()` + `subprocess.Popen`, not just the Go
    unit tests above) — confirmed the real ANSI background-color
    escape codes for the pie chart wedges are actually emitted and
    form the right circular shape (not just that the *math* was right
    in isolation), that switching to the Stepper tab and sending
    movement keys doesn't crash, and that `q` exits cleanly (exit code
    0) rather than hanging or leaving the alt-screen in a bad state.
  - **Pie chart rendering** (`debug_style.go`'s `pieChart`) is plain
    trigonometry, not a charting library: for each terminal cell inside
    a radius-7 circle (`tx² + ty² ≤ radius²`), `atan2` gives the angle
    from 12 o'clock going clockwise, and that angle picks which KPI's
    cumulative-share bucket the cell falls in
    (`sliceIndex` over `Timing.KPIs()`'s own share-of-total boundaries).
    Each logical horizontal step renders as *two* terminal characters
    — cells are roughly twice as tall as wide, so without that
    compensation the "circle" comes out as a tall oval. A muted,
    pizza-toned six-color cycle (`sliceColors`) — tomato/cheese/basil/
    olive/crimson/sage, deliberately desaturated rather than default
    ANSI brights — is what "pizza-themed but streamlined, minimalistic"
    meant in practice: the theme is in the palette choice, not in
    cartoon pizza-slice iconography. A slice has no room for its own
    label at terminal resolution, so a text legend (colored swatch +
    name + percentage + formatted value) underneath is where a reader
    actually learns which color is which.
  - **Why KPI buckets by *family*, not call site, matters most
    visibly here**: the same `family()` collapsing described above
    (recorder.go) is what makes the KPI pie chart show one wedge per
    *function* — the literal thing the user asked to see — instead of
    fragmenting a single recursive recipe's cost across as many wedges
    as it has call sites in the source, which would have made the
    pie chart actively misleading rather than merely less useful.
  - **A real bug, found from actual use: a small terminal made the tab
    bar itself disappear on the original combined KPI tab** (before it
    split into Time/Memory, below). Neither pie chart scales down for a
    short window, and — combined with there being no alt-screen and no
    height clamp — a tab taller than the terminal just scrolled the
    ordinary way, carrying the tab bar and header
    (the first two lines printed every frame) right off the top with
    it, since there was no fixed scroll region to stop them. Confirmed
    with a real pty at 80×24: the tab labels were completely absent
    from what actually reached the screen, even though `View()` was
    still generating them correctly — the bug was in what the terminal
    did with the *output*, not in the string itself, which is exactly
    the class of bug Go unit tests asserting on `View()`'s return value
    can't catch. Fixed two ways: `tea.WithAltScreen()` (the standard
    choice for a full-screen app, confining rendering to a fixed
    viewport instead of the ordinary scrolling window), and a new
    `clampHeight(s string, n int)` that trims the *entire* rendered
    frame to at most `m.height` lines, replacing whatever's cut with a
    one-line "grow the terminal" note — so the tab bar and header,
    always the first lines `View()` writes, can never be pushed off no
    matter how tall a given tab's content gets or how small the window
    is. `viewStepper` already self-limited via `stepperBodyHeight`;
    `clampHeight` is the same idea applied once, generally, to the
    whole frame, so nothing else needs its own per-tab height logic.
  - **Frame/step rows with children get a `// end ...` closing marker,
    from a second real-use report: a long recipe call or loop body can
    run for dozens of screen rows, and indentation alone doesn't make
    it easy to tell where one actually ends** — the same problem
    unbraced code would have. `buildRows` (the tree-to-`visRow`
    flattener) now appends a synthetic row after any node's children,
    at the same depth as the node's own opening row, tagged
    `closing: true`; `renderRow` special-cases it to draw a faint
    `// end <label>` line instead of the usual step/frame columns
    (`closingLabel` strips the opening row's dangling `{ …`/`…`
    continuation marker, since a one-line closer has nothing left to
    continue into, and truncates separately from the table's own
    column width). The identical marker was added to `writePlain`'s
    walk for the same reason — this is a readability fix for the
    *debugger's own display*, not something that touches the `.crust`
    source file being debugged, so both render paths need it or they'd
    disagree about what the trace actually shows. A collapsed fold row
    gets no closer (nothing under it is visible, so there's nothing to
    mark the end of); expanding it produces one, like any other node
    with visible children.
  - **The Editor tab hands the whole terminal to a real `nvim` via
    `tea.ExecProcess` rather than drawing an editor pane inline** — the
    standard bubbletea pattern for shelling out to another full-screen
    program (`Program.ReleaseTerminal`, run the command with the
    terminal's own stdin/stdout, then `RestoreTerminal` and resume).
    The alternative — capturing nvim's own screen output through a
    terminal emulator and drawing it inside a bordered pane next to the
    tab bar, the way `tmux`/`zellij` panes work — was considered and
    explicitly turned down (asked of the user directly, since it's a
    real fork in both effort and behavior, not something to guess at):
    it would mean shipping a full VT100-class emulator inside this TUI
    just to render nvim's own screen, a much bigger and more fragile
    build for a debugging tool's third tab than reusing a real editor
    wholesale.
    - **nvim runs completely natively — no autocmd forcing a quit on
      every save.** The first version forced a `quitall!` right after
      any `BufWritePost`, so it could regain control and refresh the
      other tabs; in practice that made a plain `:w` (save, keep
      editing) indistinguishable from `:wq` (save, I'm done) — every
      save kicked you out and immediately reopened nvim, which read as
      janky rather than "the debugger just updates," and a real user
      said so. The fix was to stop trying to interrupt nvim's own
      session at all: `:w` now saves and keeps editing exactly like it
      would anywhere else, and the debugger only rechecks anything once
      nvim *actually* exits, however the user chose to do that (`:wq`,
      `:x`, `ZZ`, or a plain `:q`). Telling "something was saved during
      this session" apart from "nothing was" still can't use nvim's
      exit status (0 either way); it compares the file's mtime from
      immediately before `ExecProcess` runs to immediately after, which
      is enough since the question is "did *this* invocation touch the
      file," not a general "is the file dirty" check — and it doesn't
      care how many intermediate `:w`s happened along the way, only
      whether the file differs by the time nvim hands control back.
    - **Switching to the Editor tab launches nvim immediately —
      no enter required** (`maybeOpenEditor`, called from the same
      tab/shift-tab handling every other tab switch already went
      through). The tab *is* "open the editor"; a separate keypress
      once you're already looking at it was one more step than the
      action needed. `enter` still (re)launches it manually, for
      retrying after a launch error or reopening after a plain quit
      without having to tab away and back.
    - **A save (detected once nvim exits) triggers `reloadCmd`, which
      re-parses and re-records the file with the exact same
      `--store`/`--max-steps` the original run used** (`buildDebugView`,
      factored out of `runDebug` specifically so both the initial CLI
      invocation and this reload share one implementation rather than
      two that could drift). Its interpreter's program output
      (`deliver`, etc.) goes to `io.Discard`, not the real terminal —
      unlike the very first recording, built *before* the TUI ever took
      the screen, a reload runs while the alt-screen buffer is already
      active, and writing straight to the terminal from inside it would
      corrupt the display.
    - **Neither a successful nor a failed reload reopens nvim
      automatically — the user already chose to quit it, and that
      choice is respected either way.** Success (`handleReload`)
      replaces the view, resets the stepper's fold/cursor state (the
      new tree has no relationship to the old one's), and switches
      `active` to the Time tab — the save is done, nvim already closed,
      landing back on the dashboard is the point. Failure (a save that
      left the file with a parse error) surfaces the error on the
      Editor tab without touching the previous, still-valid recording —
      a mid-edit typo shouldn't blank out the Time/Memory/Stepper
      tabs — and leaves the user there to read it and reopen nvim themselves
      (`enter`, or tab away and back) rather than forcing them straight
      back in against their own quit.
    - **Verified against a real `nvim`, not just Go unit tests**: a pty
      running the actual compiled binary, with `nvim` genuinely
      installed, confirmed switching to the Editor tab opens nvim with
      no keypress beyond the tab switch itself; that `:w` alone saves
      and leaves nvim running (no forced exit); that appending a line
      and quitting with `:wq` persists the save, reruns the file (the
      Time tab's step count changed to match), and lands on the Time
      tab — not back in nvim. The parts that genuinely can't run under
      `go test` (spawning a real interactive subprocess, a live
      `Program.Run()` against a terminal) are the same shape of gap
      `runDebugTUI` itself already had — covered by this real-pty pass
      instead, the project's established substitute for what a unit
      test can't reach.
  - **The original single KPI tab (two pie charts, side by side or
    stacked depending on window width) split into dedicated Time and
    Memory tabs, from a direct request that each chart get the full
    window instead of sharing one.** `viewKPI`'s width-dependent
    `lipgloss.JoinHorizontal`/`JoinVertical` layout branch — the code
    that decided whether the two charts sat side by side or stacked —
    was removed outright rather than kept unused, since a single-chart
    tab never needs it; `viewTime`/`viewMemory` are what's left once
    that branching is gone, each just a chart plus its own stats block.
    Memory's stats (`viewMemoryStats`) aren't a copy of Time's with the
    numbers swapped: "total time" has no memory equivalent (summing the
    sizes of unrelated values isn't a meaningful number the way summing
    their durations is), so it's dropped, and "slowest statement"
    becomes "largest single value" (`largestValue`, `slowestStep`'s
    exact structure but comparing `Step.Size` instead of a node's self
    time) — the same shape of answer, "which one row explains this
    number," just asked about size instead of duration.
  - **The Run tab types a path and, on enter, runs the file being
    debugged with it as stdin, showing raw output — genuinely "the
    command line," not another view onto the trace.** It calls
    `runFile` (`run.go`) directly, the exact function `crust run`
    itself uses, rather than going anywhere near `debugger.Recorder`:
    no tracing overhead, no KPI bucketing, just the program's own
    stdout/stderr exactly as `crust run day01.crust < input.txt` would
    produce them. An empty path runs with no stdin at all (empty
    reader, not the real terminal — the alt-screen already owns it);
    a path that fails to open surfaces that error in the output area
    without ever calling `runFile`, and a runtime error inside the
    program shows up the normal way `crust run` would show it (via
    `runFile`'s own stderr reporting), not as a separate error path.
    - **The input-file field is hand-rolled** (`runInputModel`:
      insert/backspace/delete/left/right around a rune slice plus a
      cursor index, rendered with a reverse-video cell standing in for
      a terminal cursor) **rather than reaching for a components
      library** like `charmbracelet/bubbles`' `textinput` — a single-
      line path editor is a small enough job that a third TUI
      dependency, after bubbletea + lipgloss, wasn't worth it under
      this project's add-a-dependency-only-when-needed policy (the same
      reasoning that kept the project at zero third-party dependencies
      through Phase 5, see the dependency-policy row further down).
    - **The Run tab needed its own key handler
      (`handleRunTabKey`), separate from every other tab's, because a
      file path can legitimately contain any letter** — including `q`
      (quit everywhere else), `h`/`j`/`k`/`l` (tab-switch/movement
      everywhere else), and the arrow keys (cursor movement in the
      field here, but tab-switching on every other tab). Only Tab/
      Shift+Tab (switch tabs), Up/Down (move focus between the field
      and the entry-point selector below, when there is one — see
      next), Enter (run), and Ctrl+C/Esc (quit) stay reserved — the
      small set of keys a path could never plausibly need — and
      everything else, including those normally-bound letters, goes
      straight to the field. Verified via a real pty: typing a path
      containing every one of `q`/`h`/`j`/`k`/`l` in a row landed in
      the field intact, with no quit or tab change along the way.
    - **A second row, the entry-point selector, lets the Run tab pick
      which `store`/`store_<name>` recipe to call — added from a
      follow-up request once the tab already existed, since a file with
      more than one entry point had no way to choose which to run
      without restarting `crust develop` with a different `--store`.**
      `debugView.entryPoints()` scans the file independently of the
      current recording (its own lex/parse pass, cached like `timing()`
      already is) and calls `collectEntryPoints` — `run.go`'s own
      function for picking `crust run`'s default entry point, reused
      rather than duplicated — so the Run tab's list is exactly the set
      of names `--store` would ever accept for this file. A file with
      none (the common case: a small top-to-bottom script) shows no
      selector row at all, and Up/Down from the input field is a no-op,
      rather than a selector with nothing meaningful in it.
      `runEntryFocused` (a bool, not a bigger enum — there are only ever
      two rows) decides where Left/Right and typed keys go: cursor
      movement/typing in the field, or cycling the selection (wrapping,
      like the tab bar's own) when the selector has focus instead. The
      selection starts wherever `m.opts.Store` already points
      (`indexOfEntry`), so the Run tab defaults to matching whatever the
      Time/Stepper tabs are already showing rather than always resetting
      to the first entry point, and gets recomputed the same way after
      every Editor-tab reload, since an edit could have added, renamed,
      or removed a `store_` recipe. Verified via a real pty: the
      selector lists both entry points, cycling to the second and
      pressing enter runs that recipe specifically (its own output, not
      the default's).
    - **Running from the Run tab also retraces that same entry point +
      input file and swaps it in as the Time/Memory/Stepper tabs'
      recording** — a direct follow-up request: "make it so whatever
      store is selected in the run tab... is the one it does timings
      and memory checks and step debugging against." Until this,
      cycling the Run tab's selector only ever affected the Run tab's
      own raw output; the rest of the TUI stayed pinned to whichever
      `--store` the session started with, so picking a different entry
      point there didn't do what its name implied it should. Tied to
      pressing enter specifically, not to cycling the selector itself —
      running is also the only point an input file actually gets read,
      so retracing then means the KPI data reflects a real run against
      real input, not a phantom trace against nothing (or a re-read of
      a possibly-large file on every arrow-key tap). `runProgramCmd`
      reads the input file's bytes once (if any) and feeds independent
      readers built from those same bytes to two separate executions:
      the existing untraced `runFile` call for the Run tab's own raw
      output (unchanged — still "genuinely the command line," no
      tracing overhead in the way), and a second, traced run via
      `buildDebugView` (the same function the Editor tab's
      save-triggered reload already uses) whose recording becomes the
      new `m.view`. `handleRunResult` applies it the same way a
      successful Editor reload does — replace the view, reset the
      Stepper's fold/cursor state — except it deliberately does *not*
      switch to the Time tab or reset Run-tab focus, since the user is
      still mid-interaction on the Run tab, not asking to be taken
      anywhere else. `m.opts.Store` is updated too, so the selection
      becomes "current" for the rest of the session: a later Editor-tab
      save reload keeps using this same store rather than reverting to
      whatever `--store` the session originally launched with. A failed
      retrace (`view` nil — realistically only if the debugged file
      itself vanished between the two runs) leaves the previous
      recording in place rather than blanking those tabs out over what
      the Run tab still managed to show. Verified with three targeted
      Go tests: the retrace reflects the selected entry point's body
      (not the other one's), reads the same input file the raw run
      used (confirmed via a value the trace actually shows, since
      `deliver`'s own step always reads `nobox` — that's `deliver`'s
      return value, not its printed side effect, which goes to
      `io.Discard` for a retrace the same way the Editor reload's does),
      and a failed retrace leaves `m.view` untouched.
  - Not built: resizing the pie chart radius to the terminal's actual
    size (fixed at 7 regardless of window dimensions — `clampHeight`
    above stops a small terminal from losing the tab bar over this, but
    the chart itself can still get cut off rather than shrinking to
    fit) and a search/filter over the stepper tree the way the source
    project's visualizer has; both are natural follow-ons noted in
    `TODO.md` rather than guessed at.
- **Bug: a single-recipe program's whole run showed up in the KPI/
  stepper as one anonymous `call(...)` frame instead of the recipe's
  own name** — reported as "if I have a single store_part1 function it
  only lists that, not any of the inside parts." Reproducing it with a
  `--plain` recording showed the loop/order breakdown *was* actually
  there, nested one level down, but the top frame wrapping the whole
  thing (and the KPI's "by self time" table entry it rolled up into)
  read as a bare `call(...)`, indistinguishable from any other generic
  call — which reads as "no breakdown by name" at a glance, even though
  structurally the tree wasn't flattened. Root cause: `cmd/crust`
  resolves and invokes the `store`/`store_<name>` entry point (both
  `crust run` and `crust develop`, SPEC.md §9) through
  `Interpreter.Call`, the same exported method `map()`'s builtin uses
  to invoke its per-element callback — and `Call` hardcodes its frame
  label to the generic `"call(...)"`, which is the right call for
  `map` (a distinct label per element would be noise) but wrong for an
  entry point, which has a real, known name sitting right there in the
  `target` variable that resolved it. Fixed by adding
  `Interpreter.CallNamed(fn, args, name)` — identical to `Call` except
  the label is `name + "(...)"`, matching the label an ordinary
  `CallExpression` already builds from its own call-site source text —
  and switching both `run.go`'s `runEntryPoint` and `debug.go`'s
  `runDebugEntryPoint` to it, passing the already-resolved `target`
  string. `Call` itself is untouched, still generic, still what `map`
  uses. Verified via a real `crust develop --plain` subprocess, before
  and after: the tree's top frame and the "by self time" table now
  read `store_part1(...)` instead of `call(...)`; a second case with a
  helper recipe called from inside a loop confirmed nested named calls
  were already breaking out correctly (`helper(...)` already had its
  own frame and its own self-time row) — the bug was specifically the
  entry point's own frame, not frame-nesting in general.
- **`crust debug` renamed to `crust develop`**, on direct request —
  CLI-facing only: the dispatch string in `main.go`'s switch, usage/
  help text, and every error message/doc comment that names the
  command as something a user types. Internal Go identifiers
  (`debugOptions`, `runDebug`, `parseDebugArgs`, `buildDebugView`,
  `debugView`, `runDebugEntryPoint`, ...) and the `debug*.go` file
  names stayed as-is — renaming those too would be a much larger,
  purely-cosmetic refactor of implementation details nothing outside
  `cmd/crust` (and no user) ever sees, not something the request
  asked for.
  - **Bug found and fixed in the same pass**: `runDebugEntryPoint`
    silently did nothing when the resolved entry point wasn't found —
    exactly the confusion that prompted the rename request in the
    first place. A file with `store_part1`/`store_part2` but no bare
    `store`, run via `crust develop file.crust` with no `--store`,
    recorded nothing but the top-level recipe declarations (5 steps:
    just the declarations) and looked exactly like `crust develop`
    itself was broken, rather than like "you forgot `--store`."
    `run.go`'s `runEntryPoint` already had the right behavior for this
    exact situation — an explicit `--store=<name>` that doesn't exist
    is an error naming it; no `--store` given but real entry points
    exist is an error listing them (`collectEntryPoints`/`storeFlags`,
    both reused rather than duplicated); no store-family recipe at all
    stays a legitimate silent no-op, since the file already ran
    top-to-bottom by then. `runDebugEntryPoint` picked up the same
    three-way behavior, now returning an `error` instead of nothing,
    which `buildDebugView` surfaces the same way it already surfaces a
    parse error (there's no recording to show "in place of" this kind
    of failure — unlike a runtime `Error` from *inside* a successfully
    found entry point, nothing ran at all here). Verified via a real
    `crust develop` subprocess reproducing the exact reported
    scenario (confirmed the silent-empty-recording bug first, then
    confirmed the fix); two Go tests
    (`TestRunDebugNoDefaultEntryPointListsAvailableOnes`,
    `TestRunDebugUnknownStoreNameIsAnError`) cover both new error
    paths, restoring `cmd/crust` to its coverage baseline.

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
| LSP implementation | Hand-rolled JSON-RPC/LSP in `internal/lsp`, no third-party LSP library | The protocol subset `crust lsp` actually needs (lifecycle, hover, diagnostics) is small enough that a dependency wouldn't have saved much even before the zero-dependency policy ended (see below) |
| LSP distribution | A `crust lsp` subcommand, not a separate `crust-lsp` binary | Reuses the existing build/package/Nix-flake path entirely — no new binary to build, version, or install |
| Banner colors | 24-bit true-color ANSI, no 256-color fallback tier | Matches `assets/banner.png`'s hex palette exactly; a decorative help-screen banner degrading ungracefully on an ancient terminal isn't worth a second color-rendering path |
| TTY detection | `os.Stdout.Stat()` + `os.ModeCharDevice`, not `golang.org/x/term` | Was originally "keeps the dependency count at zero"; `crust develop`'s TUI ended that policy anyway (below), but this stayed as-is since `main.go`/`debug.go`'s own stdlib check already does the whole job — pulling in `x/term` for it now would be a dependency with nothing to show for it |
| Go dependency policy | Zero third-party dependencies through Phase 0–5, ends at Phase 6 with bubbletea + lipgloss (`crust develop`'s TUI) | `flake.nix`'s `vendorHash = null` (valid only for a stdlib-only module) became `pkgs.lib.fakeHash` — nixpkgs' own placeholder that fails informatively, printing the real hash, on the first `nix build` against real dependencies. Not computed in this same change since it needs an actual Nix install to produce, which this dev environment doesn't have (same limitation the uncommitted `flake.lock` note above already lives with) |

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
