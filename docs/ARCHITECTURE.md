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
- **Module system** (`delivery "path.crust"`, `SPEC.md` §10), from a
  direct request ("go ahead with the module system") — the first of
  TODO.md's two remaining Stretch Goals, and the keyword `SPEC.md` §4
  had reserved a working name for (`delivery`) since long before this
  phase's own design was written, specifically so this wouldn't need a
  new one invented later. Deliberately the simplest thing that could
  work for AoC-shaped programs: no namespacing, no exports list, no
  separate module value — a `delivery` statement lexes/parses/
  evaluates another file's top level directly into the *current*
  Environment, the same as pasting that file's text in at the
  statement's own location.
  - **New surface area, minimal**: `token.DELIVERY` (a new keyword,
    `internal/token`), `ast.DeliveryStatement{Token, Path string}`
    (`internal/ast/statements.go` — `Path` is a plain Go string, not an
    `Expression`, since the grammar (`deliveryStmt = "delivery" STRING
    terminator`) never lets it be anything but a bare string literal —
    the same "no computed imports" choice Go's own `import` makes),
    `parser.parseDeliveryStatement` (`internal/parser/statements.go`,
    reads the STRING token directly rather than going through
    `parseExpression`). Everything downstream of parsing is new
    territory for `internal/interpreter`, though: **this is the first
    time that package has ever needed to import `internal/lexer`/
    `internal/parser` itself** (`internal/interpreter/delivery.go`) —
    every prior caller (`cmd/crust`, `internal/runner`) did the
    lexing/parsing and handed `Eval` an already-built `*ast.Program`;
    `evalDeliveryStatement` is the first thing inside the interpreter
    that needs to read a *second* file and parse it mid-run. No import
    cycle results — `lexer`/`parser` still depend on nothing above
    them, `interpreter` was already the higher-level package in that
    relationship, so this is exactly the direction `ast`/`object`'s own
    "lower-level packages never import the packages that consume them"
    layering rule (§2) already allows.
  - **`Interpreter.BaseDir`**, a new exported field (default `""`,
    which `os.ReadFile`/`filepath.Join` already treat as "resolve
    against the process's own working directory" with no special-
    casing needed — exactly right for `crust repl`, which has no
    backing file at all): the directory a relative `delivery` path
    resolves against. Every real caller with an actual file — `crust
    run`/`develop`'s Run tab/the WASM playground (all three share
    `internal/runner.Run`, which already threads a `path` string
    through for error-message prefixing and now reuses that exact same
    value for this), `crust develop`'s own traced run and Live tab —
    sets it to `filepath.Dir(path)` right after constructing the
    `Interpreter`; nothing about `interpreter.New`'s own signature
    changed, so none of that wiring touched call sites that don't care.
    `evalDeliveryStatement` temporarily swaps `BaseDir` to the
    delivered file's *own* directory for the duration of evaluating its
    top level (restored via `defer` before returning) — so a relative
    `delivery` written *inside* a delivered file resolves against where
    that file actually lives, not the original top-level file's
    directory, the same relative-to-the-current-file rule most module
    systems use. Since a `DeliveryStatement`'s own `i.Eval(program,
    env)` call passes the *same* `env` the delivery statement is itself
    running in (never a fresh, enclosed one), this also makes delivery
    fully transitive automatically, with no extra mechanism: a file
    delivered three layers deep still ends up binding its names into
    the original top-level file's own scope, because there was only
    ever one `Environment` in play the whole time.
  - **A `map[string]bool` of already-delivered absolute paths**
    (`Interpreter.delivered`, initialized in `New`) does two jobs with
    one mechanism: skips redundant re-evaluation when a shared helper
    file is delivered from more than one place (both an efficiency
    concern and a correctness one — re-running a delivered file's own
    `deliver()` calls a second time would be a visible bug, not just
    wasted work), and — since a file is marked delivered *before* its
    own top level starts running, not after — guarantees a circular
    delivery (A delivers B, B delivers A) terminates instead of
    recursing forever: by the time B's own delivery of A is reached, A
    is already marked, so B's attempt is a silent no-op and both files
    finish defining whatever they were going to define. Verified with a
    real interpreter test (`TestDeliveryCircularImportTerminates`) that
    fails on a timeout, not just an assertion, if this ever regresses.
  - **Errors surface at the `delivery` statement's own position**, not
    buried inside whatever line inside the delivered file actually
    failed — a missing file, a parse error, or a runtime error while
    the delivered file's top level runs are all wrapped into a new
    `object.Error` naming the delivered path, reusing the exact same
    `newError(tok, format, args...)` helper every other runtime error in
    this package already goes through. A genuinely honest limitation,
    not silently glossed over: a *later* runtime error inside a
    recipe that was *defined* in a delivered file (called from the
    importing file, well after the delivery statement itself finished)
    still reports an accurate line number but under the *importing*
    file's own name, since nothing in this codebase's error pipeline
    (`object.Error`, `token.Token`) tracks which physical file a token
    came from — only its line/column within whatever source it was
    lexed from. Fixing that fully would mean threading a source-file
    identity through every token end to end, a materially bigger
    change than this feature's own AoC-shaped scope justified; the
    practical mitigation already exists for free, though — a stack
    trace's `Frames` (the earlier "richer stack traces" feature, above
    Phase 6) still names the failing recipe by its real name, which is
    usually enough to locate the bug in a small helper file even
    without exact file attribution.
  - **`crust develop`'s tracer needed zero changes** to work correctly
    through a `delivery` statement — verified with a real `--plain`
    run, not just reasoned about: a delivered recipe's own call frame
    shows up in the Stepper tree exactly like any other, because
    `evalDeliveryStatement` is just another `Eval` call using the same
    `Interpreter` (and therefore the same `Trace` hook) the rest of the
    run already shares. The tree-walker's "no separate execution
    strategy for imported code" design is what makes this fall out for
    free, the same way it made richer stack traces free earlier.
  - **`internal/format` and `internal/lsp` both needed a small, local
    addition, nothing structural**: `format/statements.go` prints
    `DeliveryStatement` by delegating to its own `String()` method
    (`fmt.Sprintf("delivery %q", ds.Path)` — no expression sub-tree to
    re-render, unlike every other statement kind this package prints);
    `lsp/hover.go` gained one `keywordDocs` entry. Completion
    (`internal/lsp/definition.go`'s `keywordSpellings`) and the docs
    site (`internal/docsite`, itself data-driven from
    `lsp.KeywordDocs()`) both picked the new keyword up automatically,
    with no code change at all, since both were already built to read
    `token.Keywords()`'s live table rather than hand-maintaining a
    second copy of it.
  - Verified with table-driven tests across every layer this touches
    (lexer keyword recognition; parser path-must-be-a-string-literal
    and path-required error cases; eleven interpreter-level tests
    covering a basic import, a shared variable, a missing file, a
    parse error and a runtime error inside the delivered file, same-
    file-delivered-twice dedup, later-definition-wins under the same
    scope rule, relative-path resolution against `BaseDir`, a nested
    file's own relative import resolving against *its* directory, the
    circular-import termination case, and an absolute path) plus a
    real two-file example
    (`examples/module_utils.crust`/`module_demo.crust`, wired into
    `cmd/crust/examples_test.go`'s pinned-output table the same way
    every other shipped example is) and real subprocess verification —
    `crust run`, `crust fmt` round-tripping it canonically, `crust
    tokens`/`crust parse` on the new keyword, `crust develop --plain`
    showing the delivered recipes traced correctly, and both the
    missing-file and circular-delivery error/termination paths through
    a real built binary, not just Go's own test harness.

### Phase 5 — Standard Library (`internal/builtins`) ✅
**Built ahead of schedule**, starting alongside Phase 4 — without at
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
(`unbox`/`lines`/`split`/`join`/`trim`), general string helpers
(`contains`/`find` extended to accept a String, `replace`, `upper`,
`lower`), math (`abs`/`pow`/`sqrt`/`gcd`/`lcm`, standard names — see
§7's own note on why), list sorting (`pizzasort`, themed rather than a
plain `sort` adapter, since "sort" alone says nothing about *which*
order), type conversion (`str`/`int`/`float`/`bool`), and
`+`-as-concatenation extended from strings to Lists and Tuples, and
`filter`/`reduce` (`map`'s siblings, added last — see their own note
below). Phase 5's stdlib checklist is now fully complete.

- **`pop(list)` / `pop(list, i)` / `pop(map, key)` / `pop(set, item)`**
  (`internal/builtins/builtins.go`), added on direct request after
  answering "Do we have a way to pop items out of collections?" — the
  honest answer at the time was no true pop existed anywhere: `scrape`
  removes from a Set but returns `nobox`, not the removed item; List had
  no dedicated removal at all (only the indirect
  `wrapReplace(list, i, i, [])`, which deletes but likewise doesn't
  return); Map had no deletion mechanism whatsoever. `pop` is one
  polymorphic builtin dispatching on `len(args)` and a type switch on
  `args[0]`, deliberately the return-value counterpart to
  `push`/`sprinkle`/`scrape` rather than a fourth removal builtin with
  its own name, since every collection type already needed exactly this
  one behavior and nothing type-specific beyond it. Per-type semantics
  each mirror an existing convention rather than inventing a new one:
  `pop(list, i)` doesn't wrap negative indices, matching plain `xs[i]`
  (SPEC.md §6 — only slicing wraps, not single-element indexing), and
  errors on out-of-range `i` the same way indexing does; `pop(list)`
  (no index) pops the last element, erroring on an empty List the same
  way `pop(list, i)` would with no valid `i` to give. `pop(map, key)`
  returns `nobox` for a valid-but-absent key, matching every other Map
  read (the same convention the memoization idiom in SPEC.md/CHEATSHEET
  already leans on), but still errors on a non-Hashable key exactly the
  way `map[key]` does on read — the two Map-read error/nobox cases (bad
  key type vs. merely-missing key) needed to both carry over, not just
  one. `pop(set, item)` follows `scrape`'s more permissive lead instead
  of Map's: `item`'s type is never checked, since a non-Hashable item
  simply can't be a member and "absent" (returning `nobox`) already
  covers that case without a separate error path. Required one new
  method, `object.Map.Delete(key) (value Object, removed bool)`
  (`internal/object/map.go`), added alongside `Get`/`Set` in the same
  shape (Hashable check, then a `Pairs` map operation) — nothing on
  `object.Set` needed to change, since `Remove`/`Has` already existed
  with the right signatures. All three collection cases mutate in
  place, matching `push`/`setAt`/`sprinkle`/`scrape`'s existing
  convention rather than returning a copy. Verified via Go unit tests
  (`internal/object/map_test.go`, `internal/builtins/builtins_test.go`)
  and a real `crust run` subprocess exercising all three collection
  types plus the empty-List/out-of-range/non-Hashable-key error paths.

- Most builtins are thin adapters over Go's standard library:
  `strings` (`contains`/`find`/`replace`/`upper`/`lower`, all wrapping
  `strings.Contains`/`Index`/`ReplaceAll`/`ToUpper`/`ToLower`) and
  `math` (`abs`/`sqrt` wrap `math.Abs`/`Sqrt` directly; `pow` only
  reaches for `math.Pow` when it has to — an Integer base with a
  non-negative Integer exponent instead computes via `intPow`,
  exponentiation by squaring, so the common bit-flag-puzzle case
  (`pow(2, n)`) stays an exact Integer instead of round-tripping
  through `float64`; `gcd`/`lcm` are the Euclidean algorithm directly,
  nothing in `math` already does this for machine integers). `unbox` wraps `os.ReadFile` (with a path argument) or reads
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
- **`filter(iterable, fn)` and `reduce(iterable, fn, init)` reuse
  `map`'s exact shape** — same `Call` injection, same List/Tuple-only
  acceptance, same short-circuit-on-first-error. `filterFn` judges each
  `fn(element)` result with `object.IsTruthy` (`internal/object/truthy.go`
  — the one shared rule `order`/`hold`/`with`/`or` already use, SPEC.md
  §6) rather than a second, local notion of truthiness. `reduceFn`
  requires `init` as an explicit third argument, deliberately not
  optional the way Python's `functools.reduce` is: an optional `init`
  makes an empty iterable a special case (raise, or fall back to some
  arbitrary "identity" the caller has to know about), where a required
  one makes `reduce([], fn, 0)` just return `0` — no branch, no
  surprise. Closes out Phase 5's stdlib checklist — sort (`pizzasort`),
  `filter`, and `reduce` were the last three items in it.
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

- **`wrap(collection, i)` / `wrapSlice(collection, start, end)`**
  (`internal/builtins/builtins.go`), added on direct request: "can I
  create a circular list in crust? where if I index past the end it
  just auto mods and loops back to the beginning... if I have a list
  of size 4 ... and I index list[3..6] it would give me 3, 0, 1, 2."
  No new type — a dedicated `Circular` wrapper would mean every other
  builtin/operator that already works on a List (`push`, `+`,
  `map`/`filter`/`reduce`, equality, ...) would need either a second
  implementation or an unwrap-then-rewrap step at every call site, for
  a feature that's really just "how do I compute an index," not a
  different kind of collection. `wrap`/`wrapSlice` are plain functions
  over the existing List/Tuple/String types instead: `wrap` reduces
  `i` modulo the collection's length before doing an ordinary read
  (`trueMod`, a small local helper — Go's own `%` keeps the dividend's
  sign, e.g. `-1 % 4 == -1`, exactly backward from what "loop back
  around" needs, which must always land in `[0, n)`). This doubles as
  the negative-indexing escape hatch SPEC.md §6 deliberately leaves
  out of plain `collection[i]` (`wrap(xs, -1)` reaches the last
  element the way Python's `xs[-1]` would, without making *every*
  single-index read silently negative-wrappable the way that
  language's is).
  `wrapSlice` is the circular counterpart to `collection[start..end]`
  (always inclusive of both ends, matching `..`'s default — there's no
  `.<`-style exclusive variant, since a circular span's whole point is
  one length per call, and an inclusive bound already says that
  directly). It reuses plain slicing's own "which way does this read"
  rule (`start <= end` forward, `start > end` backward, SPEC.md §6) but
  applies it to the *raw* start/end values rather than positions
  already resolved into `[0, n)` first — unlike a plain slice's
  negative bound (always "counted from the end"), a `wrapSlice` bound
  is just a point on an unbounded integer line that happens to wrap
  onto the collection every `n` steps, so comparing the raw values
  directly is what lets a span run past the far end and keep going
  from the start, or even lap the collection more than once, rather
  than needing an explicit "how many laps" argument:
  `wrapSlice([0,1,2,3], 3, 6)` is `[3, 0, 1, 2]` (the exact example
  asked for), and `wrapSlice([0,1,2,3], -1, 2)` reaches the same
  answer walking forward from just before zero. An empty collection is
  still a runtime error for both — there's no valid index to wrap onto
  no matter how it's reduced, so returning something silently (`nobox`,
  an empty result) would hide a genuine "this collection has nothing
  in it yet" bug instead of surfacing it the way every other
  out-of-bounds cRust operation does. Verified with Go unit tests
  (`internal/builtins/builtins_test.go`, table-driven, covering
  in-range/past-the-end/negative/backward/lapping cases against
  hand-computed expected indices) and a real `crust run` subprocess
  reproducing the exact `wrapSlice(xs, 3, 6)` example from the request.
- **`wrapReplace(list, start, end, value)`** — the write counterpart
  to `wrap`/`wrapSlice`, added right alongside them on the natural
  follow-up request: "can you make a function so I can assign to a
  circular list? ... something like wrapReplace(list, start, end,
  value) where value can shrink the list." A separate builtin rather
  than teaching `wrapSlice`'s own result to be assignable — cRust
  index-assignment (`xs[i] = v`) is a single-position write handled
  entirely inside the interpreter's own `evalIndexAssignment`, with no
  general notion of "assign to an arbitrary computed sub-selection,"
  and building one just for this would be a much bigger, more
  general-purpose feature than what was actually asked for.
  - **Design, corrected after a real bug (below)**: two different
    rules apply depending on whether `value`'s length matches the
    span's own (`count`), because they answer two genuinely different
    questions. **Same length**: a pure position-wise write-back —
    `list[indices[k]] = value[k]` for each of the span's positions
    (the exact same modular sequence `wrapSliceFn`'s own `wrapIndices`
    computes, in read order), leaving every other position untouched
    — this is the case that actually matters most in practice, since
    reversing (or otherwise permuting) a span in place, including one
    that wraps around the end of `list`, is a knot-hash-style
    algorithm's entire inner loop (AoC 2017 day 10 pins a wrapping
    span, reverses it, and repeats — see the bug report below for
    exactly this use case). **Different length**: there's no single
    position each new element could "belong to," so this falls back
    to a block splice instead — the span's positions are removed from
    `list` and `value`'s elements are inserted as one block at the
    position of the span's *first* index in read order, with
    everything else kept in its original relative order (a shorter
    `value` shrinks `list`, a longer one grows it, an empty `value`
    deletes the span outright, and a plain in-range forward
    destination reduces to exactly the splice Python's own
    `list[i:j] = value` does).
  - **A real bug, found immediately after shipping the first version
    — reported directly, with a worked example**: "For wrap replace
    when I pass [2, 1, 0, 3, 4] 3 6 [1, 2, 4, 3] in it results with
    [0, 1, 2, 4, 3], where I think it should result in [4,3,0,1,2]."
    The first version of this function used the block-splice rule
    (above) *unconditionally*, for every length — including the
    same-length case. That happened to coincide with position-wise
    assignment for a *non-wrapping* span (removing a contiguous run
    and re-splicing it in place, in order, is the same operation as
    writing each value back to its own position when the positions
    are already contiguous and in increasing order) — which is
    exactly what the function's own test suite covered at the time,
    masking the bug. The moment a span actually wraps, though, its
    positions are *not* contiguous in `list`'s own 0-indexed order
    (`wrapSlice(l, 3, 6)` on a 5-element list reads positions `3, 4,
    0, 1` — two separate runs), and linear block-splicing silently
    reordered `list[2]` — a position that was never part of the
    selected span at all — along with it, exactly the discrepancy
    reported. Confirmed by hand (`indices = [3, 4, 0, 1]`; want
    `list[3]=1, list[4]=2, list[0]=4, list[1]=3`, i.e. `[4, 3, 0, 1,
    2]`, matching the report) before touching any code. Fixed by
    splitting the same-length case out into its own position-wise
    branch (above); the different-length branch is untouched, since
    the bug report and the underlying real-world use case (knot-hash
    reversal) are both exclusively same-length. Re-verified against
    the exact reported input/output, a direct `crust run` reproduction
    of the report, and — since the motivating use case was a genuine
    AoC 2017 day 10 knot-hash implementation supplied along with the
    report — the full algorithm against that puzzle's own documented
    example (`3,4,1,5` on lengths `[0..4]` → `12`), which now passes.
  - **Mutates in place, List-only** — `push`/`setAt`'s own convention,
    not `wrapSlice`'s (which always returns a new value): Tuple and
    String are immutable in cRust, so there's nothing for a
    length-changing write to mutate on either of them, and a
    "circular list you can assign into" is squarely about the one
    genuinely mutable collection type. `value` itself can be a List
    *or* Tuple — a plain ordered sequence of replacement elements, the
    same "iterable" requirement `map`/`filter`/`reduce` already share
    for their own argument, not narrowed to match `list`'s own type.
  - An empty `list` is still a runtime error, same as `wrap`/
    `wrapSlice` — no valid index to wrap onto regardless of how it's
    reduced.
  - Verified with table-driven Go tests covering same-length
    replacement in both a non-wrapped span (the reversal example) and
    a genuinely wrapped one (the bug report), growing, shrinking, an
    empty `value` deleting the span outright, a wrapped span in the
    *different-length* branch with survivors on both sides, and a
    full-wrap replace-everything case, plus real `crust run`
    subprocesses reproducing both the original reversal example and
    the bug report end to end.
- **Bitwise builtins**: `band(a, b)`, `bor(a, b)`, `bxor(a, b)`,
  `bnot(x)`, `shl(x, n)`, `shr(x, n)` — on direct request: "can you
  also create a xor and other bitwise functions for crust?" Six plain
  functions rather than new infix operators (`&`/`|`/`^`/`<<`/`>>`) —
  cRust has no bitwise operators today, and adding them would mean
  new tokens, lexer/parser precedence entries, and interpreter
  evaluation cases, a much bigger and riskier change than the actual
  ask; `idiv`'s own precedent (plain function instead of a `//`
  operator) already established this pattern for `/`'s own edge case.
  All six are Integer-only, the same restriction `idiv`/`gcd`/`lcm`
  already apply, since bitwise operations aren't meaningful on a
  Float. `band`/`bor`/`bxor`/`bnot` are `b`-prefixed as a family for
  consistency, though only `bor` strictly needs it: `or` is cRust's
  own logical-OR keyword (SPEC.md §4), but `and` and `not` are
  actually free identifiers — cRust spells its logical AND/NOT as the
  `with`/`hold` keywords instead, never claiming `and`/`not` for
  anything (confirmed directly: `and = 5` and `recipe not(x) { serve
  x + 1 }` both parse and run without incident). Naming only `bor`
  with a prefix while `band`/`bxor`/`bnot` went bare would read as an
  arbitrary inconsistency, so the whole family stays uniformly
  prefixed instead. `shr` is arithmetic, not logical — the sign bit
  fills in from the high end, matching Go's own `>>` on a signed
  integer and every mainstream language's plain `>>` on a signed
  type, since cRust has no unsigned Integer type to make a logical
  shift meaningful against in the first place. Both shifts reject a
  negative count as an ordinary cRust runtime error rather than
  letting it through to Go's own `<<`/`>>`, which panics the whole
  process for a negative shift count — cRust has no precedent for a
  builtin crashing the interpreter over a bad argument anywhere else,
  division by zero included. Verified with table-driven Go tests
  (each operator's basic behavior, negative-shift-count errors, wrong
  argument types/counts) and a real `crust run` subprocess exercising
  all six together.
- **`rebox(element, fromBase, toBase)`** — base conversion, on direct
  request: "can you make a base conversion std lib function with a fun
  pizza jargon name? something like fn(element, frombase, tobase)."
  The name leans on the same "pizza box" metaphor `nobox` already
  established (§4: `nobox` is an empty box, nothing inside) —
  converting a number's base is repacking the exact same value into
  boxes sized differently (base-2 boxes hold one bit each, base-16
  boxes hold four, ...), nothing about the value itself changes.
  `element` is always a String — the digit representation being
  converted, in `fromBase` — never an Integer, even when `fromBase` is
  10: a number only really has "digits" once it's written out in some
  base, and requiring a String uniformly (rather than only when
  `fromBase != 10`) keeps one predictable rule instead of two.
  Converting an already-computed Integer starts from `str(x)`
  (`rebox(str(x), 10, 16)`), the same composable step `int(s)`'s own
  String-only requirement already expects elsewhere. The result is
  also always a String, including for `toBase == 10` — no
  special-cased return-an-Integer path, same one-rule-not-two
  reasoning. Implemented directly on `strconv.ParseInt`/`FormatInt`,
  which is also what sets the valid range — `fromBase`/`toBase` must
  be `2..36` (2-9 use digits, 10-35 add a-z/A-Z, one letter per value
  past 9), reported as an ordinary cRust runtime error rather than
  propagating `strconv`'s own Go-flavored error text. Input digit
  letters are accepted in either case (`"ff"` or `"FF"`), but output
  is always lowercase, matching `strconv.FormatInt`'s own convention
  and, not incidentally, the lowercase hex AoC's own knot-hash puzzles
  (2017 days 10/14) expect. Verified with table-driven Go tests
  (binary/decimal/hex/base-36 round trips both directions, a negative
  number preserving its sign, out-of-range bases, an invalid digit for
  a base, wrong argument types/count) and a real `crust run`
  subprocess.
- **`sum`/`reverse`/`sortBy`/`any`/`all`/`findInts`/`zip`/`manhattan`**,
  eight additions from a direct "look into what's missing from the
  stdlib" request followed by "go ahead with those" — chosen from a
  larger candidate list by ranking for AoC-shaped value against
  implementation cost, with a priority-queue/heap type (the recurring
  "Dijkstra pathfinding" AoC category) deliberately left out as its own
  open design question rather than folded in here. Every one reuses
  existing internal plumbing rather than inventing new patterns:
  `sum`/`any`/`all` share `asElements` (already backing
  `list`/`tuple`/`set`/`freq`, List/Tuple/Set uniformly);
  `sortBy`/`reverse`/`zip` deliberately *don't* — Set has no order to
  sort/reverse/pair by, the same reasoning `pizzasort`/`enumerate`
  already apply; `zip`/`sortBy` reuse `compareTwo`/the
  `object.Hashable` check `enumerate`/`tuple`/`set` already established
  for building result Tuples; `manhattan` reuses `gridPos`,
  `neighbors4`/`neighbors8`'s own `(row, col)`-Tuple-argument helper.
  - **`sortBy(list, fn)`** is `pizzasort`'s key-function counterpart —
    call `fn` once per element up front, then sort by comparing the
    *results* with the same `compareTwo` machinery `pizzasort` already
    uses. Deliberately `slices.SortStableFunc`, not `pizzasort`'s own
    plain `SortFunc`: a key function can produce ties between elements
    that aren't equal themselves (sorting `(name, age)` pairs by `age`
    alone), and preserving original relative order in that case is
    what a key-function sort is expected to do (Python's
    `sorted(key=...)` makes the same guarantee) — pizzasort itself has
    no such case, since two elements comparing equal under natural
    order really are interchangeable.
  - **`findInts(s)`** hand-rolls a single-pass rune scanner rather than
    reaching for Go's `regexp` package (a stdlib import, not a
    third-party one — would've been fine under the zero-Go-dependency
    policy) — the task is narrow enough (find digit runs, decide when
    a leading `-` is a sign) that a regexp would add a dependency this
    package doesn't otherwise have for no real simplicity win. The one
    genuinely tricky design call: **a `-` is a sign only when it isn't
    itself preceded by a digit**, not just "whenever a digit follows
    it" — `-?\d+`'s naive regex equivalent would misparse
    `findInts("1-3 a: abcde")` (AoC 2020 Day 2's own password-policy
    line shape) as `[1, -3]` instead of the intended `[1, 3]`, since a
    dash directly between two digit runs reads as a range/list
    separator in real puzzle input far more often than it means
    subtraction-as-a-number. `findInts("y=-10..-5")` still comes back
    `[-10, -5]` correctly, since each `-` there is preceded by `=`/`.`,
    never a digit — checked with a real test for exactly this AoC 2020
    Day 2 shape, not just reasoned about.
  - **`zip(a, b)` truncates to the shorter input** rather than erroring
    on a length mismatch — Python's own convention, and the more
    useful default for AoC's usual "walk two parallel lists together"
    use, where padding or erroring on an uneven split would just get
    in the way more often than it'd catch a real bug.
  - A genuine (if narrow) test-writing hazard, not a production bug:
    `builtins_test.go` already had a package-level `var sumFn =
    &object.Builtin{...}` — a stand-in addition-reducer callback
    `reduce`'s own tests pass around — that collided by name with the
    new `sumFn` implementing the real `sum(x)` builtin once both lived
    in the same package. Resolved by renaming the older, private test
    fixture to `addFn` (more accurate anyway — it was never about
    `sum()` specifically) rather than the production function, keeping
    `sumFn` matching every other builtin's own `<name>Fn` convention
    (`reduceFn`, `pizzasortFn`, ...).
  - Verified with table-driven Go tests per builtin (happy path, empty-
    collection edge cases, wrong-type/wrong-arg-count errors, error
    propagation from a failing callback for the three that take one)
    and a real `crust run` subprocess exercising all eight together.
- **Live tab watch-panel scrolling and Run tab output scrolling**, from
  a direct "I need to be able to look through the variable watcher"
  plus "add navigation... some sort of scrollable-ness on the output"
  request. Two independent features, each following whichever of the
  Live/Run tabs' own existing conventions already fit best rather than
  inventing a third pattern:
  - **Live tab (`m.liveWatchFocus`/`m.liveWatchTop`).** The watch panel
    used to always start at index 0 and show a static "and N more"
    tail — fine for a handful of variables, useless for browsing past
    the first few. Up/Down/j/k were already claimed on this tab for
    moving the source cursor, so a third state (rather than a fourth
    modifier key) follows the Bench tab's own `benchInspect` precedent:
    `w` toggles focus onto the watch panel, and while focused, the same
    j/k/Up/Down that move the source cursor instead scroll the watch
    list (`moveLiveWatchCursor`, clamped, mirroring the shape of every
    other scroll helper in this file). The title shows a live
    "(1-6 of 13)"-style position readout, and up to two hint lines
    ("N more above"/"N more below") appear depending on scroll
    position — both at once when scrolled to the middle of a long
    list, which is why `liveExtraLines()`'s reserved budget grew from
    `maxLiveWatchLines + 2` to `+ 3` (title + both hints, the new worst
    case). `liveWatchFocus`/`liveWatchTop` reset at the same points
    `liveEnv` itself already does — a fresh `r` run and a file switch
    via the Nav tab — so scroll state never survives past the data it
    was scrolled through.
  - **Run tab (`m.runOutputTop`).** `handleRunTabKey` is a wholly
    separate key handler from every other tab's, because its text-input
    field needs to capture almost any keystroke including letters that
    mean something everywhere else (`q`, `h`/`j`/`k`, ...) — a file
    path can legitimately contain any of them. That ruled out reusing
    a letter key for scrolling, and Up/Down already toggle between the
    input field and the entry-point selector, so PageUp/PageDown (an
    unclaimed pair, confirmed against the vendored bubbletea `key.go`
    as `tea.KeyPgUp`/`tea.KeyPgDown`, mapped from `\x1b[5~`/`\x1b[6~`)
    scroll the output instead — one page (`runOutputBodyHeight()`) at a
    time via `moveRunOutputScroll`, with the same "N more line(s)
    above/below (pgup/pgdn)" hint convention the Live tab's watch panel
    uses. `runOutputTop` resets to 0 as the very first line of
    `handleRunResult`, on both the success and failure paths, so a
    fresh run — including a rerun of the same file — always starts
    scrolled to the top rather than wherever the previous run's output
    happened to leave it.
  - Both verified with table-driven Go tests (scroll clamping at both
    ends, focus/key-redirection, reset-on-rerun) and real pty sessions
    driving `crust develop` end-to-end: stepping through a 12-variable
    recipe and confirming `w` + j/k moves the watch window and its
    "(N-M of T)" readout exactly as expected, and running a 60-line
    `deliver()` loop on the Run tab and confirming PageDown/PageUp
    scroll the output window and hints correctly.

- **`heapify`/`heapPush`/`heapPop`/`heapPeek`**, the priority-queue/heap
  gap named but deliberately deferred out of the eight-builtin batch
  above, now built out from a direct "go ahead on those" against a
  short list of AoC-workflow ideas. The design call made up front:
  **no new `object.Heap` type** — heap operations mutate an ordinary
  List in place, the exact same "any List becomes a heap the moment
  you call heap ops on it" posture Python's own `heapq` module takes
  (`heapq.heapify(list)`, not a dedicated heap object). That sidesteps
  a whole category of plumbing a real fifth collection type would
  otherwise need — `equal.go`/`copy.go`/`truthy.go` cases, LSP/format
  awareness, a new `ObjectType` — for a feature that's really just
  "keep this List in a particular order," the same reasoning that kept
  `sortBy`/`reverse`/`zip` from inventing anything new either.
  - **`heapCompare` extends `compareTwo` with lexicographic Tuple
    ordering** — the one genuinely new piece, and the reason
    `heapPush(pq, (dist, node))` (the standard Dijkstra/A* idiom,
    priority paired with payload) orders by `dist` first and only
    falls back to `node` as a tiebreaker, without a separate
    key-function argument the way `sortBy` needs one. `compareTwo`
    itself stays untouched (Integer/Float/String only, as every other
    caller — `min`/`max`/`pizzasort`/`sortBy` — already expects);
    `heapCompare` checks for the Tuple case first and recurses through
    *itself* (not `compareTwo`) on each element pair, so a Tuple of
    Tuples nests correctly, falling through to `compareTwo` for the
    ordinary numeric/String leaves.
  - **Hand-rolled sift-up/sift-down instead of Go's `container/heap`.**
    `container/heap`'s `Less(i, j) bool` can't propagate an error, but
    `heapCompare` can fail (mismatched, incomparable element types) and
    needs to come back as an ordinary cRust runtime `*object.Error`,
    not a panic — the same reason `sortBy`'s own comparator threads an
    error return through `slices.SortStableFunc`'s wrapper instead of
    using a plain `bool`-returning `less`. Hand-rolling two ~15-line
    functions was cheaper than adapting `container/heap`'s
    `Len`/`Less`/`Swap`/`Push`/`Pop` interface around that constraint.
  - `heapify` is bottom-up (starting from the last parent, sifting
    every subtree down once) rather than an element-by-element
    `heapPush` loop — O(n) instead of O(n log n), the standard
    binary-heap construction algorithm.
  - Verified with table-driven Go tests (heapify-then-drain comes back
    sorted, heapPush-only construction, peek doesn't mutate, empty-heap
    errors on both pop and peek, draining to exactly one element leaves
    no trailing slice garbage, wrong-type/wrong-arg-count errors, a
    mismatched-element-type push surfaces as an error rather than
    panicking) and a real `crust run` subprocess running a small
    Dijkstra shortest-path solver end-to-end — `heapPush(pq, (dist,
    node))`/`heapPop(pq)` against a tiny hand-checked weighted graph,
    confirming both the ordering and the final distances by hand.

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
    - **Per-file Run tab settings (store + input file) are remembered
      across invocations** (`debug_state.go`), a direct follow-up: "a
      saving of user settings for each develop on a crust file... what
      file is used as input... last entered by the user, and... which
      store it's using." One JSON file under the user's config
      directory (`os.UserConfigDir()`, e.g. `~/.config/crust/
      develop_state.json` on Linux — not a dotfile dropped next to
      every `.crust` file, which would litter every project directory
      with something crust itself would then have to know to ignore),
      keyed by each debugged file's *absolute* path so it works
      regardless of which directory `crust develop` gets invoked from.
      Saving is tied to the Run tab's "run" action (`runProgramCmd`),
      the same action the Time/Memory/Stepper linkage above already
      hooks — the natural point where both the store and the input
      file are simultaneously "current" and confirmed, not the moment
      the selector merely gets cycled past. What gets remembered flows
      back in two places: `runDebug`'s new `applySavedStore` fills in
      an unset `--store` from the saved value before the very first
      recording is even built (so `--plain` and the TUI's initial
      Time/Memory/Stepper tabs already reflect it, not just the Run tab
      after one manual re-run) — an *explicit* `--store` flag still
      always wins, the standard "remembered default, explicit override
      wins" precedence; and `runDebugTUI`'s new `restoreRunInput`
      pre-fills the Run tab's input field at startup. Writes go to a
      temp file in the same directory, then `os.Rename` into place —
      atomic on every platform Go supports for a same-filesystem
      rename — so an interrupted write can never leave the whole
      settings file corrupted for every other remembered project.
      Every persistence call is best-effort: a write failure (disk
      full, permissions) is silently dropped rather than interrupting
      the run it's piggybacking on, and a missing or corrupted
      settings file on load just comes back as "nothing remembered"
      rather than an error `crust develop` would have to report before
      it can even start — remembering settings is a convenience layered
      on top of the tool working, never a precondition for it.
      `develStateDir` (defaulting to `os.UserConfigDir`) is a
      package-level variable purely so tests can point it at a temp
      directory instead of the real machine's config directory — the
      same override-a-var-for-tests shape used nowhere else in this
      codebase before now, but the simplest fix for "this function
      always touches a real, shared, machine-global path" without a
      bigger dependency-injection refactor. Verified via a real `crust
      develop --plain` subprocess (pre-seeding the state file, then
      confirming an unset `--store` picked it up, and that an explicit
      `--store` still overrides it) plus Go tests covering the
      load/save round trip, multiple files coexisting, overwriting an
      existing entry, and the config-directory/write-failure paths
      (via a blocking file/directory in the way, not permission bits —
      this sandbox runs as root, which bypasses permission-based
      failures entirely, so only a structural filesystem conflict is a
      portable way to force these errors).
    - **Bug: `crust develop` crashed outright on some WSL setups** —
      `error creating cancelreader: bubbletea: error creating cancel
      reader: add reader to epoll interest list`, reported verbatim.
      Traced into bubbletea's own dependency,
      `github.com/muesli/cancelreader`: on Linux, `NewReader` type-
      asserts its input to a `File` interface (`io.ReadWriteCloser` +
      `Fd() uintptr` + `Name() string`); when that succeeds — which it
      always does for `os.Stdin`, an `*os.File` — it registers the fd
      with `epoll_ctl(EPOLL_CTL_ADD, ...)` so a `Cancel()` call can
      interrupt an in-progress blocking read. That registration is
      exactly what was failing under this user's WSL configuration
      (a known class of issue: some virtualized/WSL-interop ttys don't
      support epoll registration the way a native Linux tty does), and
      bubbletea has no fallback for an epoll setup failure — it's a
      hard error out of `initCancelReader`, thrown before the TUI ever
      draws a frame. `runDebugTUI` was explicitly opting into this
      path: `tea.WithInput(f)` with `stdin`'s underlying `*os.File`,
      which is what makes the type assertion succeed at all (bubbletea
      defaults to `os.Stdin` anyway when no `WithInput` is given, so
      this wasn't adding a capability, just making the epoll path
      reachable explicitly). Fixed with a `stdinOnlyReader{io.Reader}`
      wrapper passed to `WithInput` instead of the raw `*os.File` —
      embedding only the `io.Reader` interface promotes just `Read`,
      so the concrete wrapper type has no `Fd`/`Name`/`Write`/`Close`
      methods at all, which makes cancelreader's type assertion fail
      and fall through to its `fallbackCancelReader` (a plain blocking
      `Read`, no epoll, works everywhere). The one capability that
      fallback doesn't have — interrupting a read that's already
      blocked, from *outside* the read call — was checked against
      every way `crust develop` actually quits (`q`/ctrl+c/esc, or
      handing off to nvim) and found to be a non-issue: every one of
      those is itself the next keypress arriving, not some other part
      of the program trying to cancel a read nothing has typed into
      yet. Verified the mechanism directly rather than trying to
      reproduce a WSL-specific epoll failure in this environment: a Go
      test asserts `*os.File` satisfies a locally mirrored copy of
      cancelreader's own `File` shape (proving the type assertion
      *would* have fired) while `stdinOnlyReader` wrapping that same
      file does not (proving the fix actually changes the outcome, not
      by accident of some other difference), plus a plain read-through
      test confirming the wrapper still works as an ordinary reader.
    - **A closely related bug, reported next: "my input after running
      developer doesn't correctly input into the develop tool"** — the
      `stdinOnlyReader` fix above stopped bubbletea's own epoll setup
      from crashing, but didn't address a second, deeper problem the
      same real-stdin exposure caused: launching the interactive TUI
      still *ran the program* first (`buildDebugView`, using the real
      process's own stdin), before bubbletea ever took the terminal
      over for its own keyboard input. Any `store`/`store_<name>`
      recipe that calls `unbox()` would read from that same stdin —
      silently consuming whatever the user typed *next* as puzzle
      input, since nothing distinguishes "a keystroke meant for the
      TUI" from "a keystroke `unbox()` is blocking on" at the OS level;
      they're the same file descriptor. The user's own proposed fix was
      exactly right and is what got built: don't require `--store`/an
      input file up front at all — start the interactive TUI with
      "no data" in Time/Memory/Stepper, and only ever populate them
      once the user runs from the Run tab (which already reads an
      explicit input *file*, never real stdin, since the previous
      Run-tab-linkage change). `runDebug` now branches before running
      anything: `--plain`/non-tty output still calls the existing
      `buildDebugView` and runs eagerly (unchanged — there's no Run tab
      to defer to there, so showing anything at all means running it
      now), but the real-terminal case calls a new `emptyDebugView`
      instead — parses `path` (so a syntax error still surfaces
      immediately, and `entryPoints()` still works for the Run tab's
      own selector, since that's a fully independent lex/parse pass)
      but runs nothing, handing `runDebugTUI` a `debugView` wrapping a
      freshly-constructed, empty `debugger.Recorder`. Empty-recording
      rendering needed no new UI work at all — `TestDebugModelViewOnEmptyRecording`
      already exercised exactly this shape before any of this session's
      work started, so Time/Memory/Stepper already knew how to show
      "nothing recorded yet" correctly. `buildDebugView`'s own
      read-file-then-parse-then-report-a-syntax-error logic was pulled
      out into a shared `parseDebugFile`, so `emptyDebugView` doesn't
      duplicate it. The Editor tab's save-triggered `reloadCmd`
      (`debug_editor.go`) had the identical latent exposure — a save
      still re-ran the program via `buildDebugView`, using `m.stdin`
      (the same real process stdin) — caught and fixed the same way
      while already in this code: `reloadCmd` now reads stdin from
      `m.runInput`, the Run tab's own input-file field (the exact
      source `runProgramCmd` already uses for an explicit run), read
      fresh each time rather than passed down at TUI startup. An
      unreadable or since-moved input path degrades to empty input
      here rather than failing the whole reload — a code edit unrelated
      to the input file shouldn't be blocked by a stale path, unlike
      the Run tab's own explicit "run" action, where surfacing that
      error loudly is exactly the point. With `m.stdin` no longer
      referenced anywhere, the `debugModel.stdin` field itself (and
      `runDebugTUI`'s assignment into it) were removed outright rather
      than left as dead state — the `stdin io.Reader` *parameter* to
      `runDebugTUI` still exists and still matters, but only now for
      wiring bubbletea's own keyboard input (`stdinOnlyReader{f}`,
      above), a genuinely different use of "stdin" than the one being
      removed. Verified via a real `crust develop --plain` subprocess
      that the eager-run path is completely unchanged (reads real
      stdin exactly as before — there's no Run tab there to have
      fixed anything for). Go tests cover `emptyDebugView` directly
      (zero steps recorded, entry points still scanned, a missing file
      or parse error still reported) and `reloadCmd`'s new input
      source (the Run tab's file content actually reaching the
      retrace, a missing input path degrading to empty rather than
      failing, and the no-input-file case still producing a real
      recording) — `runDebugTUI` itself stays untestable without a
      real terminal, the same pre-existing gap `isColorTerminal`'s tty
      branch already had before any of this.
    - **A regression in the `stdinOnlyReader` fix itself, reported as
      the TUI's own help text and the user's raw keystrokes both
      showing up interleaved in the rendered output** — `stdinOnlyReader`
      solved the epoll crash by hiding `Fd()` (among everything else)
      so cancelreader's `File` type assertion would fail, but bubbletea
      has a second, independent type assertion that happens to use an
      almost-identical shape for an entirely different purpose:
      `tty_unix.go`'s `initInput` checks `p.input.(term.File)` — from
      `github.com/charmbracelet/x/term`, defined as just
      `io.ReadWriteCloser` + `Fd() uintptr`, no `Name()` — to decide
      whether to call `term.MakeRaw()` at all. Raw mode is what turns
      off the terminal's own local echo and line buffering; without it,
      every keypress both gets echoed straight to the screen by the
      OS *and* only reaches bubbletea a full line at a time (whenever
      the OS's line-buffered read finally sees a newline), rather than
      arriving as discrete key events. `stdinOnlyReader` hiding `Fd()`
      defeated this check too, so raw mode never engaged — explaining
      the exact symptom reported: the TUI's rendered frame (including
      its own help text) with the user's typed navigation keys
      (`h`/`j`/`k`/`l`, arrows) visibly mixed into it, echoed by the
      terminal rather than consumed as tab/cursor movement. Fixed by
      replacing `stdinOnlyReader` with `stdinNoNamer{f *os.File}` (a
      named, non-embedded field, so no method is promoted by accident):
      it forwards `Read`/`Write`/`Close`/`Fd()` explicitly but not
      `Name()`, satisfying `term.File`'s narrower shape (so
      `initInput` still finds a `Fd()` to call `MakeRaw` on — raw mode
      engages correctly again) while still failing cancelreader's
      stricter shape (`Name()` included), so the epoll path — and the
      crash it caused — stays avoided exactly as before. The two
      interfaces differing by exactly one method or the whole
      "same-attack-surface, opposite-desired-outcome" property. Verified
      the same way as the original fix: Go tests assert `stdinNoNamer`
      satisfies a locally mirrored `term.File` shape (proving raw mode
      will engage) while still failing a locally mirrored
      `cancelreader.File` shape (proving the epoll path stays avoided),
      plus a plain read-through test. Confirmed via `grep` across
      bubbletea's vendored source that it never calls `.Close()` on
      `p.input`/`p.ttyInput` directly, so `stdinNoNamer`'s `Close`
      passthrough can't cause stdin to close at some unexpected point.
      A real pty-based end-to-end check of raw mode actually engaging
      isn't possible in this sandboxed environment (no controlling
      terminal), so this fix rests on the interface-shape tests plus
      the mechanism read directly from bubbletea's and cancelreader's
      source, the same evidentiary standard the original epoll fix used.
    - **Feature request: "stores should be updated in the run tab
      everytime the editor is exited"** — `handleReload` (fired only
      after `handleNvimExit` sees `msg.saved`) already recalculated the
      Run tab's entry-point selector against the freshly re-recorded
      view, but that path only runs when a save was detected at all,
      and detection is an mtime-diff heuristic (`openEditorCmd` captures
      `mtimeOf(path)` before launching nvim, compares it to the same
      call after nvim exits) — a save landing inside the same
      mtime-resolution window the file was opened in could slip past it
      undetected on some filesystems. Rather than trying to make the
      heuristic itself more precise, decoupled "refresh the entry-point
      list" from "detected a save": added `debugView.refreshEntryPoints`
      (clears the `entryOnce`/`entryVal` cache `entryPoints()` already
      keeps, so the next call re-reads and re-parses the file rather
      than returning a stale scan from before the edit) and now call it
      — plus recompute `runEntryIndex` via the same `indexOfEntry` helper
      `handleReload` already used — unconditionally in `handleNvimExit`,
      on every clean exit, before the `msg.saved` branch that decides
      whether to also kick off a full `reloadCmd` retrace. The two are
      independent by design: rescanning entry points is a cheap
      lex+parse with no execution, so it costs nothing to do even when
      nothing changed, while a full retrace stays gated on an actual
      save, since re-running the file's `store` recipe isn't free and
      (for `unbox()`-calling programs) isn't side-effect-free either.
      Verified with Go tests that edit the file on disk (adding a second
      `store_<name>` recipe) and then call `handleNvimExit` directly
      with `saved: false`, confirming both the option list and the
      selected index update anyway — plus that a clean, saveless exit
      still doesn't trigger `reloadCmd` itself, matching the code
      comment's stated split.
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

- **`crust bake documentation`** (`internal/docsite`) — a browsable,
  pizza-themed reference site served over local HTTP, from a direct
  request to build something in the shape of RFuller25/domainlang's
  `domain expansion: documentation` (an embedded site + local HTTP
  server + best-effort browser launch), but themed for cRust and with
  one structural difference explained below.
  - **Server-rendered Go templates instead of a client-side JS app.**
    domainlang's site is a single large `index.html` that fetches
    Markdown pages and JSON data client-side and renders them with a
    hand-written JS Markdown parser (`render.js`). cRust's own
    conventions lean the other way on adding client-side machinery —
    `debug_run.go`'s hand-rolled text field over a components library
    is the same call for the same reason — and this codebase's whole
    culture is Go tests over manual verification wherever possible.
    `html/template`, executed server-side per request, gets both:
    every page is `httptest`-able (`docsite_test.go` asserts on the
    actual rendered HTML, not on JS behavior nothing here can execute
    in a test), and there's no Markdown parser to write, test, or trust
    — the two data-driven pages (see below) only ever need to render
    backtick-delimited code spans, the one piece of "Markdown" cRust's
    own hover docs actually use, which `renderInlineDoc` handles
    directly (split on `` ` ``, alternate plain/`<code>`, HTML-escape
    every segment either way — a full Markdown library would be a lot
    of unused surface for one feature).
  - **One `*template.Template` per page, not one shared parse of
    everything.** Every page file defines a `{{define "content"}}`
    block consumed by the shared `templates/layout.html.tmpl`; parsing
    all of them together (`ParseGlob`) would fail outright, since
    `html/template` errors on redefining a template name within one
    parse and every page's file defines that same name. Parsing
    layout+page pairwise (`pages["keywords"] =
    template.Must(template.ParseFS(fs, "layout.html.tmpl",
    "keywords.html.tmpl"))`, one call per page name) sidesteps the
    conflict entirely — each combination only ever sees one `"content"`
    definition — at the cost of the layout's raw text being parsed
    once per page rather than once total, a cost that doesn't matter at
    this site's size and request volume.
  - **Keywords and Standard Library are read live from
    `internal/lsp`, not written twice.** `crust lsp`'s own
    `keywordDocs`/`builtinDocs` tables (hover.go) are already
    hand-maintained copies of the language's real vocabulary, and
    `builtinDocs` is already guarded against drifting from
    `internal/builtins`' actual registrations by
    `TestBuiltinDocsCoversEveryRealBuiltin`. Adding a *third*
    hand-maintained copy for this site — the obvious-looking path,
    since domainlang's own primitives.json is exactly that, generated
    from Go source via `go test -update` — would mean a doc string
    could go stale here without any test catching it. Exporting
    `lsp.KeywordDocs()`/`lsp.BuiltinDocs()` (each re-keyed to what a
    reader actually types, `keywordDocs` itself being keyed by
    `token.Type`, and each returning a fresh copy so a caller can't
    mutate the package's own table through the result) instead makes
    this site inherit that same test's guarantee for free: whatever
    `crust lsp` would hover for a name is exactly what `/keywords` or
    `/builtins` prints for it, checked once, upstream, rather than
    needing its own duplicate check. `sortedEntries` turns either map
    into alphabetically-ordered rows — Go's map iteration order is
    randomized per the language spec, and a reference table that
    reshuffled itself on every page load would be unreadable. Each
    table also ships a small vanilla-JS filter box (`oninput` hiding
    non-matching `<tr>`s by substring) — the one piece of client-side
    script on the whole site, small enough not to need its own test
    beyond confirming the markup it operates on renders correctly.
  - **Theming**: light and dark pizza-crust palettes (cream/tomato-red/
    basil-green light, charcoal-oven/lighter-tomato dark) purely via
    `@media (prefers-color-scheme: dark)` CSS custom properties, no JS
    theme toggle — this site has no persisted user state to remember a
    manual choice across, so following the OS/browser preference
    already covers the real need.
  - **Command shape** (`cmd/crust/bake.go`) closely mirrors
    domainlang's `documentation` command: `-p`/`--port` (both
    space- and `=`-joined forms), bind, print the URL, best-effort
    `xdg-open`/`open`/`cmd /c start` (OS-specific, never fatal if
    missing), `http.Serve` until interrupted — Ctrl+C via the OS's
    default SIGINT handling, no signal code needed here either.
    `runBakeDocumentation(port, ...)` (binds, then hands off) is split
    from `serveDocumentation(net.Listener, ...)` (banner text + serve)
    specifically so a test can drive the latter against a
    self-created, self-closed `net.Listener` on an OS-assigned
    ephemeral port (`"127.0.0.1:0"`) — bind, real `http.Get` against
    the actual served content, `Close()` the listener to end
    `http.Serve` deterministically, assert the reported exit path —
    without needing a fixed port that could collide with a real `crust
    bake documentation` the person running the tests already has open,
    or leaving a goroutine serving forever past the end of the test.
    Verified end-to-end via a real subprocess too: `crust bake
    documentation --port 4747` bound, served, and a `curl` against
    `/`, `/builtins` (confirmed `abs`/`gcd`/`replace`/... — this
    session's own stdlib additions — showed up, proving the live-data
    wiring), `/keywords`, and a 404 path all returned the expected
    content, plus a real headless-Chromium screenshot of both the
    light and dark themes and the Standard Library page's search box.
- **`crust repl`** (`cmd/crust/repl.go`) — one persistent
  `*interpreter.Interpreter`/`*object.Environment` pair for the whole
  session, wrapped in a `bufio.Scanner`-driven read loop over the same
  `lexer.New`/`parser.New`/`Eval` pipeline `crust run` already uses.
  Three design points worth recording:
  - **Multi-line continuation reuses the real parser's own notion of
    "incomplete," rather than a second, approximate one.** The
    tempting shortcut — track brace/paren/bracket depth across lines
    by hand — is exactly the kind of logic that can quietly disagree
    with what the parser itself considers balanced (string escapes,
    comments, `..`/`.<` ranges that look like they could be
    ambiguous but aren't, ...), and cRust already has one correct
    definition of "this doesn't parse yet because more tokens are
    coming" sitting right there in `internal/parser`: every error path
    that can fire because the token stream ran dry (`peekError`'s "got
    %s (%q) instead", `noPrefixParseFnError`'s "no prefix parse
    function for %s found", the block/statement-terminator errors)
    substitutes the offending token's `Type` into the message, and
    `token.EOF`'s `Type` is literally the string `"EOF"` — a substring
    nothing a human would type as part of a real error collides with.
    `needsMoreInput` checks only the *last* collected error for that
    substring, not any of them: `synchronize()` stops advancing the
    moment it reaches EOF, so a genuine earlier error in a buffered
    multi-line chunk (which more input could never fix) still gets its
    own, earlier entry and isn't mistaken for "just needs more" — the
    two cases are told apart correctly rather than by which one
    happens to be checked first. Failing that fast is what keeps the
    REPL from getting stuck at a `...>` prompt no amount of typing
    could ever resolve.
  - **Only a bare expression statement's value auto-echoes** — the
    same convention Python's REPL (and most others) use, checked by
    `lastStatementIsExpression` looking at whether the chunk's last
    top-level statement is an `*ast.ExpressionStatement`. An
    assignment, loop, or conditional doesn't print anything of its
    own, since their entire point is the side effect. A recipe
    declaration is the one case worth calling out: `recipe foo() {
    ... }` parses as an `*ast.ExpressionStatement` wrapping an
    `*ast.FunctionLiteral` (the same shape `collectEntryPoints`,
    SPEC.md §9, already relies on to find `store`/`store_<name>`
    recipes), so defining one *does* auto-echo —
    `Function.Inspect()`'s `recipe(n) { ... }` — which reads less like
    an inconsistency and more like useful confirmation the definition
    took, the same spirit as Node's REPL echoing `[Function: foo]`
    after a `function` statement. `object.NULL` results are suppressed
    regardless (an explicit `nobox` included, matching Python
    suppressing `None` even when typed directly) — otherwise every
    `deliver(...)` call, which itself already wrote its own output and
    returns `nobox`, would print a redundant `nobox` right after.
  - **Known, accepted limitation: `unbox()` (no argument) shares stdin
    with the REPL's own line scanner.** `interpreter.New(stdout,
    stdin)` wires the same `stdin` the outer `bufio.Scanner` is reading
    lines from into the interpreter's `unbox` builtin — on a real
    interactive terminal this doesn't collide in practice (each
    blocking read only sees whatever's currently queued, typically
    just the line just entered), but under piped/redirected input the
    `Scanner` can have already buffered ahead of whatever line is
    currently being evaluated, so a mid-session `unbox()` call could
    observe fewer bytes than a naive reading would expect. Not solved
    here, unlike `crust develop`'s TUI (which really did need a fix,
    since a debugger silently eating a puzzle's real input as
    keystrokes was the actual bug being reported) — the REPL's stdin
    is for typing commands, and processing a real input file was
    always `crust run <file>`'s job, not something a REPL session is
    the intended tool for. Documented rather than engineered around,
    the same way this codebase notes other deliberately-out-of-scope
    edges elsewhere instead of silently leaving them unexplained.
  - Verified with Go tests (echoing, suppression, cross-line state and
    recipe persistence, multi-line continuation actually completing
    and evaluating, a genuine syntax error surfacing immediately
    rather than hanging at a continuation prompt, a runtime error not
    ending the session, blank lines, and a clean EOF exit) plus a real
    subprocess session piped through `printf | crust repl` reproducing
    all of the above end to end, including chained recipe definition
    and invocation and an error recovering back to a working prompt.

- **Source formatter** (`internal/format`, `crust fmt`
  (`cmd/crust/fmt.go`), and `crust lsp`'s `textDocument/formatting`
  (`internal/lsp/formatting.go`)) — reprints a parsed `*ast.Program` in
  cRust's one canonical style. Several design points worth recording:
  - **AST-based, not a token-stream reflow.** `internal/format.Format`
    takes a `*ast.Program` (already lexed and parsed once by the
    caller) and walks it, emitting fresh text — the same architecture
    `gofmt` uses, and the only one that can *guarantee* the output
    still means the same thing, since it's built from the structure
    the language itself assigns meaning to rather than from
    surface-level token patterns.
  - **Comments are entirely invisible to the AST** —
    `internal/lexer`'s `skipLineComment` strips `//...` before the
    parser ever produces a token for it, so a naive `Format` would
    silently delete every comment in the file. `internal/format/comments.go`
    runs a second, independent, string-literal-aware scan
    (`scanComments`) directly over the raw source to recover
    `{Line, Text, Standalone}` for every comment (tracking whether the
    scanner is inside a string literal, and skipping exactly one rune
    after a `\` escape, so a `//` inside a string like a URL is never
    misread as a comment start). The printer interleaves these back in
    by source line as it walks the AST (`flushStandaloneBefore`,
    `trailingComment`), rather than the AST carrying comments as
    first-class nodes — keeps `internal/ast` and `internal/parser`
    exactly as they were, at the cost of the formatter needing its own
    line-tracking discipline.
  - **Parens are always recomputed, never preserved** — architecturally
    forced, not a choice: `internal/parser`'s `parseGroupedExpression`
    returns the inner expression directly with no wrapper node, so a
    `(a + b) * c` and a hypothetical unparenthesized equivalent are
    indistinguishable by the time `Format` ever sees the tree. Every
    paren in the formatter's output is therefore derived at print time
    from operator precedence/associativity, via a `minPrec` threshold
    threaded down through `writeExpr` — traced per node type against
    exactly how `internal/parser`'s Pratt-parsing loop
    (`parseExpression(precedence)`, `for precedence < p.peekPrecedence()`)
    would need to see that same subexpression positioned in order to
    reparse it identically:
    - Ordinary left-associative `InfixExpression`: Left prints at
      `minPrec = ownPrecedence` (same-precedence chains naturally
      left-associate, so this is both safe and minimal); Right prints
      at `ownPrecedence + 1` (a same-precedence operator unparenthesized
      on the right would re-associate differently, so the extra
      precedence is load-bearing, not conservative padding).
    - Right-associative `ElvisExpression` (`?:`, parsed via
      `parseExpression(ELVIS-1)` for its right side): the mirror image
      — Right at `ownPrecedence`, Left at `ownPrecedence + 1`.
    - `RangeExpression`: Start (the accumulator side, since Range is
      registered as an ordinary infix parse function) at `precRange`;
      End (parsed via a separate, fixed `parseExpression(SUM)` call,
      not folded into the same precedence-climbing loop) at
      `precProduct` — deliberately *not* the naively-expected
      `precRange + 1`.
    - `PrefixExpression` operand: `precPrefix`, not `precPrefix + 1` —
      a prefix operator's operand comes from an unconditional recursive
      `prefix()` call with no precedence bound of its own, so nested
      same-operator prefixes (`- -x`) reparse correctly without a paren,
      unlike an infix operator's precedence-bounded continuation.
    - `TernaryExpression`: Cond conservatively uses
      `precTernary + 1` (treated as an "extension" position rather than
      a true accumulator, because Else's own greedy
      `parseExpression(LOWEST)` would swallow an immediately-following
      ternary continuation before the outer loop could ever reuse the
      whole first ternary as a new Cond); Then/Else both use
      `precLowest` (already fully delimited by `(|`/`|)`, so this
      almost never triggers a paren in practice).
    - `CallExpression`/`IndexExpression` bases print at
      `precCall`/`precIndex` (chained calls/indexes never need parens;
      a lower-precedence base like `(a + b)(1)` correctly keeps its
      paren). Call arguments, list/tuple/set elements, map values, and
      index expressions all print at `precLowest` — already delimited
      by commas/brackets/braces, never needing a synthetic wrap.
    Validated by the single strongest test in the suite,
    `TestFormatPreservesSemanticsAcrossExamples`: for every file in
    `examples/*.crust`, run both the original source and the
    formatted-then-reparsed source through `interpreter.Eval` and diff
    `deliver()` output byte-for-byte — all 9 pass, both via `go test`
    and independently via a real `crust fmt` subprocess.
  - **A genuine correctness bug, not just a cosmetic one, found by
    manual testing**: printing stacked negation (`- -1`) without a
    separating space produced `--1`, which `internal/lexer` re-lexes as
    the *decrement* operator (`IncDecStatement`'s `--`) — a different
    and invalid token in that position, not merely ugly output. Fixed
    with a single targeted check, `startsWithMinus`, that inserts a
    space specifically when a `-` prefix's operand is itself a
    `-`-operator `PrefixExpression` — the only place this grammar can
    produce two adjacent operator characters that collide with a
    different token.
  - **Blank-line preservation** uses one running cursor,
    `printer.lastEmittedLine`, checked by a single shared
    `blankLineIfGap(nextLine int)` helper called both when flushing a
    leading comment and when printing a statement — not two separate,
    ad hoc gap checks. `blockStatements` takes an explicit
    `headerLine int` and sets `lastEmittedLine = headerLine` before
    recursing into a block's body, so a block-opening header line
    written outside the shared per-statement loop (`order (...) {`,
    `} combo (...) {`, a `recipe foo() {` header, ...) doesn't leave
    `lastEmittedLine` stale and cause a spurious blank line right
    after it. Reaching this design took three rounds of real-file
    testing against `examples/the_works.crust`, each round catching a
    genuine bug an isolated unit test wouldn't have surfaced: blank
    lines inside block bodies being dropped entirely (only top-level
    ones were preserved), blank lines *before* a leading comment block
    being silently eaten (an earlier version only checked the gap
    *after* the last flushed comment), and a regression the fix for
    the second bug introduced — spurious blank lines appearing right
    after cuddled-brace headers — caught by a pre-existing
    `TestFormatIfComboSpecial` test failing.
  - **String and float literal re-escaping is hand-rolled**
    (`stringLiteral`/`floatLiteral` in `format.go`), not built on Go's
    `%q`/`strconv.Quote`, because it must exactly match
    `internal/lexer.readString`'s decode set (`\n \t \r \" \\` and
    nothing else — no `\x`/`\u`/`\a`/`\b`/`\f`/`\v`); reusing Go's
    quoting could emit an escape cRust's own lexer doesn't understand.
  - **`crust fmt`** (`cmd/crust/fmt.go`) prints the formatted result to
    stdout by default and only rewrites the file with `-w` — gofmt's
    convention, deliberately not rustfmt's (which overwrites by
    default) — with a `formatted == string(src)` short-circuit before
    any `-w` write so an already-canonical file's mtime is left alone.
  - **`textDocument/formatting`** (`internal/lsp/formatting.go`,
    `handleFormatting` in `lsp.go`) returns one whole-document
    `TextEdit` (`{0,0}` through the computed end-of-document
    `Position`) rather than a minimal line-level diff — the same
    "replace everything" shape a number of real LSP formatters use
    when they skip diffing. Returns an empty, non-nil `[]TextEdit{}`
    when the document is already canonical (so a format-on-save
    binding doesn't touch the buffer's mtime/undo history for no
    reason), and `nil` — no separate error channel fits here any
    better than it does for `hover`/`definition`/etc. — when the
    document doesn't parse. `formattingParams` deliberately omits the
    real LSP spec's `options` field (`tabSize`, `insertSpaces`, ...)
    since `internal/format` is fully opinionated with no
    configuration knobs, gofmt's stance applied to protocol params
    too. `initialize` advertises `documentFormattingProvider: true`.
    Verified with Go tests exercising the full JSON-RPC session
    (`TestServerFormattingViaJSONRPC`, matching the same
    `clientMessage`/`decodeServerMessages` wire-level pattern every
    other rich `internal/lsp` feature is tested with) and, separately,
    a real `initialize`/`didOpen`/`textDocument/formatting` session
    piped into a built `crust lsp` binary.
  - **Incremental document sync + code actions** (`sync.go`,
    `codeaction.go`), a direct follow-up request — "further LSP:
    incremental sync, code actions" — extending this already-shipped
    package rather than starting over. `initialize` now advertises
    `TextDocumentSyncKind.Incremental` (2, was `Full`/1);
    `handleDidChange` applies each `contentChanges` entry in order
    (per spec, later entries in one notification are relative to the
    *result* of earlier ones, not all against the original text) via
    `applyContentChange` — `change.Range == nil` is still accepted as a
    full-document replace (not every client honors the server's
    advertised kind on every edit), otherwise `change.Text` splices in
    at a byte span computed by `positionToByteOffset`. **No new
    offset-conversion math was needed** — `positionToByteOffset` reuses
    `position.go`'s existing `decodeOffset` (an LSP `Position.Character`
    in the negotiated encoding -> a rune count within that line) and
    `encodeOffset` in its `"utf-8"` mode (that rune count -> a byte
    count), the same encoding-aware plumbing hover/definition/etc.
    already needed, just not previously wired to turn a `Range` into a
    document-wide byte offset. Out-of-range Positions (a stale edit
    racing a fast-typing client) clamp to the nearest valid offset
    rather than panicking. `codeActionsFor` offers exactly one action —
    "Format document" (`CodeActionKind` `source.fixAll`) — built by
    calling `formatting.go`'s own `formatDocument` and wrapping its
    result in a `WorkspaceEdit`; its existing idempotency check means an
    already-canonical file offers no action at all, never a no-op edit.
    Deliberately the *only* action offered: cRust's diagnostics are
    lex/parse errors with no generically safe mechanical fix (unlike,
    say, an unused-import quick fix in a language with real static
    analysis), so this stays the one action that's always correct
    rather than guessing at ones that sometimes wouldn't be. Verified
    with table-driven unit tests for `applyContentChange`/
    `positionToByteOffset` (insert, replace, delete, a multi-line span,
    sequential changes composing, utf-8 multi-byte columns, an
    out-of-range clamp) and `codeActionsFor` (offered/not-offered/
    unparseable), two new full `Server.Run` wire-level tests
    (`TestServerIncrementalDidChange` inserting then deleting an
    illegal `@` via real `Range`-based edits, checking the resulting
    diagnostics actually track the edit; `TestServerCodeActionOffersFormat`),
    and — since this changes what `initialize` itself advertises, not
    just adds a new optional method — a real `crust lsp` subprocess
    session (raw JSON-RPC over a real pipe, not Go's own test harness)
    confirming both `textDocumentSync: 2` and `codeActionProvider:
    true` actually appear in a real `initialize` response and that an
    incremental edit and a code action's `WorkspaceEdit` are both
    correct end to end.

- **Run tab "run all stores" option** (`debug_run.go`'s
  `runAllStoresCmd`, `debugModel.runAllStores`, Ctrl+R), from a
  follow-up request: run every `store`/`store_<name>` entry point a
  file declares against the same input file, one after another,
  instead of only the selector's current pick — the natural next thing
  to want once a file has a `store_part1`/`store_part2` split, since
  checking both currently meant cycling the selector and pressing enter
  twice by hand.
  - **`debugger.Recorder.Merge(label string, other *Recorder)`**
    (`internal/debugger/recorder.go`) is the piece that makes combining
    several independent runs into one Stepper tree and one shared
    Timing/KPI pass possible with zero changes to either: it appends
    `&TraceNode{Frame: label, Children: other.Roots()}` to the
    receiver's own `roots`, sums `steps`, and propagates `truncated`.
    That this works at all falls out of a fact already true of
    `Timing.measure` and `buildRows` (debug_tui.go): both walk purely
    by tree structure — `IsFrame()`, `Children`, `Label()` — with no
    notion of "this came from one run" baked in anywhere. A synthetic
    wrapper frame is therefore indistinguishable from an ordinary
    recipe-call frame to everything downstream, including KPI bucketing
    (`family()`) — which means each merged store's own total time shows
    up as one KPI row too (`store_part2(...)`, `Calls: 1`), not just the
    recipes it happens to call, a small extra benefit rather than
    something that had to be special-cased.
  - **Each store still gets its own complete, independent `runFile`/
    `buildDebugView` call** — its own `Interpreter`, environment, and
    `bytes.NewReader(data)` over the input — rather than being chained
    through one shared `Interpreter` across the loop, even though that
    would have been the more obvious way to build one merged `Recorder`
    directly (call `interp.CallNamed` once per store against the same
    `Interpreter`/`Recorder`, since `CallNamed` already opens its own
    top-level frame per call). The reason it doesn't: `unbox()` with no
    argument reads whichever `stdin io.Reader` was bound at
    construction (`internal/builtins.New`'s own doc comment), so a
    shared `Interpreter` would leave every store after the first
    reading a reader the first store's own `unbox()` calls had already
    drained, rather than the fresh, full view of "the same input file"
    this option promises. Running each store as a fully separate
    `runFile`/`buildDebugView` pair — the exact same two calls
    `runProgramCmd` already makes for a single store — reproduces
    exactly what separately typing `crust day01.crust --store=part1 <
    input.txt` and `crust day01.crust --store=part2 < input.txt` at a
    real shell would each get, which is the behavior this feature is
    meant to stand in for.
  - **Falls back to one bare-`store` run when the file declares no
    entry points at all**, rather than being a dead end for the common
    small-AoC-script shape: `options := m.runEntryOptions(); if
    len(options) == 0 { options = []string{""} }` before the loop, so
    toggling "run all" on for a plain top-to-bottom script just runs it
    once, same as leaving the toggle off would.
  - **Raw output concatenates under a `=== <name> ===` heading per
    store**, in entry-point order (`entryLabel`, `"(default)"` for the
    bare `store` — the same convention `renderEntryOptions` already
    uses for the selector row itself). The wrapper frame's own label
    (`entryFrameLabel`) instead reuses `target+"(...)"`, the exact shape
    `interpreter.CallNamed` already labels a traced entry-point call
    with, so a merged run's Stepper/KPI rows read identically to an
    ordinary single-store one, just one level further out.
  - **Persisted per-file alongside the existing entry-point/input-path
    settings** (`debug_state.go`'s `develState.RunAll`,
    `restoreRunAll`/`saveDevelStateBestEffort`'s new fourth parameter)
    — reopening the same file later starts the Run tab with the toggle
    exactly where it was left, the same "remember my last choice"
    treatment `--store`/the input path already got.
  - Verified with unit tests at both layers — `internal/debugger`'s
    `TestMerge*`/`TestTimingWorksAcrossAMergedRecording` prove `Merge`
    and `Timing` compose correctly in isolation; `cmd/crust`'s
    `TestRunAllStoresCmd*`/`TestHandleRunTabKeyCtrlR*` prove the Run tab
    wiring — plus a real pty-driven `crust develop` session: typed an
    input path, toggled Ctrl+R (the hint line flips from "off" to "ON",
    and the pre-run instructions update to describe running every entry
    point), pressed enter, and confirmed both `store_part1`/
    `store_part2`'s own output appeared under their own headings
    against the same input file, with the header's step count updating
    from "0 steps" to the real merged total.

- **Files tab** (`debug_nav.go`, `tabNav`), from a follow-up request:
  browse and switch to another `.crust` file alongside the one
  currently open, without leaving `crust develop` and relaunching it
  against a different path on the command line — the natural next
  thing to want on a directory full of `day01.crust`..`day25.crust`
  AoC solutions sitting side by side.
  - **Appended as a sixth tab, after Run, rather than inserted
    anywhere else in the lineup.** `tab` is a plain `int` `iota`
    enum, and every existing tab is referred to by its symbolic
    const (`tabTime`, `tabEditor`, ...) everywhere in the codebase, so
    reordering the block would have been safe too — but appending at
    the end was still the simpler diff, touching `tabCount` and one
    new case per switch rather than renumbering anything, and it
    keeps the existing tabs' muscle-memory order (Time/Memory/Stepper/
    Editor/Run) completely undisturbed.
  - **File listing and switching reuse existing machinery almost
    entirely** rather than inventing a parallel path: `listCrustFiles`
    is a plain `os.ReadDir` + `.crust`-suffix filter + `sort.Strings`
    (AoC's own `day01.crust`..`day25.crust` naming already sorts into
    the order you'd want) that degrades to `[]string{currentPath}` on
    a read failure, the same "don't fail the whole tab over a listing
    problem" choice `scanEntryPoints` already makes for a bad parse.
    `switchToFile` builds the target file's view via `emptyDebugView`
    — the exact same "parsed but not yet run" starting point the whole
    session begins with (`runDebug`'s real-terminal path already
    builds one before bubbletea ever takes the screen, specifically so
    a `store` recipe's `unbox()` can never silently eat a keystroke
    meant for the TUI instead of the interactive session; see that
    function's own doc comment) — rather than eagerly tracing the
    newly-opened file, which would reintroduce exactly the bug that
    machinery was built to avoid. Every per-file setting (`opts.Store`,
    the Run tab's input path, the run-all toggle) reloads from the
    *target* file's own remembered state (`debug_state.go`) rather than
    carrying over whatever the file being left had, matching what a
    fresh `crust develop otherday.crust` invocation would start with —
    not "the same settings, new file," but "this file, however it was
    last left."
  - **A parse failure in the target file surfaces on the Files tab
    (`navErr`) without disturbing the still-valid current
    view/recording** — the same "leave the last good state alone"
    choice `handleReload` already makes for a save that breaks the
    file being edited. `navErr` renders *above* the file list rather
    than replacing it, deliberately: hiding every other file behind
    the error would leave no way to pick a working target after a
    failed attempt, and `refreshNavFiles` clears it on the next visit
    to the tab so a stale error from a prior attempt doesn't linger
    once you've moved on.
  - **Rescanned on every tab switch that lands on Files
    (`maybeRefreshNav`), not cached for the session** — directory
    contents can change between visits (a new day's file added, one
    renamed), and an `os.ReadDir` is cheap enough that trusting a
    stale list would save nothing worth the staleness. This is also
    what repositions `navCursor` onto whichever file just became
    current after a switch, so returning to the tab always shows you
    where you are, not where you started.
  - **A real gap this surfaced, not an intentional design choice**:
    reaching Files by tabbing *forward* from Run goes through
    `handleRunTabKey`, the Run tab's own separate key handler (it
    needs almost every key for its text field, so it doesn't share the
    rest of the TUI's `handleKey` switch — see that function's own doc
    comment) — which has its *own* Tab/Shift+Tab cases, not the ones
    `maybeRefreshNav` got added to first. Missing the call there meant
    reaching Files the most direct way (tabbing forward from Run,
    where it sits immediately after) showed a stale or empty listing
    even though every other path into the tab worked. Caught by
    `TestTabFromRunReachesFilesTabAndRefreshesIt`, fixed by adding the
    same `maybeRefreshNav()` call `handleRunTabKey`'s own Tab/Shift+Tab
    cases were missing.
  - Verified with unit tests — 100% statement coverage on every
    function in `debug_nav.go` — and a real pty-driven `crust develop`
    session: opened `day01.crust` alongside a `day02.crust`, confirmed
    the Files tab listed both with `day01.crust` marked `(current)`
    and the cursor already there, moved down and pressed enter, and
    confirmed the header switched to `day02.crust — 0 steps` and a
    return trip to the Files tab now marked `day02.crust` current
    instead.
- **`crust develop <file.crust>` auto-creates a missing target file**
  (`ensureFileExists`, `cmd/crust/debug.go`) instead of erroring out —
  starting a new AoC day's file is the single most common reason to
  point `develop` at a path that isn't there yet, and the old behavior
  (`crust develop: open day06.crust: no such file or directory`, exit
  1) meant that always had to be a two-step "touch the file, then
  develop it" dance. `ensureFileExists` runs first, ahead of even
  `applySavedStore`, and only ever creates the target file itself —
  never a missing parent directory, since a missing directory much
  more likely means a typo'd path than an intentional "scaffold a new
  subdirectory" request, and silently creating directories nobody
  asked for is a bigger, more surprising side effect than creating one
  empty file. `os.OpenFile` with `O_CREATE|O_EXCL` makes the
  existence check and the create atomic (no separate Stat-then-Write
  race); `os.IsExist` on the resulting error is what tells "a file was
  already there" apart from every other reason the open could fail
  (missing parent directory, permissions, ...) — only the latter is
  actually reported as an error, both from `ensureFileExists` and
  visibly as `crust develop: <path> doesn't exist yet — created it` on
  stderr when it *does* create something, so the behavior is never
  silent. From there the rest of `runDebug` proceeds exactly as it
  would for a file that already existed: the interactive TUI opens on
  its ordinary default (Time) tab — empty and harmless, since there's
  nothing recorded yet for a file nobody has run — with the Editor
  tab's `nvim` hand-off one tab-key away, or, `--plain`, an (empty,
  harmless) "0 steps" printout. Verified with unit tests
  (`TestEnsureFileExistsCreatesAMissingFile`,
  `TestEnsureFileExistsLeavesAnExistingFileAlone` — confirms an
  existing file's contents are left untouched and nothing is printed
  to stderr for it, `TestEnsureFileExistsMissingParentDirIsAnError`,
  `TestRunDebugAutoCreatesMissingFile`,
  `TestRunDebugAutoCreateWorksInTUIPath` — confirms the auto-create
  happens once in `runDebug` itself, ahead of the `--plain`/TUI branch
  point, not duplicated into each branch) and a real pty-driven `crust
  develop` session against a path that didn't exist: confirmed the
  file was created (0 bytes) and the TUI came up normally on the Time
  tab, then tabbed over to Editor and confirmed real `nvim` opened on
  the freshly-created empty file (correct filename in nvim's own
  status line) rather than erroring — the first draft of this doc
  comment claimed auto-create landed "straight into the Editor tab,"
  which this same pty session caught as wrong: it lands on the normal
  default tab like any other file, and reaching Editor is still a
  deliberate tab-key press away, same as always.
- **The Files tab can create a new `.crust` file directly**
  (`debug_nav.go`'s `handleNavCreateKey`/`createNavFile`), on direct
  request: "can you make it so I can create a new crust file from the
  file tab in the develop tool?" — the interactive counterpart to
  `crust develop`'s own missing-file auto-create above, for starting a
  *second* (or third, ...) day's file without leaving the session.
  Pressing `n` on the Files tab sets `navCreating` and swaps the tab's
  body to a single-field prompt, reusing `runInputModel` (the Run
  tab's own hand-rolled text field) rather than building a second
  implementation of the same small job; `handleKey` routes every key
  to a dedicated `handleNavCreateKey` the moment `navCreating` is true,
  the same "this mode wants nearly every key for itself" reasoning
  `handleRunTabKey` already established. Enter appends a `.crust`
  suffix if the typed name doesn't already have one and creates the
  file via the same `O_CREATE|O_EXCL` atomic-existence-check pattern
  `ensureFileExists` uses — but with the opposite answer for "it
  already exists": `ensureFileExists` treats that as success (a
  missing target is the *expected* case it's guarding against), while
  this reports it as `navErr` and leaves the prompt up, since creating
  is an explicit "make something new" action where silently switching
  to an existing `day01.crust` typed by habit would be a surprise, not
  a convenience. A successful create clears `navCreating` and hands
  off to `switchToFile` — the exact same path Files-tab switching
  already uses — so the new file gets the identical fresh-view/
  reset-settings treatment any other switch does. Esc cancels the
  prompt without quitting (it's a transient dialog over the ordinary
  Files list, not a permanent tab fixture with nothing to back out
  of); Ctrl+C still quits the whole session regardless, the same
  always-available hard exit the Run tab's own field keeps for the
  same reason. Verified with unit tests (typing, Esc/Ctrl+C, empty
  name, an already-existing name, the `.crust`-suffix behavior both
  ways, and the full create-then-switch path) and a real pty-driven
  session: pressed `n`, typed a name, hit enter, confirmed the file
  existed on disk at 0 bytes and the session had switched to it.
- **Fixed: the Run tab's entry-point selector sometimes silently
  failed to refresh after saving in the Editor tab** (`debug_editor.go`),
  on direct request: "I'd like stores in the run tab to refresh when I
  exit from the editor" — surprising, since exactly this refresh was
  already built and tested (the auto-create entry above's neighbor in
  the checklist). A real pty-driven session reproduced the actual
  bug: `nvim` exiting cleanly (status 0) after a genuine `:wq` save,
  but `handleNvimExit`'s `msg.err` still non-nil — a spurious `read
  /dev/stdin: resource temporarily unavailable` (`EAGAIN`) — which
  `handleNvimExit` treats as a hard launch/exit failure and returns
  from immediately, before ever reaching the entry-point rescan or the
  save-triggered reload, so a perfectly good save silently never
  updated the Run tab.
  - **Root cause**: `openEditorCmd`'s `exec.Command("nvim", path)` left
    `Stdin` unset, so bubbletea's own `ExecProcess`
    (`osExecCommand.SetStdin`'s "if `c.Stdin == nil`" guard) filled it
    with `p.input` — `runDebugTUI`'s `stdinNoNamer{f}` wrapper around
    the real terminal, deliberately *not* a concrete `*os.File` so
    `cancelreader`/`term.File`'s own type assertions engage correctly
    (see `stdinNoNamer`'s own doc comment, and the WSL-epoll bug
    earlier in this section that motivated it). That's exactly wrong
    for a *child process*, though: since Go's `os/exec` can only hand
    a real `*os.File`'s fd directly to a spawned process, anything
    else forces it onto a slower fallback — open an `os.Pipe` and
    spin up a background goroutine that copies bytes from the wrapped
    reader into the pipe's write end, with the child's own stdin
    wired to the read end. `nvim` itself never touches the real
    terminal fd at all in that setup; the copier goroutine does, and
    it was that goroutine's own blocking `Read` on the real terminal
    racing `nvim`'s exit that returned the transient `EAGAIN` — which
    Go's `os/exec` then surfaces as `cmd.Run()`'s own returned error
    (collecting a non-nil error from an I/O copier goroutine when the
    process itself exited successfully is documented `os/exec`
    behavior, not a bug in the standard library), even though `nvim`
    itself never saw or caused it.
  - **Fix**: a new `nvimCmd(path string) *exec.Cmd` helper (split out
    of `openEditorCmd` specifically so `cmd.Stdin`'s value is directly
    assertable by a Go test, not only observable through a real pty
    session) presets `cmd.Stdin = os.Stdin` — the real, global
    `*os.File` — *before* `tea.ExecProcess` ever runs, so
    `SetStdin`'s "if unset" guard never fires and `os/exec` dup's the
    fd straight into `nvim`, the same way any ordinary shell
    redirection would. No pipe, no copier goroutine, nothing left to
    race.
  - Verified two ways: a direct Go test
    (`TestNvimCmdUsesRealStdinNotAWrapper`, asserting `cmd.Stdin ==
    os.Stdin`) that locks the fix in independently of pty timing, and
    the same real pty-driven session that first reproduced the bug
    (adding a `store_part1` recipe inside `nvim`, saving, quitting),
    re-run repeatedly post-fix and confirmed clean every time: no
    `editorErr`, and the Run tab's selector shows `part1` immediately
    on return.
- **A Bench tab** (`cmd/crust/debug_bench.go`, `tabBench`), on direct
  request: "write a KPI for the develop tool that does x number of
  runs of the current settings of the run tab and shows a graph on the
  average runtime, the average memory, the max runtime and memory, the
  min memory and runtime, and any other stats that would be notable in
  the graph over those runs? Also make it so any of the lines on the
  graph can be enabled or disable[d] depending on what I'd like to
  compare."
  - **Where it sits and what it reads**: inserted between Run and
    Files in the tab order — it's a direct extension of "run this file
    with these settings," so it belongs right next to the tab whose
    settings it reads (`m.selectedRunEntry()`, `m.runInput`), not off
    on its own. Type a run count into its own field (reusing
    `runInputModel`, the Run tab's own hand-rolled text field — digits
    only, everything else either an editing key or one of five
    reserved toggle mnemonics), press enter, and the file runs that
    many times back to back with whatever entry point and input file
    the Run tab currently has selected — genuinely the *same*
    settings, read fresh at run time, not a separate copy that could
    drift out of sync with what the Run tab shows.
  - **Deliberately untraced**: every iteration goes straight through
    `runFile` (`run.go`), the exact function `crust run` and the Run
    tab's own "run" action already use for their own command-line-style
    output — not `buildDebugView`/`debugger.Recorder`, whose
    per-statement tracing machinery would add real, systematic
    overhead to every single statement executed and badly distort
    exactly the timing numbers this feature exists to measure.
    `cmd/crust/benchmark_test.go`'s own `BenchmarkAoC2020Day1Part1` had
    already established this same "measure through the real CLI
    pipeline, not an isolated interpreter microbenchmark" principle for
    the identical reason (Phase 7, below) — reused here rather than
    reinvented.
  - **What gets measured**: wall-clock time via a plain
    `time.Now()`/`time.Since()` bracket, and memory via
    `runtime.MemStats.TotalAlloc` deltas (before/after each iteration)
    — cumulative bytes allocated for heap objects, which only ever
    increases, so a delta is accurate regardless of *when* a GC
    happens to run during or between iterations, the same technique
    `go test -bench -benchmem` itself uses internally. A run's own
    exit code is recorded too (`benchRun.failed`) but doesn't stop the
    batch or get excluded from the stats — a slow, failing run is
    still real data about the program, called out separately in a
    footer count rather than silently dropped.
  - **Two charts, not one, each on its own scale**: Runtime above
    Memory — the same reasoning that split the original KPI tab into
    Time and Memory tabs in the first place (this section, above, "per
    direct user feedback"): nanoseconds and bytes overlaid on one
    shared Y-axis would make neither legible, since neither metric's
    range has any relationship to the other's. Both charts share one
    toggle set (`benchShow [benchSeriesKindCount]bool`, keyed by a
    `benchSeriesKind` enum: raw/average/median/max/min) applied
    identically to each — "average" is one abstract concept that shows
    up on both charts with different underlying numbers, not two
    independent things to track and toggle separately, so there's
    exactly one legend, one set of `r`/`a`/`m`/`x`/`n` mnemonics,
    governing both.
  - **Median, specifically, as the "other stat that would be
    notable"**: more resistant than the mean to one slow outlier run
    (a stray GC pause, a scheduler hiccup on a shared machine) skewing
    the whole picture — the standard first reach for a benchmarking
    tool once you have more than a couple of samples, and directly
    responsive to the "any other stats" part of the request rather
    than only literally implementing the four explicitly named ones.
  - **The stats themselves** (`benchAvgOf`/`benchMedianOf`/
    `benchMaxOf`/`benchMinOf`) are one generic implementation each,
    not two near-duplicate copies for `time.Duration` and `uint64` —
    a `benchOrdered interface{ ~int64 | ~uint64 }` constraint covers
    both, since Duration is itself defined as `~int64` and both need
    identical arithmetic (sum, sort-and-pick-the-middle, min/max via
    `slices.Max`/`slices.Min`).
  - **The chart itself is hand-rolled ASCII/Unicode**, not a new
    dependency — consistent with this project's existing pie chart
    (`debug_style.go`'s `pieChart`, block characters filled by angle)
    and its own "add a dependency only when needed" policy already
    established for bubbletea/lipgloss themselves. A fixed-size grid
    of runes plus a parallel grid of `lipgloss.Color` (one color per
    cell, since a plain `[]rune` alone can't carry per-character
    color): the raw series draws as a solid stepped line (`●` at each
    data point, `│` filling the vertical gap to the previous point, so
    a value jump reads as a connected line rather than isolated dots);
    each shown reference line draws as a flat dashed row (`·` every
    other column) across the *entire* width, visually distinct from
    the raw line on sight. Reference lines paint over the raw series
    wherever they'd land on the same cell, in a fixed draw order
    (average, median, max, min) matching `benchSeriesInfo`'s own
    declared order — deterministic regardless of which subset happens
    to be toggled on, rather than depending on Go's randomized map
    iteration order (which is exactly why `benchShow`/`refs` are fixed
    -size arrays indexed by `benchSeriesKind`, not maps, throughout).
  - **A real bug, found only by looking at a rendered chart, not by
    any unit test**: the first version mapped the raw series onto the
    chart via `bucketAverage(raw, width)` alone, which returns only
    `min(width, n)` points — exact per-run values with no averaging
    when there are fewer runs than columns, the common case. Those
    points landed starting at column 0 and stopping at column
    `n-1`, leaving the rest of the chart's width blank, while the
    reference lines (drawn independently, always spanning the full
    width) kept going the whole way across — the raw line visibly
    crammed into the chart's left edge at a different effective
    horizontal scale than everything else on the same chart. No
    existing test caught this because every one of them asserted on
    line *count* or *content*, never on *where within the width* a
    line actually landed — exactly the class of bug a real rendered
    screenshot (here, a real pty session) catches and a unit test
    structurally can't. Fixed with a new `resampleForChart` that
    always produces exactly `width` points: `bucketAverage` (unchanged)
    when there are at least as many runs as columns, or linear
    interpolation between the two nearest raw points when there are
    fewer — stretching a short series smoothly across the *entire*
    chart width instead of leaving it crammed against one edge, so it
    now shares the same horizontal scale the reference lines always
    used. A dedicated regression test
    (`TestResampleForChartStretchesFewerRunsAcrossFullWidth`) pins
    exactly this case: two points resampled onto eleven columns must
    reach column 10, not stop at column 1.
  - **`maxBenchRuns` (1000)**: `benchCmd` runs synchronously inside one
    blocking `tea.Cmd`, the identical shape `runAllStoresCmd` already
    established for "run every entry point in sequence" — and
    bubbletea cannot process a quit keypress, or re-enter `Update` at
    all, until that `Cmd` returns. An accidental extra zero on the
    typed count (`10000` instead of `1000`) would otherwise hang the
    whole session with no way out until every last iteration finished
    on its own; the cap turns that into an immediate, recoverable
    error instead.
  - **Persistence**: the run count is remembered per file
    (`develState.BenchCount`, `restoreBenchCount`/
    `saveBenchCountBestEffort`), the same "reopening this file starts
    back where you left it" convention `store`/`input`/`runAll`
    already established — defaulting to 10 (a reasonable first-try
    sample size) when nothing's been remembered yet, preserving
    whatever else was already saved for that file rather than
    clobbering it.
  - Verified with table-driven Go tests (every stats helper including
    empty-input edge cases, `bucketAverage`/`resampleForChart` in both
    the downsample and stretch directions, chart rendering at a fixed
    size including the "nothing shown" placeholder and a flat-data
    divide-by-zero guard, every key binding including the toggle
    mnemonics *not* leaking into the numeric field, a real multi-run
    `benchCmd` execution against a real file on disk with a real
    non-zero-exit case, every error path, the persistence round trip)
    and a real pty-driven session: navigated to the Bench tab (via
    Shift+Tab twice from Time, deliberately avoiding tabbing *forward*
    through Editor, which auto-launches `nvim` and would steal the
    remaining keystrokes — the same trap an earlier Files-tab pty
    session in this same log already hit once), ran the default count,
    confirmed both charts rendered with real numbers and the shared
    legend, and confirmed toggling a series with `a` visibly removed
    it from the redrawn chart.
  - **The legend prints each reference line's actual computed value**,
    on direct follow-up request: "can you add values for the average
    and median lines?" — added to all four reference lines (`max`/
    `min` too, for the same reason and at essentially no extra cost,
    not just the two explicitly named), not the shared single legend
    the tab originally had below both charts. That single-legend shape
    couldn't have shown a value at all: Runtime and Memory need their
    own formatted number even for the identical series (`231.461µs`
    means nothing printed under a chart whose own numbers are in
    KiB), so `viewBenchLegend` became one call per chart, each passed
    the exact same `refs`/`format` pair `benchChart` itself was just
    called with — the printed value can never drift from the line
    actually drawn above it. `raw` still gets no value (there's no
    single number a per-run series could show), confirmed by a
    dedicated test asserting the string `"runs: "` followed by a
    formatted value never appears. `benchChartHeight`'s reserved-lines
    budget (`10`) grew to `12` to make room for a second legend line
    plus the blank line separating the two charts, verified against a
    real pty session showing both legends (`average: 231.461µs`,
    `median: 55.0KiB`, ...) rendered correctly beneath their own chart.
  - **Inspect mode** (`i`/`h`/`l`/`g`/`G`, `columnForRun`,
    `viewBenchInspect`), on direct follow-up request: "is there a way
    for the bench tool that we could somehow navigate inside the
    individual graphs and look at the stats of individual runs?" Every
    line the chart already draws deliberately blurs individual runs
    together — the four reference lines are single aggregate numbers by
    definition, and even the raw line's own points get resampled
    (`resampleForChart`, bucket-averaged or interpolated depending on
    run count vs. chart width) rather than plotted one-for-one — exactly
    the tradeoff that makes a trend legible and makes "what did run 47
    actually do" unanswerable from the chart alone. Inspect mode adds a
    `benchCursor` (a run index into `benchRuns`) and `benchInspect`
    (whether it's active) to `debugModel`, toggled with `i` (a no-op
    with no runs yet) and stepped with `h`/`l` (±1, clamped at both
    ends) or jumped with `g`/`G` (first/last) — deliberately not
    Left/Right or Home/End, which the run-count field already owns, so
    there's no ambiguity between "edit the count" and "move the cursor"
    the same reasoning that picked the `r`/`a`/`m`/`x`/`n` toggle
    mnemonics over reusing digits.

    The one real design problem: the chart's x-axis isn't 1:1 with run
    index once resampling is involved, so "which column is run *i* on"
    needed its own answer, not just "which column is the cursor at."
    `columnForRun(i, n, width)` inverts whichever of `resampleForChart`'s
    two mappings actually produced the line being looked at: when there
    are more runs than columns, it mirrors `bucketAverage`'s own
    `b*n/width` bucket boundaries, solved for "which bucket does run *i*
    fall in" instead of "which runs does bucket *b* average"; when there
    are fewer runs than columns, it mirrors the interpolation position
    formula, solved for the column instead of the value, so run *i*
    lands exactly on the same vertex `resampleForChart` interpolates
    every other column's value between; a single run centers in the
    chart, since there's no direction to place it toward. Both charts
    always share one `cursorCol` (computed once in `viewBench`, not
    twice), since it depends only on run count and chart width, never on
    which metric a given chart happens to show — the same "one shared
    toggle set" reasoning `benchShow` already established for the five
    line toggles.

    `benchChart` gained a `cursorCol` parameter (`-1` for none) that
    appends one extra row below the chart's own data rows: a caret
    (`^`) at that column in `colorSelected` (the same color
    `styleSelectedRow` already uses for "this is the selected thing"
    elsewhere — the Run tab's entry-point selector, the Files tab's file
    list), on its own row rather than recoloring whatever's already at
    that cell, so the marker stays visible and unambiguous even over a
    blank column (a toggled-off series, or a gap between two raw
    points). `viewBenchInspect` prints the readout line underneath both
    charts: the selected run's exact 1-based position, duration, and
    memory, plus a `FAILED` flag if that specific run's exit code was
    non-zero — pulled straight from `benchRuns[benchCursor]`, with no
    resampling or aggregation in the path at all.
    `handleBenchResult` clamps `benchCursor` (not resets it) against the
    new batch's length whenever fresh results land, so re-running with a
    smaller count doesn't leave the cursor pointing past the end, but a
    same-or-larger re-run keeps the same selected run without the user
    having to re-navigate. `benchChartHeight`'s reserved-lines budget
    grows from `12` to `16` while inspect mode is active — one caret row
    per chart plus the readout line and its own separating blank line —
    and stays at `12` otherwise, so non-inspect rendering is pixel-
    identical to before this feature existed.

    Verified with table-driven Go tests (`columnForRun` against all
    three cases — fewer/equal/more runs than width — `clampBenchCursor`'s
    edge cases including zero runs, every new key binding including h/l
    being inert outside inspect mode and not leaking into the count
    field, the caret row appearing only when a cursor is passed, the
    inspect readout appearing/disappearing with `benchInspect`, a
    failed run getting flagged) and a real pty-driven session: navigated
    to Bench, ran a batch, pressed `i` and confirmed a caret appeared
    under both charts at the same column with a matching readout line
    below, then confirmed `l`/`l` advanced the readout through
    consecutive runs, `G` jumped straight to the last run (caret at the
    chart's right edge), and `g` back to the first (caret at the left
    edge) — all against real per-run data, not a mock.
  - **Adaptive pie chart radius** (`pieRadius`, `cmd/crust/debug_tui.go`),
    one of six "what could I add to the develop tool?" ideas offered on
    direct request and picked to start on (along with a web
    playground) — closes out the pie-chart half of the debugger
    follow-ons stretch item TODO.md already flagged (the other half,
    Stepper search, is its own bullet below). The radius was a fixed
    `7` since the KPI tab first split into Time/Memory: fine on a
    normal terminal, but on a short one the chart's own height (`2r+1`
    circle rows, plus a growing legend line per KPI family) could run
    past the bottom, and `clampHeight` — built for exactly this class
    of overflow elsewhere — can only trim the rendered *result*, not
    shrink the chart back into the room actually available. `pieRadius`
    computes the radius from `m.height`/`m.width` and the caller's own
    KPI count, reserving the same "everything else this tab is already
    committed to printing" budget `stepperBodyHeight`/
    `benchChartHeight` already reserve for their own tabs (tab bar,
    header, chart title, the blank line `pieChart` itself leaves before
    its legend, one line per KPI, the stats block below, the help
    footer) — floored at 3 (smaller stops reading as a circle at all),
    capped at the original 7 (nothing asked for a *bigger* chart on a
    roomy terminal, only for a small one to stop losing its bottom
    rows), and separately bounded by width too, for the rare narrow-
    but-tall terminal. Verified with table-driven Go tests (roomy vs.
    short terminal, radius shrinking as KPI count grows, the width
    bound, the floor) and a real pty-driven session comparing a 50-row
    and a 15-row terminal side by side: the short one's circle visibly
    shrank and the legend plus the stats block beneath it stayed fully
    on screen, where before this change they'd have been the next thing
    `clampHeight` cut off.
  - **Export Bench results to CSV** (`e`, `benchExportCmd`,
    `cmd/crust/debug_bench.go`) — another of the same six ideas.
    `benchExportPath` swaps the debugged file's own extension for
    `.bench.csv` (`day01.crust` → `day01.bench.csv`, written right next
    to it), overwritten on every export rather than accumulating a
    history — a snapshot of "the last batch I ran," matching this tab's
    own best-effort run-count persistence rather than growing a pile of
    timestamped files nobody asked for. One row per run: 1-based index, raw
    duration in nanoseconds, bytes allocated, whether it failed —
    deliberately *not* the chart's own already-computed average/
    median/max/min, since a spreadsheet's own `AVERAGE`/`MEDIAN`
    formulas already derive those from the raw column, and repeating
    them here would just be a second copy that could drift out of sync
    with `benchAvgOf`/`benchMedianOf`/etc. Runs asynchronously through
    the same `tea.Cmd` → result-message shape every other action in
    this tab already uses (`benchCmd`/`benchResultMsg`, `runProgramCmd`/
    `runResultMsg`), reporting success or failure in a one-line
    `benchExportStatus` rather than either silently succeeding or
    crashing the TUI on a write error (an unwritable directory, a full
    disk) — cleared on the next batch of runs, since it describes data
    that's no longer what's on screen. Verified with Go tests (the path
    derivation, exact CSV content against known input including the
    failed-run boolean, an unwritable-directory error path, the status
    line showing and clearing) and a real pty-driven session: ran a
    batch, pressed `e`, confirmed the `"exported to ..."` status line,
    and diffed the written file's numbers against the chart's own —
    exact matches (`69058` ns in the CSV for the `69.058µs` the runtime
    chart's own max reference line showed).
  - **A variable/environment watch panel** (`v`, `viewStepperWatch`,
    `cmd/crust/debug_tui.go`) — the last of the same six ideas, and the
    one expected to matter most day to day: a step's "out" column only
    ever shows what that one statement itself evaluated to, never the
    surrounding variable state a real debugging session usually
    actually wants ("what was `x` when this went wrong").

    This one needed real plumbing underneath the UI, not just a new key
    binding. `object.Environment` gained `Snapshot() map[string]Object`
    (`internal/object/environment.go`): walks the scope chain outermost
    to innermost, letting each inner scope's assignment overwrite the
    map entry for a shared name — exactly the shadowing rule `Get`
    already implements by walking the *other* direction (inner first,
    falling outward) — flattening the whole chain into one map. This
    exists specifically because a live `*Environment` reference isn't
    safe to hold onto across time: its own `store` map keeps mutating
    as the run continues past the point it was captured at, so a
    debugger holding one and reading it later would silently show
    *future* values, not the ones at the moment it actually cared
    about. `Snapshot()` is deliberately shallow — it copies which
    `Object` each name currently points to, not a recursive deep copy
    of every value's own contents the way `object.DeepCopy` (`copy()`,
    SPEC.md §7) does — since `copy()` only ever runs once per call site
    while this needed to run on *every single traced statement*; the
    tradeoff is that a later in-place mutation of a still-shared List/
    Map/Set/Grid (`push`, `setAt`, `sprinkle`, `wrapReplace`, index
    assignment, ...) is still visible through an old snapshot, since
    only the *binding* was frozen, not the value underneath it —
    Integer/Float/String/Boolean/Tuple are all immutable in cRust
    (SPEC.md §2), so this caveat only ever touches those four
    container types, and only when something else still holds a
    reference to the exact same one.

    `trace.StepEvent` and `debugger.Step` each gained an `Env` field
    carrying that snapshot straight through the existing pipeline
    (`internal/interpreter`'s `evalTracedStatement` → `trace.StepEvent`
    → `internal/debugger`'s `Recorder.Step` → `Step.Env`) — no new
    plumbing path, just one more field riding along the one that
    already exists. Taken *after* `Eval` returns, the same "after the
    fact" timing `Out`/`Dur` already use, so a step's *own* effect (a
    new assignment, a loop variable's fresh binding) shows up in that
    same step's snapshot rather than only becoming visible one step
    later. Performance was the real open question before writing any
    of this — computing a flattened snapshot on every one of up to
    `DefaultMaxSteps` (20,000) traced statements sounded like it could
    regress `crust develop`'s own responsiveness — so it got measured,
    not assumed: `BenchmarkTraced`/`BenchmarkUntraced`
    (`internal/interpreter/trace_test.go`) before this change already
    showed traced running ~6.5x slower than untraced (from the timing
    and `trace.SizeOf` work already happening on every step); adding
    `Env: env.Snapshot()` moved that number by only run-to-run noise
    (12.10ms → 12.35ms on this session's own before/after runs) — the
    untraced path (`crust run`, the actual perf-critical one) is
    completely unaffected either way, since a nil `Tracer` still skips
    this whole branch with one check, same as always.

    `viewStepperWatch` renders the selected row's own `Env`, names
    sorted alphabetically for scanning, capped at `maxWatchLines` (8) —
    a deep call stack can have dozens of visible names, and an
    unbounded panel would fight `stepperBodyHeight`'s own row budget
    unpredictably — with a "… and N more" note past the cap. A frame or
    closing row has no `Step` of its own to read `Env` from (only
    individual statements are traced, not a frame's own entry state),
    so the panel says so explicitly rather than silently showing stale
    data from whichever step was last selected or nothing at all.
    `stepperExtraLines` grows by `maxWatchLines + 2` while the panel is
    on, the same fixed-worst-case reservation pattern the search field/
    status message already established for this same tab, and Bench's
    own inspect mode established before that.

    Verified with Go tests at every layer this touched:
    `object.Environment.Snapshot`'s shadowing and point-in-time
    behavior (a snapshot must not see a rebinding that happens after it
    was taken) in `internal/object`; the interpreter reporting each
    step's own effect in its own snapshot in `internal/interpreter`;
    `Step.Env` landing correctly (and a step not yet reached correctly
    *not* seeing a variable a later statement declares) in `internal/
    debugger`; the panel's rendering, truncation, frame-row placeholder,
    and `v` key handling in `cmd/crust`. And a real pty-driven session:
    ran a program with a top-level call to a `combine` recipe, watched
    the top-level call's row after it returned (showed `a`, `b`, `c`,
    and `combine` itself — a recipe value is a variable too), then
    stepped inside the recipe's own body and confirmed the panel
    correctly added the local `x`/`y`/`total` while keeping the
    enclosing `a`/`b`/`combine` visible too, and — the one that would
    have caught a shadowing or ordering bug — correctly did *not* show
    the caller's own `c`, since the call hadn't returned yet at that
    point in the actual recorded run.
  - **A full-value detail panel** (`o`, `viewStepperDetail`,
    `wrapRunes`, `cmd/crust/debug_tui.go`) — the sixth and last of the
    same batch of ideas. `shortInspect` (`debug_view.go`) — reused by
    every table row's own "out" column — caps a value's `Inspect()`
    text at `maxInspectRunes` so the column stays a fixed width, the
    right call for scanning the tree at a glance but a genuine loss for
    anything with a real tail: a 40-element List truncates the same way
    a one-word String would. `o` opens a panel showing the selected
    step's complete, untruncated `Inspect()` text instead of the
    table's own capped one, hard-wrapped across the panel's own width
    (`wrapRunes` — a plain fixed-width rune chunker, no word-boundary
    logic, since a raw `Inspect()` dump like `[1, 2, 3, ...]` has no
    natural word breaks worth preserving) rather than squeezed into the
    table's fixed column. Capped at `maxDetailLines` (10), the same
    fixed-worst-case reservation `maxWatchLines` already established
    for the watch panel one bullet up — `stepperExtraLines` grows by
    the same `+2` pattern for whichever of the two (or both) panels are
    currently on. Shares the watch panel's own frame-row guard: a frame
    or closing row has no `Step` to read `Out` from, so the panel says
    so rather than showing stale data from whichever step was selected
    last. Verified with Go tests (`wrapRunes`'s exact wrapping behavior
    including the empty-string and non-positive-width edge cases, the
    panel actually surfacing a value long enough that `shortInspect`
    would have cut it off, the frame-row placeholder, a nil `Out`
    reading as `nobox` rather than panicking) and a real pty-driven
    session: a 40-element List showed truncated with `…` in the table's
    own "out" column exactly as expected, then `o` showed all 150
    characters, correctly wrapped across multiple lines at the panel's
    own width rather than the table's.
  - **Bench regression baseline diff** (`b`, `benchBaseline`,
    `cmd/crust/debug_bench.go`) — the last of the same six ideas.
    "Did this get faster or slower since I last checked" needs
    something to compare *against*, and the chart's own reference
    lines only ever describe whichever batch happens to be on screen
    right now. `b` computes `benchBaselineFrom(m.benchRuns)` — the
    average duration and average allocation, the same numbers the
    chart's own `[a]` reference line already shows, via the existing
    `benchAvgOf` — and persists it through the same `develState`
    mechanism `BenchCount` already established
    (`restoreBenchBaseline`/`saveBenchBaselineBestEffort`, mirroring
    `restoreBenchCount`/`saveBenchCountBestEffort`'s own read-mutate-
    write shape so saving a baseline can never clobber the file's other
    remembered settings). Deliberately scoped to average only, not the
    other three reference kinds (median/max/min) the chart already
    tracks: those stay visible as absolute numbers on the current
    batch's own legend regardless, so a diffed copy of them too would
    be answering a question ("did the *typical* run get faster") the
    average alone already answers, for a UI cost every additional
    number adds.

    `benchDiffPct(v, baseline)` is the signed-percentage math
    (`(v-baseline)/baseline*100`), factored out mainly so the zero-
    baseline edge case (`ok=false` rather than computing `±Inf`) has
    exactly one implementation rather than one per call site — not
    that a real saved baseline is ever actually zero, but a hand-edited
    or corrupted state file could still produce one, and a stray
    `+Inf%` in the UI would be a strange way to find out. `viewBench-
    Baseline` renders the diff for whichever of duration/allocation
    still has a meaningful percentage, so a corrupted single field
    degrades to showing one metric's diff rather than hiding the whole
    line.

    The saved baseline value and the "baseline saved" confirmation
    message have two different lifetimes, and `handleBenchResult`
    treats them accordingly: `m.benchBaseline` is left untouched on a
    fresh batch (the whole point is comparing a *new* batch against an
    *older* saved one, so the baseline has to survive the exact event
    that would otherwise blow it away), while `m.benchBaselineStatus`
    is cleared the same way `benchExportStatus` already is — it
    specifically means "I just pressed b," not "a baseline currently
    exists," and leaving it up past the next run would misreport which
    batch it was actually confirming.

    Verified with Go tests (`benchBaselineFrom`'s average computation,
    `benchDiffPct`'s sign in both directions and its zero-baseline
    no-op, the persistence round trip preserving every other
    remembered setting alongside the new baseline, `'b'` requiring at
    least one run present, the diff line hidden entirely until a
    baseline actually exists, and the two-different-lifetimes behavior
    on a fresh batch) and a real pty-driven session: ran a batch on the
    Bench tab itself (not the Run tab's own single run, which populates
    a completely separate recording), pressed `b` and confirmed
    `+0.0%` (comparing the just-saved baseline against itself), ran a
    second batch, and confirmed the shown diff (`+22.3%` runtime,
    `+12.5%` memory) matched hand-computed percentages from the two
    batches' own displayed averages.
  - **A richer Stepper tab** (`cmd/crust/debug_tui.go`), on direct
    request: "can you do richer stepper inside the develop tool?"
    followed by a clarifying `AskUserQuestion` that narrowed it to
    three concrete directions, all three picked: search/filter the
    tree (the exact stretch item already flagged in TODO.md's Phase 6),
    jump to the next/previous failed step, and show each row's source
    line number. All three share one real design problem: the visible
    rows (`buildRows`) stop descending into any collapsed "N
    iterations" fold, so a match or failure sitting inside one is
    invisible to a naive row-by-row scan — exactly the case these
    features exist for, since a real failure is disproportionately
    likely to be buried in a loop's later laps, not its first three
    (the ones still shown unfolded before `foldFrom` kicks in).
    - **`flattenAll`** walks the *entire* tree, folds included — a
      second, low-level traversal alongside `buildRows`' own
      fold-aware one, existing specifically because search/failure-jump
      need to *find* a match regardless of fold state (the row list
      itself is the wrong data structure to search over) while still
      needing the fold-aware view for actually *displaying* it once
      found. **`expandPathTo`** is the bridge back: given a target node
      found via `flattenAll`, it walks the same tree again and marks
      every ancestor on the path to that target as expanded in
      `m.expanded` (the same map `toggleFold` already writes to) — so a
      subsequent `rebuildRows` makes the target's own row actually
      exist to land the cursor on. **`jumpToNode`** is the shared
      landing sequence both search and failure-jump call once they have
      a target: expand, rebuild, find the row, `clampAndScrollCursor`
      (pulled out of `moveCursor` for exactly this reuse, rather than a
      second copy of the same scroll-into-view math).
    - **`findNext(all, start, forward, match)`** is the shared "next
      matching node" search both features are built on: linear scan
      from `start`'s position in `all` (found by pointer identity,
      since `flattenAll`'s order is stable across calls — the recorded
      tree never mutates after a run finishes, only which folds are
      expanded does), wrapping around either end rather than stopping
      at the boundary — the same "keep going, don't just give up"
      behavior vim's own `/` and `n`/`N` train users to expect.
      `start == nil` (nothing selected, or a fresh recording) searches
      the whole list from the natural end for that direction.
    - **Search (`/`, `stepMatches`, `handleStepperSearchKey`)**: `/`
      opens a text field (the same hand-rolled `runInputModel` every
      other tab's own field already reuses) with its own dedicated key
      handler — the same "a query can contain letters bound to actions
      elsewhere" reasoning `handleRunTabKey`'s own doc comment already
      gives for the Run tab's input-file field. Enter confirms into
      `m.stepQuery` (kept around specifically so `n`/`N` can repeat the
      *same* search without retyping it, mirroring vim's own `/` then
      `n`/`N` two-step) and jumps to the first match from the current
      cursor position; an empty query is a no-op close, matching an
      empty vim `/` prompt. `stepMatches` matches case-insensitively
      against exactly the text `renderRow` already prints for that row
      (its label, plus a step's own output) — never a separate,
      invisible field — so a match is always recognizable the instant
      the cursor lands on it.
    - **Jump-to-failure (`f`/`F`, `isFailedStep`, `jumpToFailure`)**:
      `isFailedStep` is exactly `renderRow`'s own existing
      `!n.IsFrame() && n.Step.Failed()` check, factored out so both
      places can never drift apart on what "a failure" means. One real
      surprise found while testing this against a real failing program:
      `Failed()` is true for *every* statement an error bubbles up
      through, not only the one that actually raised it — an `order`
      wrapping a failing statement, and the `knead` loop around that,
      both read as "failed" too, since their own `Out` *is* that same
      propagated `object.Error`. This isn't a bug to fix (it's the
      exact reason `renderRow` already colors every one of those rows
      in `styleError`, not just the innermost one, so `f`/`F` staying
      consistent with what's already on screen is correct, if a shade
      noisier than "jump straight to the one true root cause" would
      be) — but it did break this feature's first test draft, which had
      assumed exactly one failing node per recording; fixed by testing
      against the *specific* culprit statement rather than an exact
      failure count.
    - **Line numbers (`stepLineText`)**: a new leftmost "line" column,
      right-aligned to match `size`/`time`/`self%`'s own existing
      style — `Step.Line()` for a real step, blank for a frame (a
      recipe call or loop lap has no single line of its own) or the
      rare zero-`Pos` step. `renderClosingRow`'s synthetic `// end ...`
      marker gets a matching blank line-column field too, so every
      row's other columns stay aligned regardless of which kind of row
      it is.
    - **`stepperExtraLines`/`stepperBodyHeight`**: the search field (or
      a confirmed-query reminder once it's closed) and/or a status line
      ("no matches for ...", "no failed steps") print above the column
      header when active, so the row-count budget `stepperBodyHeight`
      reserves grows by the same 2-line increments — computed by one
      shared helper (`stepperExtraLines`) rather than duplicating the
      exact same conditions in both the rendering code and the height
      math, which could otherwise drift out of sync the way
      `benchChartHeight`'s own inspect-mode reservation was careful to
      avoid.
    - Verified with table-driven Go tests (`flattenAll` seeing inside a
      fold that `buildRows` still hides by default, `expandPathTo`
      actually revealing it, `findNext`'s wraparound in both
      directions, `stepMatches` case-insensitivity, every new key
      binding including 'n' staying correctly overloaded between "new
      file" on the Files tab and "repeat search" on the Stepper tab)
      and a real pty-driven session: ran a 5-lap loop whose 4th lap
      divides by zero through the Run tab (Shift+Tab three times from
      Time to reach Run without ever crossing Editor, the same
      established trap-avoidance this log keeps reusing), then tabbed
      forward the long way around to Stepper (through Bench/Files/Time/
      Memory, same reasoning) and confirmed: line numbers rendered
      correctly; `f` expanded the fold and landed exactly on
      `y = idiv(1, 0)`; `/idiv` found the next match after it (the
      wrapping `order` statement, since its own propagated error text
      also contains "idiv"); and `F` stepped back to `y = idiv(1, 0)`
      again.
- **Web playground** (`crust bake playground`), on direct request:
  "go ahead and start on those and the web playground" — a new,
  separate idea added to the same brainstorm that produced the six
  Bench/Stepper features above, not one of the six itself.
  - **`internal/runner` extraction**: the actual design problem wasn't
    the browser side, it was that `run.go`'s `runFile` — lex, parse,
    Eval, then resolve and call a `store`/`store_<name>` entry point —
    needed to run inside a `js && wasm`-build-tagged `cmd/wasm`
    binary, a separate `package main` that can't import `cmd/crust`'s
    unexported functions. Rather than reimplement that logic a second
    time (the exact kind of drift this project has been careful to
    avoid — see `debug.go`'s `runDebugEntryPoint` comment, which
    already called out mirroring `run.go`'s `runEntryPoint` by hand as
    a wording-duplication risk), `runEntryPoint`/`collectEntryPoints`/
    `storeFlags`/`reportRuntimeError` and a new top-level `Run`
    function moved verbatim into `internal/runner`, exported as
    `runner.Run`/`runner.CollectEntryPoints`/`runner.StoreFlags`.
    `debug.go` and `debug_view.go` — which already called
    `collectEntryPoints`/`storeFlags` directly, being in the same
    package as the old unexported versions — were repointed at the
    new package's exports instead of getting their own copies.
    `run.go`'s own `runFile` shrank to "read the file, hand `src` to
    `runner.Run`."
  - **`msgPrefix` instead of a hardcoded `"crust run: "`**: every
    message `Run` writes to stderr is prefixed with a caller-supplied
    string rather than the literal `crust run: ` the original inline
    code used. `run.go` passes `"crust run: "` (byte-for-byte
    identical existing output — verified by leaving every one of
    `main_test.go`'s/`examples_test.go`'s/`benchmark_test.go`'s
    existing `runFile`-based tests unchanged and green), while
    `cmd/wasm/main.go` passes `""` and lets the browser side show a
    bare `line:col: message`. The one deliberate, cosmetic behavior
    difference between the CLI and the playground; everything else
    (which entry point runs, what counts as an error, exit-code-shaped
    return values) is identical by construction, not by convention.
  - **`cmd/wasm/main.go`** (`//go:build js && wasm`) exposes a single
    `crustRun(source, storeFlag, stdin) -> {stdout, stderr, code}`
    global via `syscall/js`, backed by `runner.Run` writing into a
    `bytes.Buffer` rather than streaming — the playground runs a whole
    program synchronously per click, so there's no terminal on the
    other end for `deliver`'s real-time output to matter the way it
    does for `crust run`/`develop`'s Run tab. Because `syscall/js` only
    builds for `GOOS=js GOARCH=wasm`, the build tag keeps this package
    invisible to a normal host build — confirmed `go build ./...`
    silently skips it on linux/amd64 rather than failing, the same way
    Go treats any package whose build constraints exclude every file
    for the current target.
  - **Checked-in build artifacts, not a required pre-build step**:
    `cmd/crust/playground.go` embeds `playground_assets/{index.html,
    wasm_exec.js, crust.wasm}` via `go:embed`, and `go:embed` needs the
    embedded files to exist at compile time — so `crust.wasm` is built
    now and committed, rather than left for whoever builds `crust` next
    to generate first (which would break `go build ./...` working out
    of the box for a fresh clone, the same bar every other embedded-FS
    feature in this codebase — `internal/docsite`, the tree-sitter
    grammar — already clears). `scripts/build-wasm.sh` regenerates both
    files after any language change: `GOOS=js GOARCH=wasm go build -o
    playground_assets/crust.wasm ./cmd/wasm/`, then copies
    `wasm_exec.js` from `$(go env GOROOT)/lib/wasm/` (Go 1.24's location
    for it; falls back to the pre-1.24 `misc/wasm/` path) — copied
    rather than hand-written, since it has to match the exact Go
    version that compiled the `.wasm` down to the byte, and the
    toolchain already ships the correct one.
  - **`playgroundHandler`** re-roots the embedded FS with `fs.Sub`
    before handing it to `http.FileServer`, so `index.html`/
    `wasm_exec.js`/`crust.wasm` serve at `/`, `/wasm_exec.js`,
    `/crust.wasm` rather than nested under `/playground_assets/` —
    the embed directory name is an implementation detail of where the
    files happen to live in the repo, not part of the served URL
    shape.
  - **`runBakePlayground`/`servePlayground`** split the same way
    `bake.go`'s `runBakeDocumentation`/`serveDocumentation` already do
    — specifically so a test can bind `127.0.0.1:0` for a real,
    OS-assigned ephemeral port and exercise actual serving (banner
    text, real HTTP responses for all three assets, shutdown behavior
    on `Close()`) without needing to know or guess which port a fixed
    default resolves to. `defaultPlaygroundPort` (4748) sits one above
    `defaultDocsPort` (4747) so `bake documentation` and `bake
    playground` can both run at their defaults simultaneously — the
    two embedded-site features were never going to collide on purpose,
    but there's no reason to make them fight over one port by
    accident.
  - **`index.html`** reuses `internal/docsite`'s exact pizza-crust
    color palette (light/dark via `prefers-color-scheme`) for visual
    consistency between the two `bake` subcommands, but is otherwise a
    small, dependency-free static page: a source editor pane, a stdin
    box, a `--store` field, and a Run button (also bound to
    Ctrl/Cmd+Enter) wired to the `crustRun` global `wasm_exec.js`'s
    `Go` class exposes once `WebAssembly.instantiateStreaming` resolves
    against `crust.wasm`. No client-side framework, no build step for
    the HTML/JS itself — matching `internal/docsite`'s own "no JS
    beyond what one page genuinely needs" restraint.
  - **The infinite-loop caveat**: `crustRun` runs on the browser's main
    thread (the standard `syscall/js` shape — no Worker, no timeout),
    so a pasted-in program with an infinite loop hangs the tab until
    reloaded. Documented rather than engineered around: running the
    interpreter inside a Web Worker would need its own message-passing
    protocol and a duplicate WASM instantiation path, a real jump in
    complexity for a project-status feature whose main goal is "try
    cRust without installing anything," not "safely sandbox untrusted
    third-party code" — the same scoping judgment call as skipping a
    resizable pie-chart radius until a real terminal-size complaint
    shows up.
  - Verified with Go tests (`internal/runner`'s own suite reproducing
    every case `main_test.go`'s old inline `runFile` tests already
    covered — bare `store`, `--store` selection, unknown `--store`,
    no-default-lists-options, parse errors, runtime errors both at
    top-level and inside an entry point, the empty-`msgPrefix` case —
    plus `playground_test.go` mirroring `bake_test.go`'s own
    port-parsing-table and real-HTTP-serving conventions, including
    fetching all three embedded assets over an actual ephemeral-port
    listener) and, since this is the project's first genuinely
    browser-side feature, real Playwright-driven verification against
    the pre-installed Chromium rather than only a `curl` health check:
    launched `crust bake playground`, loaded the page, waited for
    `#status` to flip to `ready` (confirming
    `WebAssembly.instantiateStreaming` actually resolved), ran the
    default program and got `hello, pizza`, switched to a
    `store_part1`/`store_part2` program with `--store=part1` and real
    stdin text and confirmed only `store_part1`'s output came back, and
    ran `x = 1 / 0` and confirmed the runtime-error path reported
    `1:7: division by zero` with no path prefix (the `msgPrefix=""`,
    `path=""` case) — all three outcomes matching what the identical
    source produces through `crust run` itself.
- **`crust game`** (`cmd/wasmgame`, `cmd/crust/game.go`,
  `cmd/crust/game_assets/`), on direct request — "how hard would it be
  to add a game dev library" followed by "what if I wanted it to
  develop for pixijs so I could use it in the browser," resolved (after
  sketching the bridge/API design in conversation first) into: reuse
  the web playground's WASM-in-the-browser approach rather than build a
  second cRust-to-TypeScript compiler backend from scratch, since
  `crust bake playground` already proved the WASM path works and a
  transpiler would mean a whole new codegen target plus a parallel JS
  reimplementation of every stdlib builtin.
  - **A genuinely different execution model from the playground, not a
    reskin of it.** `crustRun` (`cmd/wasm`) runs a whole program to
    completion once per call — exactly right for "paste code, see
    output," exactly wrong for a game, which has to keep running with
    its own state (positions, score, anything declared at the top
    level) surviving between many short calls, one per browser
    animation frame. So this is a second WASM binary
    (`cmd/wasmgame`, `js && wasm`-build-tagged the same way
    `cmd/wasm/main.go` is) with three exports instead of one:
    `crustGameInit(source)` parses and evaluates top-level code once
    against a persistent `*interpreter.Interpreter`/`*object.Environment`
    pair held in package-level vars (there's only ever one game running
    in a given WASM instance — one browser tab, one page — the same
    "only one of these exists" reasoning `cmd/wasm/main.go`'s own
    single `crustRun` export already relies on); `crustGameFrame(dt)`
    calls whatever `onFrame(fn)` registered, once per
    `requestAnimationFrame` tick the page drives; `crustGameKey(name,
    down)` updates the pressed-keys set `keyDown()` reads, from the
    page's own keydown/keyup listeners. There's no `store`/
    `store_<name>` entry-point resolution at all here — a game has no
    one-shot "answer" to compute, so top-level code (spawn shapes,
    call `onFrame`) doubles as setup instead, the same "top-level code
    runs first" every cRust program already has (SPEC.md §9).
  - **New builtins live only in `cmd/wasmgame`, never in
    `internal/builtins`.** `rect`/`circle`/`setPos`/`setRotation`/
    `setScale`/`destroy`/`keyDown`/`stageSize`/`onFrame`
    (`cmd/wasmgame/builtins.go`) are a second table, merged into a
    fresh `interpreter.New(...)`'s own `.Builtins` map (a plain
    exported field) right after construction, rather than folded into
    `internal/builtins.New` itself — that package is imported by the
    ordinary CLI interpreter too, which has no `__crustGameHost` to
    call out to and would gain nine builtins meaningless outside a
    browser tab. `onFrame(fn)` doesn't type-check that `fn` is actually
    callable up front, the same posture `map`/`filter`/`reduce`'s own
    injected `call` parameter already takes — an uncallable value
    surfaces as an ordinary cRust runtime error the moment
    `crustGameFrame` first tries to call it, not a special case here.
  - **Handles are opaque `Integer` IDs, not a new `object.Object`
    type.** Go allocates them (a package-level counter) and calls out
    to the page's own `__crustGameHost.rectCreate(id, ...)`; the JS
    side owns the real `PIXI.Graphics` object, keyed by that same ID in
    a `Map`. Neither side ever holds a live reference into the other's
    world across a call boundary — `setPos(handle, x, y)` is a thin
    relay (`host().Call("setPos", id, x, y)`), not a wrapped `js.Value`
    living inside an `object.Object`, which would need its own
    `Type()`/`Inspect()`/GC-lifetime story for one browser-only feature.
    Calling a builtin on a `destroy`ed handle is a silent host-side
    no-op (JS just won't find the ID), not a cRust runtime error — kept
    out of the interpreter loop on purpose, the same "the host owns the
    real object, and objects don't need cRust guarding what a JS `Map`
    already guards for free" trade every handle-based builtin here
    makes.
  - **PixiJS is vendored, not CDN-loaded** (`cmd/crust/game_assets/pixi.min.js`,
    `third_party/pixijs/NOTICE.md`) — pulled from the `pixi.js` npm
    package (`registry.npmjs.org` sits outside this environment's own
    outbound-proxy restrictions, unlike arbitrary CDN hosts, which is
    what actually made vendoring practical here) rather than a
    `<script src="https://...">` tag, so `crust game` keeps the same
    "nothing fetched at request time" offline posture `crust bake
    playground` already has for `crust.wasm`/`wasm_exec.js`. PixiJS 8's
    `Application.init()` is async, but every `__crustGameHost` method
    is called *synchronously* from Go (`syscall/js` has no way to await
    a call out to JS), so `index.html` creates and awaits the one
    `PIXI.Application` up front, alongside the WASM instantiation
    (`Promise.all([...])`), and keeps the Run button disabled until
    both resolve — by the time any game builtin can possibly run, the
    app is already a real, ready object, never a pending promise.
  - **Shapes only for this first cut, not textures.** `rect`/`circle`
    draw with `PIXI.Graphics` (`.rect(...).fill(color)`/
    `.circle(...).fill(color)`, PixiJS 8's chained API — it replaced
    v6/v7's `beginFill`/`drawRect`/`endFill`), no image/sprite-sheet
    loading. Deliberate scope cut: browser texture loading is
    inherently async (`Assets.load` returns a promise), and the
    interpreter's own call-out mechanism is synchronous end to end —
    supporting it would mean either blocking `crustGameInit` until
    everything's preloaded (fine for a fixed startup asset list, a
    real constraint for anything loaded mid-game) or inventing a
    "some builtin calls need a callback" exception to every other
    builtin's synchronous contract. Punted rather than half-solved for
    an MVP explicitly scoped to ship fast and iterate from user
    feedback.
  - **`crust game [file.crust] [-p PORT]`** (`cmd/crust/game.go`)
    mirrors `crust bake playground` almost exactly —
    `parseGameArgs`/`runGame`/`serveGame` split the same way
    `parsePlaygroundArgs`/`runBakePlayground`/`servePlayground` already
    are, `game_assets` embedded and re-rooted with `fs.Sub` the same
    way, `defaultGamePort` (4749) sitting one above the playground's
    own default — with one addition: an optional file argument, read
    server-side and served back through a small `/source.json`
    endpoint (`{"source": "..."}`) that `index.html`'s own JS fetches
    to preload the editor. A separate endpoint rather than templating
    the source into `index.html` itself specifically avoids writing
    (and testing) HTML/JS-escaping logic for arbitrary user source —
    `encoding/json` already escapes a string correctly for embedding
    in a script-fetched response, so there's nothing bespoke to get
    wrong.
  - Verified with Go tests mirroring `playground_test.go`'s own
    conventions (`parseGameArgs`' table, real HTTP fetches of all three
    embedded assets over an ephemeral-port listener, `/source.json`
    both with and without a preloaded file, listen-failure and
    missing-file error paths, the subcommand dispatch's own bad-port
    and too-many-files cases) and, since this is a second genuinely
    browser-side feature, real Playwright-driven verification against
    the pre-installed Chromium: loaded the page, waited for `#status`
    to flip to `ready` (confirming both the WASM instantiation *and*
    the PixiJS `Application.init()` promise resolved), clicked Run on
    the default arrow-key demo, wrapped `__crustGameHost.setPos` from
    the test's own side to record every position call made, held
    `ArrowRight` for 500ms and confirmed the tracked sprite's x moved
    from ~324 to ~434 — a ~110px delta matching the demo's own `speed =
    220`px/s at 0.5s almost exactly — and confirmed it stopped moving
    within one frame of releasing the key; separately confirmed
    `deliver("hello from cRust")` reaches the on-page console panel,
    and that a deliberately malformed program (bare `if` instead of
    `order (...)`) reports real parse errors there rather than failing
    silently. An initial demo written with a bare `if` (not
    `order (...)`, cRust's own keyword — see SPEC.md §4) is exactly the
    kind of mistake this verification pass exists to catch rather than
    ship unnoticed in the one program most people will paste first.
  - **`examples/game/game_survivors.crust`**, on direct follow-up request —
    "add an example game... vampire survivors but just with a basic
    attack, a five minute limit, and just circles instead of sprites."
    Added one tenth builtin, `random()` (`math/rand.Float64()`, no
    explicit seeding needed — Go's global source has been auto-seeded
    from real entropy since 1.20): the one genuine gap the nine
    existing game builtins had for this — every other builtin in this
    codebase is a pure function of its arguments (AoC puzzles have one
    correct answer), but enemy spawn points and wait times have no
    business being deterministic, and hand-rolling an LCG in cRust
    itself for one example program would be solving the wrong problem.
    Kept in `cmd/wasmgame`, not `internal/builtins`, same reasoning as
    the other nine. Design choices worth calling out: entities
    (enemies, projectiles) are Maps (`{"handle": ..., "x": ..., "y":
    ...}`), not a new type — cRust's `List`/`Map`/`Set` are reference
    types (see the "Committed to now" performance section above), so
    mutating `e["x"] = ...` inside a `knead` loop updates the same Map
    stored in the enclosing list in place, no rebuild-and-reassign
    needed for simple field updates; only actual *removal* (a dead
    enemy, an expired or spent projectile) rebuilds the list via
    `filter`-shaped `knead`-and-`push`, since cRust lists have no
    in-place "remove while iterating" primitive. The basic attack
    always targets whichever enemy is nearest at the moment it fires (a
    straight shot at that position, not a homing missile) — the same
    "Magic Wand"-style no-aim-required weapon Vampire Survivors itself
    starts every run with, and it needs no trigonometry (no `sin`/`cos`
    builtin exists yet): both "walk toward the player" and "fly toward
    a fixed point" are just a `dx, dy` difference normalized by
    `sqrt(dx*dx + dy*dy)`, the exact same shape `manhattan`-adjacent
    grid code already uses elsewhere in this codebase, just with a real
    (not taxicab) distance. No on-stage text builtin exists yet (see
    the "shapes only for this first cut" note above), so the 5-minute
    clock is a `rect` whose `setScale` shrinks toward zero each frame
    (`(gameDuration - elapsed) / gameDuration`) rather than a countdown
    number, and score/status is a periodic `deliver()` heartbeat (every
    ~10s) into the console panel instead of an on-screen HUD. Verified
    the same way as the runtime itself: real Playwright-driven
    headless verification, `crust game examples/game/game_survivors.crust`
    with an untouched (never-moved) player — long enough to prove
    spawning/attacking/collision/scoring all actually fire (46
    `circleCreate`/44 `destroy` calls and `t=20s score=12 enemies=0` in
    the console log over ~25s, not just "no errors were logged") rather
    than only confirming the game-over path a real player would
    normally trigger by moving badly.
  - **Fleshed out into a real 2D engine** (`cmd/wasmgame/builtins.go`,
    `cmd/crust/game_assets/index.html`, `cmd/crust/game.go`), on direct
    follow-up request — "can you flesh out the game engine to have
    popular features used for creating 2d games," scoped via
    `AskUserQuestion` to `crust game` alone (not `crust studio`, whose
    character-cell medium doesn't have a meaningful equivalent for
    textures/audio/camera anyway) and to four feature groups: collision
    detection, text/label rendering, images/audio, and camera/tilemaps
    — all four were picked, so all four shipped together rather than
    the usual single-feature increment.
    - **Go starts tracking real per-handle state for the first time.**
      Every builtin before this was a pure relay: Go allocated an ID
      and forwarded the call, the host owned every fact about the
      object (position, size, kind) with nothing mirrored back. That
      breaks down for `overlaps()`, which needs to run every frame,
      for potentially many pairs, without paying a `syscall/js` round
      trip per check -- so a new package-level `entities
      map[int64]*gameEntity` (kind, `w`/`h`/`r`, `x`/`y`) now lives on
      the Go side too, updated by `rectFn`/`circleFn`/`spriteFn`
      (which record kind+size at creation) and `setPosFn` (via the new
      shared `moveEntity` helper, also reused by `loadTilemap` so
      placing a tile doesn't need to round-trip through
      `object.Object` argument boxing just to immediately unbox it
      again). A `kind` of `""` (text, or a stale/unknown handle) is a
      real, distinct state `overlaps()` checks for and refuses on,
      rather than a hitbox silently collapsing to a zero-size point.
    - **`overlaps()` does real geometry, not just AABB-everywhere.**
      Circle-vs-circle is exact (distance between centers vs. the sum
      of radii); rectangle-vs-rectangle is an axis-aligned box overlap
      (rotation from `setRotation` is deliberately ignored -- the
      standard simplification any lightweight 2D engine's basic
      collision helper makes, real rotated-hitbox precision being a
      much bigger, separate feature); circle-vs-rectangle (including a
      `sprite`, which gets a rectangle hitbox from its own given `w`/
      `h`) is a real closest-point-on-rectangle-to-circle-center test,
      not an approximation -- the standard building block for correct
      circle/AABB collision, factored into its own
      `closestPointOnRect` helper.
    - **`sprite`/`sound` resolve the async-loading problem flagged as a
      known gap when `crust game` first shipped**, via the same split
      `spriteCreate` (`index.html`) already established:
      `PIXI.Sprite(PIXI.Texture.EMPTY)` is created and added to the
      scene *synchronously*, so the handle cRust gets back is usable
      (`setPos`/`setRotation`/`overlaps`/...) the instant `sprite()`
      returns, exactly like every other builtin here -- `PIXI.Assets.load`
      then swaps the real texture into that same, already-positioned
      object once the network actually resolves, with an `objects.get(id)
      === s` guard against a `destroy()` (or an `r`-triggered restart's
      whole-scene teardown) racing ahead of a still-in-flight load and
      resurrecting a texture onto an object nothing points at anymore.
      `sound()` doesn't share `objects` with visual handles at all (a
      separate `sounds` Map) -- `setPos`/`overlaps`/etc. on an audio
      handle would be nonsensical, so there's no reason to make them
      silently no-op instead of the handle namespace itself keeping the
      two apart. `playSound` clones the `Audio` element per call
      (`cloneNode()`) so overlapping plays (a pickup sound firing again
      before the last one finished) never cut each other off, the
      standard trick for short SFX; a rejected autoplay promise (real
      browsers refuse audio before any user gesture on the page) is a
      silent no-op, not a surfaced cRust error -- a game reasonably
      calling `playSound` before the player has clicked anything
      shouldn't crash over a browser policy it has no way to detect or
      route around.
    - **`sprite`/`sound` need actual files to load, which meant
      teaching `crust game` to serve more than its own embedded
      assets.** `gameHandler` (`game.go`) now takes an `assetDir` (the
      directory containing whatever `.crust` file `crust game` was
      pointed at) and falls back to `http.FileServer(http.Dir(assetDir))`
      for any path its own embedded FS doesn't recognize (checked via a
      real `fs.Stat`, not a try-and-inspect-the-status-code guess) --
      `sprite("cat.png", ...)`/`sound("hit.wav")` then resolve as
      ordinary relative URLs against the running page. Embedded assets
      always win a name collision on purpose (verified with a test
      planting a local `wasm_exec.js` and confirming the real one still
      serves) -- realistically only reachable by accident, and the
      unsurprising choice either way is serving the page that actually
      makes the rest of the tool work.
    - **The camera is one `PIXI.Container`, not per-object math.**
      Every spawned object now joins a `world` Container sitting
      between the stage and everything a program spawns (previously
      objects were added directly to `app.stage`); `setCamera(x, y)`/
      `setCameraZoom(zoom)` (`applyCamera` in `index.html`) just
      reposition and rescale that one Container so `(x, y)` in world
      space lands at the viewport's own center, at the given zoom.
      `setPos` itself is deliberately never affected by the camera --
      it always sets world-space position, camera or no camera, the
      same "screen space and world space are different axes" split
      `stageSize()`'s own doc comment already draws (`stageSize`
      reports the viewport's raw pixel size, not the world, and that
      stays true here too).
    - **`loadTilemap` is pure Go sugar, no new host method at all.**
      It reuses `grid(s)`'s own established convention (a multi-line
      String, one character per cell) rather than inventing a second
      "ascii map" format, and a `palette` Map from character to hex
      color -- a character missing from `palette` spawns nothing (the
      standard "unmarked means walkable floor" convention every ascii
      roguelike map already uses). Every tile is spawned via the same
      `spawnRect` helper `rect()` itself now calls, so
      `setColor`/`destroy`/`overlaps` all work on an individual tile
      exactly like any other rect handle -- there's no separate "tile"
      concept for a user to learn, and nothing new for `index.html` to
      implement on the host side at all.
    - Verified with real Playwright-driven headless verification
      against a purpose-built program exercising all six additions at
      once (two overlapping circles, two non-overlapping rects, a
      `sprite()` loaded from a real temp-file image, a `sound()` loaded
      from a real temp-file WAV, a 3x3 `loadTilemap`, a panned+zoomed
      camera, and a `text()` label showing the live `overlaps()`
      results) served through a real `assetDir` fallback: screenshots
      confirm the two circles visually overlapping matches
      `circleOverlap=stuffed` in the rendered label text, the two rects
      correctly reported as not overlapping, the tilemap block and the
      camera-panned/zoomed rect positions rendering where expected, and
      -- after simulating held spacebar input -- the on-stage label
      updating live from "score: 0" to "score=13" across real animation
      frames, confirming `setText` mutates in place rather than
      requiring a destroy+recreate. No sprite/sound-related console
      errors surfaced in any of it, only the expected favicon 404 and
      software-WebGL warnings headless Chromium always logs. Also
      covered by new Go tests for the asset-directory fallback
      (`TestGameHandlerServesAssetDirFallback`,
      `TestGameHandlerEmbeddedAssetsTakePriorityOverAssetDir`).
- **`crust studio`** (`cmd/crust/studio.go`, `studio_tui.go`,
  `studio_builtins.go`), on direct follow-up request — "what about
  something like the develop tool: a bubbletea application that I can
  use as a game dev studio," resolved (after a short design exchange
  about resolution/architecture) into a terminal-native counterpart to
  `crust game`: same persistent-interpreter-plus-per-frame-callback
  idea, rendered directly into the terminal via the exact
  bubbletea/lipgloss stack `crust develop` already uses instead of a
  browser tab.
  - **No WASM boundary at all, unlike `crust game`.** This is the one
    genuine architectural simplification the terminal host buys: since
    there's no browser process to cross into, the game builtin table
    (`studioBuiltins`) is ordinary Go code inside the normal `crust`
    binary — no `js && wasm` build tag, no `syscall/js`, no second
    binary, no JS bridge object for Go to call out to. `studioState`
    (entities map, pressed-keys map, `onFrame` callback, current
    `cols`/`rows`) is held as a `*studioState` field on `studioModel`
    rather than the package-level vars `cmd/wasmgame/builtins.go`
    needs — bubbletea's own "`Update` returns a (possibly copied)
    model" convention would make package-level vars pointless *and*
    unnecessary here, since a pointer field's target, and the maps it
    points at, share their backing storage across however many times
    the containing `Model` value gets copied around; builtin closures
    just capture that one `*studioState` and every mutation is visible
    to whatever `View()` renders next, no extra plumbing required.
  - **A different builtin vocabulary on purpose, same handle
    lifecycle.** `cell(char, color)` instead of `rect`/`circle` — a
    terminal cell is the drawing primitive here, one glyph per grid
    position, not a shape to be filled — but `setPos`/`destroy`/
    `keyDown`/`stageSize`/`onFrame`/`random` keep the exact same names
    as `crust game`'s own table (see `studio_builtins.go`'s own
    package doc comment), so anyone who's used one tool already knows
    most of the other's shape, without pretending the two ever share
    source code (they don't — different vocabularies, different hosts,
    never loaded into the same interpreter).
  - **`keyDown` can't be level-triggered here, and says so.** A
    terminal has no key-release escape sequence at all — raw-mode
    keypresses report only that a key was struck, never that it was
    let go — so faithfully porting the browser's "true while held"
    semantic is impossible, not just harder. Rather than fake it with
    an arbitrary timeout window (considered and rejected: real
    terminal OS-level key-repeat has its own initial-delay-then-fast-
    repeat rhythm, typically a 300-500ms pause before repeating starts,
    which a short timeout would misread as "key released" mid-hold),
    `keyDown(name)` is honestly defined as "was `name` pressed at any
    point since the *previous* `onFrame` call" — `tea.KeyMsg` sets
    `state.pressed[name] = true`, and the tick handler clears the
    whole map right after each `onFrame` call. Holding a key still
    reads as roughly "held" in practice, riding on the terminal's own
    key-repeat, but a single tap reads `true` for exactly one tick, not
    fading out over a guessed timeout — a per-frame-boundary contract
    is simpler to reason about and to document accurately than a
    magic-number window would have been.
  - **`term.GetSize` before the `Program` ever starts, not
    `WindowSizeMsg`.** A game's top-level code runs immediately (there
    being no `store`/`store_<name>` entry point to defer to, same as
    `crust game`) and very likely calls `stageSize()` right away to
    size everything against — but bubbletea only delivers
    `WindowSizeMsg` through its own `Update` loop, which doesn't start
    until *after* `Init()` returns, so evaluating the program inside
    `Init()` would see a still-zero stage. `runStudio` instead calls
    `github.com/charmbracelet/x/term`'s `GetSize` directly against
    `stdout`'s fd, synchronously, before `loadStudioProgram` (and
    therefore the program's own top-level code) ever runs — the same
    already-vendored package `debug_tui.go`'s own resize handling
    uses, just called once up front here instead of only reactively.
  - **`studioMinCols`/`studioMinRows` clamp a degenerate reported
    size**, found by the feature's own pty verification pass rather
    than reasoned out in advance: a raw pty that never receives a
    `TIOCSWINSZ` call reports a 0x0 (or near-zero) window, which
    `term.GetSize` dutifully returns — and an ordinary game (a border
    drawn at the stage's own edges, movement bounds-checked against
    `stageSize()`) would fail its own first bounds check on the very
    first tick against a 0x0 stage, ending instantly with no visible
    cause. Clamping to a floor (20 cols, 8 rows) regardless of what the
    terminal claims turns "looks completely broken" into "renders
    correctly, just clipped, in a pathologically small or
    misconfigured terminal" — worth guarding explicitly rather than
    trusting every real-world terminal negotiates a sane size, the
    same defensive instinct as `debug_tui.go`'s own `clampHeight`.
  - **Loading is shared between the initial run and every `r` restart**
    (`loadStudioProgram`: lex, parse, build a fresh interpreter with
    `studioBuiltins` merged in, `Eval` top-level code) but their
    *failure handling* deliberately differs. A bad file on the initial
    `crust studio file.crust` fails before the `Program` ever starts —
    prints the error, exits 1, the same "fail before handing over the
    screen" posture `emptyDebugView` already has for `crust develop` —
    since there's no already-running session to protect. A bad file
    reached via `r` (edited in another window while `crust studio` was
    already open) instead leaves the *previous*, still-working
    `interp`/`state` in place and only sets `m.err`. Verified as two
    distinct, deliberately different test cases
    (`TestStudioModelRestartOnBrokenFileShowsErrorAndKeepsRunning`
    alongside a plain `runStudio`-on-a-bad-file test), not one
    "handles errors" test standing in for both.
  - **`deliver()` output is one line, not a scrollback** — a
    `studioLogWriter` (wrapping `interpreter.New`'s own `output`
    parameter, same mechanism `crust game`'s `consoleWriter` uses for
    `console.log`) keeps only the most recent line in
    `state.lastLog`, shown on the help bar. There's no browser-page
    two-pane layout to spare a console panel's worth of terminal real
    estate for here — a full-screen game already claims the whole
    frame, and a one-line "what did it last print" is enough to debug
    a value while iterating without turning the help bar into a log
    viewer.
  - **A verification bug the pty pass itself caught**: `View()`
    originally concatenated the help bar directly onto `renderStage`'s
    own output with no separating newline between them — invisible in
    a naive flat-text capture of the raw byte stream, but glaringly
    obvious once verification switched to `pyte` (a real terminal
    emulator reconstructing actual on-screen state) after the initial
    naive capture produced results that couldn't be explained by
    "wrong terminal size" alone: the help bar's text was landing on the
    *same row* as the stage's last line, past the terminal's own
    width, rather than on its own line beneath it. The naive
    accumulate-every-byte-ever-seen approach (fine for `crust game`'s
    own pty checks, which only ever needed a handful of full initial
    repaints) breaks down for a TUI that redraws the same region
    dozens of times a second with bubbletea's own line-diffing — later
    frames only retransmit *changed* lines, so a flat concatenation of
    raw bytes across many ticks interleaves fragments from different
    moments in time rather than reflecting any single coherent screen.
    `pyte`'s `Stream`/`Screen` replay the exact same escape sequences a
    real terminal would interpret, correctly reconstructing "what's
    actually on screen right now" regardless of which lines a given
    frame did or didn't retransmit — the same category of "verify
    against the real thing, not a text-log approximation of it"
    upgrade the project's own Playwright-over-`curl` precedent already
    established for the browser-side features.
  - Verified with two layers: ordinary Go tests
    (`studio_builtins_test.go`, `studio_test.go`) covering every
    builtin's argument validation and entity mutation, `loadStudioProgram`'s
    parse/runtime/success paths, `studioModel.Update`'s key-recording/
    quit/tick/error-freezing behavior, and both restart outcomes
    directly — no pty needed for any of this, since none of it depends
    on a real terminal; and a real pty session (`pyte`-backed, as
    above) driving the actual built binary through
    `examples/game/snake.crust`: header/border/snake/food all render,
    the snake's own `###` head genuinely advances between two real
    reads a couple of seconds apart (not just "no crash"), `r`
    restarts cleanly, `q` exits the process, an initial bad file exits
    with a parse error shown, and editing the file to something broken
    then pressing `r` mid-session shows the error inline while leaving
    the tool itself running.
- **Live breakpoint / step-through debugging** (`crust develop`'s new
  Live tab, `internal/debugger/live.go` + `cmd/crust/debug_live.go`),
  on direct request — "go ahead" to the last of the original six
  develop-tool ideas, once the web playground was done.
  - **The core mechanism is a blocking `Step`, not a UI flag.** Every
    other tracer in this codebase (`Recorder`) is a passive observer:
    `Step`/`PushFrame`/`PopFrame` record something and return
    immediately, the run never waits on them. `LiveTracer.Step` is
    different on purpose: when the current statement's line is a
    breakpoint (or single-step mode is on), it sends a `LivePause` on
    its own `Paused` channel and then *blocks* reading `Resume` until
    the caller says what to do next. That blocked state — on the
    interpreter's own goroutine, not the TUI's — genuinely *is* "the
    program is paused," not a separate boolean this file has to keep
    in sync with reality. `LiveTracer` starts in single-step mode
    (`NewLiveTracer`), so the very first traced statement always
    pauses: a live session should show you the first thing about to
    run, not silently execute an unknown amount of the program before
    you get a say — the same "don't run ahead of the user" reasoning
    `emptyDebugView`'s own doc comment already gives for why the whole
    TUI starts with nothing recorded rather than an eager first run.
  - **`RequestStop` vs `LiveStop`.** `LiveStop` sent on `Resume` only
    reaches a `Step` call already blocked waiting for it — it stops a
    *paused* run cleanly. It does nothing for a run currently racing
    toward a breakpoint (or one with no breakpoints ahead of it at
    all) after `c` (continue): nobody is blocked in `Step` to receive
    it. `RequestStop` covers that case with a lock-free `atomic.Bool`
    every `Step` call checks first, breakpoint or not — set it from
    outside at any time, and the run unwinds at the very next
    statement. The Live tab's `x` key uses whichever actually applies:
    `LiveStop` via `Resume` while paused, `RequestStop` otherwise.
  - **Unwinding via `panic(LiveStopSignal{})`.** There's no cooperative
    return-value path back out of a Go call stack that's potentially
    many `evalFramed` frames deep (nested recipe calls, loop bodies) —
    a panic is the only way to abort a goroutine blocked or executing
    partway through someone else's call stack from the outside.
    `LiveStopSignal` is exported specifically so the caller running the
    interpreter (`startLiveRun`'s own goroutine) can `recover()` and
    type-assert against it, converting a deliberate stop into an
    ordinary `liveDoneMsg{Stopped: true}` rather than a crash — the
    same "not the primary error path, just an escape hatch" reasoning
    `run.go`'s own top-level `recover()` already documents for a Go
    panic that isn't a cRust runtime `Error`. Any *other* panic is
    reported as `Failed` instead of silently swallowed, since that
    would be a real interpreter bug worth seeing, not a deliberate stop.
  - **`Env` timing matches the Stepper's own "after the fact"
    semantics.** `trace.StepEvent.Env` is captured after `Eval` returns
    for that statement (already true before this feature — see its own
    doc comment), so a pause at line N shows that line's *own* effect
    already applied, not just everything before it. This tripped up the
    first draft of `TestLiveTracerStepAdvancesOneStatementAtATime`,
    which assumed `x` wouldn't be bound yet at the pause for `x = 1` —
    fixed by asserting what the design actually promises instead of
    what felt intuitive at a glance.
  - **`startLiveRun`** (`debug_live.go`) mirrors `buildDebugView`'s own
    "parse the file, build a fresh `Interpreter`, run it" shape — the
    same one the Editor tab's save-triggered reload already reuses —
    just with `interp.Trace` set to a `LiveTracer` instead of a
    `Recorder`, and run on its own goroutine rather than synchronously,
    since a paused `Step` call has to be able to block without freezing
    the whole `develop` process. `liveOutputBuffer` (a mutex-guarded
    `bytes.Buffer`) is the program's stdout target, safe across the two
    goroutines that touch it: the interpreter's (writing, as `deliver`
    calls happen) and the TUI's (reading it fresh on every redraw, the
    same content the Run tab's own `runOutput` shows, just updated
    incrementally instead of all at once at the end).
  - **`liveGen` — the "stale message from an abandoned run" problem.**
    Once `c` is pressed, the interpreter goroutine keeps running toward
    the next breakpoint independently of whatever tab is currently
    showing, and its eventual `livePauseMsg`/`liveDoneMsg` will still
    be delivered to `Update` regardless (the same "messages arrive
    whichever tab you're on" behavior `runResultMsg`/`benchResultMsg`
    already rely on). That's fine as long as it's still the *same*
    run — but switching to a different file via the Files tab
    (`debug_nav.go`'s `switchToFile`) leaves that goroutine dangling
    with no way to reach it synchronously (`RequestStop` helps a
    *future* `Step` call, but the message already in flight was built
    before that). `liveGen` is bumped on every fresh run and on every
    file switch, stamped onto every message a run produces at Cmd-
    construction time, and checked against the model's current value
    before a `handleLiveStart`/`handleLivePause`/`handleLiveDone` ever
    applies one — a mismatch means "this is from a run I've since
    moved on from," silently ignored. The abandoned goroutine itself is
    never force-cleaned up (it holds no real resources — an in-memory
    buffer and reader, nothing that outlives the process), just left to
    finish unheard.
  - **Reusing the Run tab's settings, not a second input UI.** The Live
    tab has no store selector or input-file field of its own — `r`
    reads `m.selectedRunEntry()` and `m.runInput` fresh at run time,
    exactly the same "the exact *same* settings... not a separate copy"
    reasoning the Bench tab's own doc comment already establishes for
    why *it* has no duplicate fields either.
  - **Breakpoints persist per file** (`develState.LiveBreakpoints`, a
    `map[int]bool` — `encoding/json` marshals an int-keyed map as an
    object with stringified keys automatically, no custom
    (un)marshaling needed) the same `restore*`/`save*BestEffort`
    read-mutate-write shape `benchBaseline` already established.
    Toggling one (`toggleLiveBreakpoint`) also pushes the updated set
    straight to a currently-active `LiveTracer.SetBreakpoints` — which
    copies its input rather than aliasing it, so the caller mutating
    its own map afterward can never reach back in and change what a
    `Step` call already in flight sees — so editing breakpoints mid-run
    (even while paused somewhere else) takes effect on the very next
    statement, not just the next time `r` is pressed.
  - Verified with table-driven Go tests under `-race` (breakpoint
    pausing and single-stepping through a real parsed-and-evaluated
    program via `internal/interpreter`, not hand-built `StepEvent`
    structs; `Env` snapshot timing; `RequestStop` both mid-flight and
    while already paused; `SetBreakpoints`' copy-not-alias semantics;
    `liveGen` staleness guarding in `handleLiveStart`/`handleLivePause`/
    `handleLiveDone`; breakpoint persistence round-tripping) and a real
    pty-driven session: launched `crust develop` on a small program,
    Shift+Tab'd directly to the Live tab (now the last tab, so a single
    backward wrap reaches it), `r` to start (paused at the top-level
    `recipe store() {` declaration — the always-pause-first entry
    point), `s` twice to step through `x = 1` and `y = 2` watching both
    show up in the variables panel, moved the cursor down and set a
    breakpoint on `deliver(z)`, `c` continued straight to it (variables
    panel showed `z = 3`, output panel already showed `3` — `deliver`
    had already run by the time its own statement's pause reports, the
    same after-the-fact timing as everywhere else), and confirmed via
    `develop_state.json` that the breakpoint was still there — and
    still took effect — on an entirely separate `crust develop` launch
    afterward.
- **Richer stack traces** (`object.Error.Frames`, `internal/object/
  error.go` + `internal/interpreter`'s `applyFunction`), on direct
  request — the last of the three stretch goals in TODO.md, picked as
  the most impactful of the three: modules are low-payoff for AoC's
  mostly-single-file solutions, and a bytecode VM was explicitly
  deferred until a real profile says the tree-walker is the bottleneck
  (§5's Performance Strategy) — richer error output, by contrast, pays
  off on *every* runtime error hit while solving a puzzle, with no new
  syntax and no architectural rewrite.
  - **No separate call-stack bookkeeping — it falls out of Go's own
    call stack for free.** `object.Error` gained `Frames []Frame`
    (`Frame{Name string; Line, Col int}`), appended one entry per level
    by `applyFunction`'s `*object.Function` case, exactly at the point
    each call's result comes back as an Error: `errObj.Frames =
    append(errObj.Frames, object.Frame{Name: frameName(calleeExpr,
    label), Line: tok.Line, Col: tok.Col})`. Because this happens as
    each nested `applyFunction` call *returns* (unwinding back up
    through Go's own recursive call stack, one level per recipe call
    it's inside), the chain builds itself in exactly the right order —
    innermost failure first — with no explicit stack, push, or pop
    anywhere. Recursion needs no special-casing either: `fib`'s own
    resulting Frames simply has one entry per recursive level that was
    actually unwound through, since each recursive call is its own
    independent `applyFunction` invocation on Go's call stack, same as
    any other nested call.
  - **Zero cost on the success path was the real design constraint,
    not an afterthought.** `applyFunction` already had a `label`
    parameter for the debugger's own frame display
    (`evalCallExpression`), but that's eagerly computed only when
    `i.Trace != nil` — an ordinary `crust run` never sets it, so
    `label` is always `""` there. A stack trace has to work on *every*
    run, traced or not, so it couldn't reuse that same gate — but
    `ce.Function.String()` (turning `sum` into the frame name
    `"sum(...)"`) still isn't free, and paying it on every single call
    regardless of recursion depth would have directly worked against
    the very reason this feature was picked over the bytecode VM: not
    regressing the performance this session's own benchmarks track.
    Solved by threading the *unevaluated* `ce.Function` expression
    itself through as a new `calleeExpr ast.Expression` parameter
    (`nil` from `i.Call`/`i.CallNamed`, which have no source-level call
    expression to read from and fall back to their own already-eager
    `label` instead) and only calling `.String()` on it — `frameName`
    — inside the one branch that already knows an Error is
    propagating, the rare path. Verified, not just asserted:
    `BenchmarkUntraced` (`internal/interpreter`, a recursive `fib`) and
    `BenchmarkAoC2020Day1Part1`/`Part2` (`cmd/crust`) both landed
    within normal run-to-run noise of their documented baselines
    before and after this change.
  - **A single level of call wrapping prints exactly as before.**
    `Error.FrameLines()` — the rendering side, called by both
    `internal/runner.reportRuntimeError` and `crust repl`'s own
    separate error-printing path — returns `nil` for fewer than two
    frames: an error happening directly inside a `store()` entry
    point's own body (the single most common shape — no further
    nesting) produces exactly one frame (for the `store()` call
    itself), which adds nothing beyond what the primary
    `path:line:col: message` line already says, so no caller ever sees
    an extra line for it. Every existing substring-matching test (the
    primary line's own format is completely unchanged) kept passing
    with zero modifications.
  - **Deep but finite recursion is capped at display time, not capture
    time.** `Frames` itself stays a complete, uncapped slice — cheap
    even at a few thousand entries, and there's no reason to lose data
    the caller might still want. `FrameLines` caps what it actually
    *renders* to `maxErrorFrames` (12), with a "... and N more
    frame(s)" tail — the same bounded-output shape
    `debugger.DefaultMaxSteps`/the Stepper's `maxWatchLines`/every
    other capped panel in this codebase already uses, applied here to
    keep a legitimate 1,000-level recursive failure from dumping 1,000
    near-identical lines. Verified with a real 20-level recursive
    program: 21 `boom()` calls plus 1 `store()` call is 22 frames — 12
    shown, "... and 10 more frame(s)," matching the math exactly.
- **Puzzle input auto-fetch + solve timer** (`internal/aoc`,
  `cmd/crust/login.go`/`fetch.go`/`done.go`/`debug_aoc.go`), from a
  direct request resolved into three concrete choices via
  `AskUserQuestion`: fetch triggered by both an explicit subcommand and
  `crust develop`'s own auto-fetch; the session cookie stored in a
  crust-managed config file; the timer shown live during a run and
  saved at the end. Every part of this feature is opt-in and silently
  inert the moment no session has been saved — `crust run`, `crust
  develop` on a file with no day number, and everyone not using `crust
  login` at all see zero behavior change.
  - **`internal/aoc` — no dependency on `cmd/crust` or the terminal at
    all**, same "pure Go, testable in isolation" shape as
    `internal/debugger`. `session.go`: `LoadSession()`/`SaveSession()`,
    a `0600` file at `$XDG_CONFIG_HOME/crust/session` (`os.UserConfigDir`
    behind a swappable `sessionDir` var, the same test-seam pattern
    `cmd/crust/debug_state.go`'s `develStateDir` already established) —
    `$AOC_SESSION` is checked first, the same override-by-env-var
    convention `NO_COLOR` already uses elsewhere in this codebase.
    `fetch.go`: `Client.FetchInput(year, day)`, with an overridable
    `BaseURL` field existing purely so tests can point it at an
    `httptest.Server`. **No test anywhere in this feature — not
    `internal/aoc`'s own, not `cmd/crust`'s — ever makes a real request
    to adventofcode.com**, both because there's no legitimate session
    cookie available to test with here and, independently of that, out
    of respect for the site operator's own publicly stated request that
    automated tools not hammer the endpoint; `FetchInput` sets a
    descriptive `User-Agent` for the same reason. `timer.go`:
    `StartTimer`/`StopTimer`/`Elapsed`, one shared `timers.json` under
    the same config directory, keyed `"<year>/<day>"`, read-mutate-
    write-the-whole-file on every call — deliberately the same shape
    `debug_state.go`'s `saveDevelState` already uses for its own
    per-file settings blob, not a new pattern.
  - **Three new `cmd/crust` subcommands**, matching the existing
    any-order-flags-then-positional parsing shape `parseRunArgs`/
    `parseFmtArgs` already use. `crust login`: prompts on stdin with no
    input masking — `golang.org/x/term` isn't a dependency this project
    otherwise needs (confirmed via `go.mod` before deciding this), so
    adding one purely to hide a single paste wasn't judged worth it; the
    prompt states the lack of masking up front rather than leaving it a
    surprise. `crust fetch <day> [--year Y] [--force] [--out path]`:
    refuses to overwrite an existing `dayNN_input.txt` unless `--force`
    is passed (a hand-edited or already-fetched input file is never
    silently replaced), starts that day's timer on a successful fetch.
    `crust done <day> [--year Y]`: stops the timer and prints elapsed —
    **deliberately a separate, explicit command rather than
    auto-stopping on a clean run**, since nothing in this codebase
    checks a run's output against AoC's own accepted answer, so a
    program finishing without a runtime error doesn't mean the puzzle
    is actually solved; only the solver knows that.
  - **`crust develop`'s auto-fetch and live stopwatch**
    (`cmd/crust/debug_aoc.go`), wired into `runDebug` right after
    `ensureFileExists` and into the Nav tab's file-switch/new-file
    paths. `dayNumberFromPath` parses a day number from a filename
    following `examples/dayNN_template.crust`'s own convention
    (`day06.crust`, `Day6_input.txt`, ...); `maybeAutoFetchInput` fetches
    and starts the timer only when a day number parses, a session is
    saved, and `dayNN_input.txt` doesn't already exist — any fetch
    failure (day not unlocked, bad cookie, network error) is reported to
    stderr but never fatal to `develop` itself, the same "convenience on
    top of a workflow that works fine without it" posture as the rest of
    this feature. The header's live stopwatch (`aocStatusLine`, appended
    to `debugView.header()`'s existing one-line summary) needed **the
    first periodic-redraw mechanism anywhere in this TUI** — every other
    tab only ever redraws in response to a keypress or a background
    goroutine's own result message. `aocTickMsg` (`time.Time`) plus
    `aocTickCmd` (`tea.Tick(time.Second, ...)`) starts ticking from
    `Init()` only when `aocTimerRunning()` already holds for the file
    being opened, and the `Update` case only reschedules itself while
    that's still true — so a file with no timer, or nobody using the
    feature at all, never pays for a background tick loop, and a timer
    stopped elsewhere (`crust done` in another terminal) is noticed and
    the ticking stops itself within one second rather than running
    forever. Switching files via the Files tab (`switchToSelectedFile`)
    or creating a new one there (`n`) both re-check
    `aocTimerRunning()` for the *new* file and restart the tick if
    needed, since `Init()` only ever runs once at TUI startup and
    wouldn't otherwise notice a file swap mid-session. Verified with a
    real subprocess run (help text, `crust login`'s saved-cookie file
    permissions and content, the no-session error path naming `crust
    login`, `$AOC_SESSION` overriding a saved file) and a real pty
    session with a pre-seeded `timers.json` showing the header's
    stopwatch genuinely counting up once a second on screen (`0s` ->
    `6s` over ~6 real seconds) plus a separate run confirming a file
    opened with no session configured never even creates the config
    directory, proving the feature is fully inert by default rather
    than merely "looks inert in the cases tested."
  - **Manual fetch/submit/login/new actions** (`cmd/crust/debug_aoc_actions.go`),
    on direct request: auto-fetch only covers "open a file, its input
    shows up" — a solver still needs a way to retry a failed
    auto-fetch, submit an answer without leaving the TUI, or log in for
    the first time from inside an already-open session, none of which
    the passive auto-fetch path can do on its own. All three live on
    the Run tab, bound to `Ctrl+F`/`Ctrl+S`/`Ctrl+L` — never bare
    letters, since the Run tab's input-file field needs every printable
    key for itself; `Ctrl+<letter>` is safe for the same reason
    `Ctrl+R` ("run all stores") already is — a terminal in raw mode
    never generates one from ordinary typing, confirmed by reading the
    vendored `bubbletea` source rather than assuming. `aocFetchCmd`
    resolves day/year the same way `maybeAutoFetchInput` does, adopts
    an already-downloaded file without re-fetching (reported as a
    successful no-op, not silence — pressing the key is an explicit
    request, unlike auto-fetch's silent skip), and on either path calls
    the new `saveInputPathBestEffort` (`debug_state.go`) to overwrite
    the Run tab's remembered input path — the same "explicit action,
    unconditional persistence" shape `--year`'s `saveYearBestEffort`
    already established, and `handleAocFetchResult` applies the result
    straight to `m.runInput` too, so the *very next* run already reads
    the real puzzle input without retyping the path — this is also what
    makes auto-fetch itself pre-populate the field, since
    `maybeAutoFetchInput` calls the same save function on a successful
    startup fetch. `aocSubmitCmd` submits the Run tab's own last output
    line (`lastNonEmptyLine`) as the answer — the most direct stand-in
    for "what a solver would read off the screen and paste into AoC's
    website by hand" — refusing before ever making a request if nothing
    has been run yet or the last run failed (its final line is far more
    likely to be error text than an answer). The part comes from
    `levelForEntry`, mapping the Run tab's selected entry point
    (`"part2"` -> level 2, everything else -> level 1) to match every
    file this session's own tooling generates. `loginCmd` reuses the
    Editor tab's exact established pattern for handing the whole
    terminal to another interactive program: self-exec's `os.Args[0]
    login` (not a hardcoded `"crust"`, so this works identically for a
    `go run` build, a locally built binary under any name, or an
    installed one) via `tea.ExecProcess`, with `cmd.Stdin` preset to the
    real `os.Stdin` ahead of time for the same reason `nvimCmd`'s own
    doc comment already gives. `handleLoginExit` deliberately re-queries
    `aoc.LoadSession()` itself afterward rather than trusting
    `cmd.Run()`'s error, since a non-zero exit is also `runLogin`'s own
    legitimate result for an empty typed cookie — checking the real,
    current state directly is simpler than reverse-engineering that
    distinction from an exit code. A fourth action, `Ctrl+N` on the
    Files tab (`debug_nav.go`'s `createNavAoCFile`), scaffolds a new AoC
    day without leaving `develop` to run `crust new` separately: a
    second prompt-mode bool (`navCreatingAoC`) alongside the existing
    blank-file `navCreating`, sharing one text field and one key
    handler rather than a merged enum, specifically so every existing
    `navCreating`-only test and call site keeps working unchanged — the
    two are only ever set one at a time. It parses a day number and
    optional year out of the typed text, stamps the file from
    `new.go`'s own `dayFileTemplateFmt` (no drift between the two
    creation paths), and persists a given year immediately via the
    existing `saveYearBestEffort`. Verified with a real pty session
    exercising all four keys in a real terminal: `Ctrl+F`/`Ctrl+S`'s
    own "no session saved" error paths (deliberately not hitting the
    real adventofcode.com from a verification script, this codebase's
    established posture for AoC-network features), `Ctrl+L` genuinely
    suspending into `crust login`'s real prompt text, and `Ctrl+N`
    creating a real `dayNN.crust` from the AoC template, switching into
    it, with the typed year stamped into the file and persisted to
    `develop_state.json`.
- `lexer_test.go` / `parser_test.go`: table-driven unit tests (input
  string in, expected tokens/AST shape out).
- `interpreter_test.go`: evaluate a snippet, assert the resulting
  `object.Object` (type + value).
- **Integration tests** (`cmd/crust/examples_test.go`,
  `TestRealExamplesProduceExpectedOutput`): real shipped `.crust`
  programs — not synthetic snippets — run through the actual CLI
  pipeline (`runFile`) with output pinned to values verified
  independently of the test suite. A plain table-driven test (`path`,
  `store`, `inputPath`, `want`), not the paired `testdata/*.crust` +
  `*.golden`-file scheme an earlier draft of this section described:
  every case's expected output here is one short line, and a whole
  directory-scanning golden-file harness would be more machinery than
  a dozen one-liners justify — see this codebase's own
  don't-add-abstraction-beyond-what's-needed convention, applied to
  its own test infrastructure. `TestRunFile` (`main_test.go`) already
  covers `runFile`'s own dispatch/error-handling logic with small
  synthetic snippets built inline; this file is deliberately the
  opposite. Seeded with all ten `examples/aoc2020/day01.crust`..
  `day05.crust` part1/part2 runs (Phase 8's dry-run below) plus the two
  pre-existing AoC-shaped examples — an initial real slice, not
  exhaustive coverage of every file under `examples/`.
- **Benchmark** (`cmd/crust/benchmark_test.go`,
  `BenchmarkAoC2020Day1Part1`/`Part2`): runs through the same real
  `runFile` CLI pipeline the integration tests use, not an isolated
  interpreter microbenchmark, against a deterministic (fixed-seed)
  synthetic input sized like a real puzzle rather than the puzzle's own
  tiny documented example (6 entries) — 200 distinct entries, the size
  real AoC 2020 Day 1 personal inputs are. Day 1 specifically: its two
  solutions are the closest thing among `examples/aoc2020/` to a
  worst-case loop/index workload (part 1's O(n²) pair search, part 2's
  O(n³) triple search) — the one most likely to notice a regression in
  identifier lookup, index-expression evaluation, or loop overhead. The
  answer pair/triple is appended last in the generated input on
  purpose, so the early-exits-on-first-match nested search does close
  to its full work rather than getting lucky early and understating
  what a differently-shaped real input would cost. Baseline on the
  hardware this was written on: ~2.1ms/op (part 1), ~9.0ms/op (part 2)
  at `-benchtime=1s`.

### Phase 8 — AoC 2026 Readiness
Process, not architecture: once Phases 2–5 are solid, do a dry run
solving a handful of old AoC days end-to-end in cRust. Anything that's
awkward to express (missing builtin, clunky syntax) becomes a punch-list
item to fix before December — this is the point of building the language
months ahead of the event instead of the week before.

**The dry run**: AoC 2020 days 1–5, each written as an independent,
real solution (`examples/aoc2020/day01.crust`..`day05.crust`, own
`store_part1`/`store_part2`, run against the puzzle's own documented
small example input, verified against the documented example answers
via a real built `crust run` subprocess for every part of every day —
not just `go test`). This is a genuinely different kind of exercise
than the earlier "shape only" examples (`day01_find_pair.crust`,
`day06_group_answers.crust`, both pre-dating this phase): those exist
to demonstrate one feature in isolation with data already massaged
into the right shape, while a real day's solution has to get raw text
*into* that shape first — multi-record parsing, string-based
validation, coordinate arithmetic — the parts of a language that don't
show up until something real is built with it.

Two real language bugs turned up, both parser/interpreter correctness
issues rather than missing builtins, and both fixed with regression
tests rather than worked around in the example code:

- **Slicing's exclusive end bound rejected the single most common
  legitimate value: the container's own length.** `s[3.<5]` on a
  5-character string ("get the last two characters") errored "slice
  index out of range: 5" instead of returning them. `sliceIndices`
  (`internal/interpreter/expressions.go`) required an exclusive end
  bound to be a real, dereferenced index (`0 <= end < n`) — the correct
  rule for an *inclusive* bound, which really is read directly, but
  wrong for exclusive, which only ever reads up to `end-1` walking
  forward and never touches `end` itself. Fixed by giving exclusive
  bounds a ceiling of `n` (one past the last index) instead of `n-1`,
  walking forward specifically — Python's `s[3:5]` on a 5-character
  string is the same "read up through the very end" case, not an
  off-by-one.
- **A range's End didn't consume its own trailing `+`/`-` chain
  without an explicit paren.** `s[0 .< slices(s) - 2]` silently
  misparsed as `(s[0 .< slices(s)]) - 2` rather than
  `s[0 .< (slices(s) - 2)]` — a String/List minus an Integer, which
  isn't defined, so it surfaced as a confusing "unsupported operand
  types" error nowhere near the actual mistake. `parseRangeExpression`
  (`internal/parser/expressions.go`) parsed End at `SUM` precedence,
  which is the *correct* choice for an ordinary infix operator's own
  right operand (`parseInfixExpression` does the same, deliberately, so
  a trailing same-precedence operator gets left for an outer loop at
  that same precedence context to build proper left-associativity) —
  but a range's End has no such outer "range-level" loop to hand a
  continuation to; whatever invoked `.<`/`..` in the first place is
  usually running at a *lower* precedence (e.g. `LOWEST`, parsing an
  index bracket's full contents), so the trailing operator reattached
  to the whole range one level up instead of just its End. Fixed by
  parsing End at `SUM-1`, letting it swallow a complete term —
  matching the grammar's own `range = term [(".."|" .<") term]`
  production, where the second `term` was always meant to resolve its
  own internal `+`/`-` chain exactly like the first one already does.

A third real bug turned up later, during the "day-1 essentials"
confirmation pass — a formatter idempotency bug this time, found by the
same mechanism as the two above: a real, independently-written program
(`examples/day1_essentials.crust`) tripping over something a synthetic
test never happened to exercise.

- **`crust fmt` wasn't idempotent for a call whose argument is a
  compact, single-line `recipe(...) { ... }` literal** — e.g.
  `deliver(map(xs, recipe(x) { serve x * x }))` immediately followed by
  another statement with no blank line between them, a perfectly
  ordinary way to pass a short callback to `map`/`filter`/`reduce`.
  Formatting once correctly inserted no blank line; formatting *that
  output* again spuriously inserted one. `internal/format`'s
  `lastLine`/`blankLineIfGap` (format.go) decide whether to preserve a
  blank line between two statements by comparing an estimate of where
  the first one's source "ends" (`lastLine`) against the second one's
  own start line — but `lastLine`'s only block-aware cases were
  statements that are *themselves* a block header
  (`IfStatement`/`CountedLoop`/`ForEachLoop`/`BakeStatement`); a plain
  `ExpressionStatement` that merely *contains* a block via a nested
  `FunctionLiteral` argument fell through to its default case
  (`s.Pos().Line`, a single source line, no matter how far the
  statement's own tokens actually extended). The canonical printer
  always expands a recipe body onto multiple lines regardless of how
  compact the source was, so on a second formatting pass — now
  reformatting output where that expansion had already happened — the
  stale single-line estimate made the next statement look further away
  than it actually was, i.e. "there must have been a blank line here."
  Fixed with `lastLineOfExpr`, a recursive walker mirroring `lastLine`'s
  own shape one level down into `ast.Expression` (which, unlike
  `ast.Statement`, has no `Pos()` of its own — see ast.go's own doc
  comment on why — so each case reads the concrete type's `.Token`
  directly instead): a `FunctionLiteral` resolves through
  `lastLineOfBlock` exactly like a statement-level block already does,
  and every other node (`CallExpression`'s arguments, an infix/index
  operand, a range's `Start`/`End`, a ternary/Elvis's branches, a
  List/Tuple/Set's elements, a Map's keys and values) recurses and
  takes the widest span across whatever it holds, so a `FunctionLiteral`
  buried at any depth — not just as a direct call argument — is found.
  Regression-tested directly
  (`TestFormatIdempotentWithInlineFunctionLiteralArgument`,
  `TestFormatIdempotentWithFunctionLiteralNestedDeeper` covering a
  recipe literal buried in a List element and an infix operand, not
  just a call argument) in addition to being caught by the pre-existing
  glob-based `TestFormatIsIdempotent` the moment the new example file
  existed.

Both bugs are the specific shape a hand-written unit test suite tends
to route around without ever tripping: every pre-existing slice test
happened to parenthesize a compound end bound (see
`TestSliceExpressionBoundsAreEvaluated`'s `xs[a..(a+2)]`), and no
existing test needed a suffix slice reaching exactly to a container's
length. A real, independently-authored program hits both immediately,
which is the entire reason this checklist item exists rather than
trusting coverage percentages alone. Both fixes shipped with dedicated
regression tests at the level each bug actually lived at —
`TestRangeEndSwallowsATrailingSameLevelOperator` (parser, asserting the
AST shape directly) and `TestSliceExclusiveEndAtContainerLengthIsValid`
/ `TestRangeEndConsumesTrailingSameLevelOperatorWithoutParens`
(interpreter, asserting the runtime result) — not just a fixed example
file, which would only prove today's five puzzles work, not that the
underlying bug is actually gone.

- **`crust new <day>` (`cmd/crust/new.go`)**, from the same "go ahead
  on those" AoC-workflow batch as the heap builtins above. Stamps out
  `dayNN.crust` using the exact `store_part1`/`store_part2` shape
  `examples/dayNN_template.crust` already documents for manual `cp`ing
  — one starter layout taught in one place, not a second one invented
  just because this path fills in its own placeholder — with the real
  day number substituted in via `fmt.Sprintf` everywhere the template
  file's own copy still says "NN" for a human to replace by hand.
  Deliberately narrow scope: only `dayNN.crust`, no custom `--out` path
  the way `crust fetch` offers one, since the whole point is removing
  the one remaining manual step (`cp ... dayNN.crust`) from a workflow
  that's otherwise `crust new`/`crust fetch`/`crust develop` in
  sequence — a custom name would just be `cp` again with extra steps.
  Refuses to overwrite an existing file unless `--force` is passed,
  the same protection `runFetch` already gives an existing input file,
  for the same reason (a file someone's already started writing in is
  exactly what the guard exists to not silently discard).
  - **No change needed in `crust develop`'s own Nav tab.** The Nav
    tab already lists every `.crust` file "alongside the one currently
    open" by directory scan, so a file `crust new` creates from the
    command line shows up there automatically the next time the tab
    renders — confirmed by inspection of `debug_nav.go`'s listing
    logic, not assumed. The Nav tab's own `n` key (task history:
    "Persist crust develop's per-file store/input settings") still
    creates a genuinely blank file, deliberately left alone here — that
    key makes a new file of *any* name (a shared helper, not
    necessarily a `dayNN.crust`), and seeding it with
    `store_part1`/`store_part2` stubs wouldn't make sense for a file
    that was never meant to hold either.
  - Verified with table-driven Go tests (`parseNewArgs`'s flag/day
    parsing, force-vs-refuse-to-overwrite, dispatch wiring through
    `run()`) plus one that goes a level further than the other CLI
    subcommands' own tests do: `TestRunNewCreatedFileParsesAndRuns`
    feeds the freshly-stamped file straight through `runFile`, proving
    the template is actual parseable, runnable cRust — not just text
    that happens to contain the right substrings — the same standard
    `examples_test.go` already holds every checked-in example file to.
    Also checked by hand against a real built binary: created,
    confirmed the header/day number/stub content, ran it against piped
    stdin, and confirmed the overwrite guard actually refuses a second
    `crust new` for the same day.

- **`crust submit <day> [answer] --part=<1|2>` (`internal/aoc/submit.go`,
  `cmd/crust/submit.go`)**, the last item in the same "go ahead on
  those" batch: `crust fetch`/`crust done` already handle getting a
  day's input and timing how long it takes, but closing the loop back
  to adventofcode.com itself — actually submitting an answer — still
  meant leaving the terminal for a browser tab. `Client.Submit` posts
  the same `level`/`answer` form fields the site's own page does to
  `/<year>/day/<day>/answer`, reusing `Client`'s existing
  `Session`/`BaseURL`/`userAgent` plumbing from `FetchInput` rather
  than a second HTTP setup.
  - **Response parsing is plain substring matching on a stripped
    `<article><p>...</p>` extract, not an HTML parser.** AoC has no
    structured submission API — only this one HTML page — but its own
    response wording ("That's the right answer", "too low"/"too high",
    "you gave an answer too recently", "don't seem to be solving the
    right level") is stable, narrow, and long-documented enough by the
    AoC community that reaching for a real parser (a new dependency,
    against the zero-Go-dependency policy) to recognize five known
    sentences would be over-engineering a problem this doesn't have —
    the same reasoning `findInts` gave for hand-rolling its own scanner
    instead of pulling in `regexp`. `stripTags` is a ~15-line
    depth-counting scan, not a parser: good enough to turn `That's the
    right answer! <a href="...">[Return to Day N]</a>` into readable
    plain text, nothing more ambitious attempted.
  - **`SubmitOutcome` is a closed enum a caller switches on
    (`OutcomeCorrect`/`OutcomeIncorrect`/`OutcomeTooLow`/
    `OutcomeTooHigh`/`OutcomeRateLimited`/`OutcomeAlreadySolved`/
    `OutcomeUnknown`), not just an error-or-not bool** — a caller
    scripting around this (the whole point of a CLI over a browser)
    needs to tell "wrong, try again" apart from "rate-limited, wait"
    apart from "already solved, nothing to do" programmatically, not
    just read a sentence. `OutcomeTooLow`/`OutcomeTooHigh` get their
    own values rather than folding into a generic `OutcomeIncorrect`
    specifically because they're the signal a binary-search-refinement
    workflow needs. `OutcomeUnknown` (an unrecognized response, AoC
    wording changing) is deliberately not an error — the HTTP request
    itself succeeded — so `Message` still carries whatever text came
    back for the caller to show verbatim.
  - **`runSubmit` reads the answer from stdin when the CLI omits it**,
    the one piece with no `crust fetch`/`crust done` precedent to
    follow: `crust run day06.crust --store=part1 | crust submit 6
    --part=1` pipes a run's own `deliver()`d answer straight through
    without a manual copy-paste step, mirroring how cRust programs
    themselves read input via `unbox()`. Exit code 0 only on
    `OutcomeCorrect`, every other outcome (including the request
    succeeding but the answer being wrong) exiting 1 — success-is-0 is
    what makes `crust submit ... && crust done N`-style chaining work
    the way every other crust subcommand's exit code already does.
  - **Deliberately doesn't touch the timer.** `crust fetch`/`crust
    done`'s own history already reasoned through this: "nothing here
    checks a run's output against AoC's own accepted answer, so a
    program finishing without error doesn't mean the day is actually
    solved." `crust submit` could now tell — but a correct part 1
    isn't "done" for a two-part day, and auto-stopping on part 2's
    correct answer would be one more piece of implicit behavior for a
    command whose only job is reporting what AoC said, matching
    `runDone`'s own explicit "the solver is the only one who knows
    that" stance rather than reopening it.
  - Verified with table-driven Go tests at both layers:
    `internal/aoc/submit_test.go` (all six outcomes classified
    correctly against `httptest`-served AoC-shaped HTML, `stripTags`/
    `articleText` tested directly, bad-session and server-error paths)
    and `cmd/crust/submit_test.go` (flag/positional parsing including
    "answer omitted", stdin-fallback, exit codes per outcome, and
    dispatch wiring through `run()`) — the same "never touch the real
    adventofcode.com from an automated test" posture `fetch_test.go`
    already established, for the same two reasons (no legitimate
    session available here, and not putting load — or in submit's
    case, a real wrong-answer rate-limit penalty — on the live site).

- **Files-tab "(used by N other files)" delivery-usage hints**
  (`cmd/crust/debug_nav.go`'s `deliveryTargets`/`computeDeliveryUsage`),
  the last item in the same AoC-workflow batch — a directory of
  `dayNN.crust` files sharing a handful of helper files (`grid_utils.
  crust`, a parsing routine) makes "is this helper still used, and by
  how many days" a real question with no easy answer short of grepping
  every file by hand.
  - **Reuses the real lexer/parser, not a text scan**, unlike a few
    other narrow-task hand-rolled scanners this session leaned on
    (`findInts`, `stripTags`) — `delivery`'s own syntax
    (`delivery "path.crust"`) is simple enough that a scan would mostly
    work, but the real parser is already a dependency this package has
    (every other tab already lexes/parses target files — switching to
    one, formatting, the Editor tab's reload), so reusing it here costs
    nothing extra and is correct by construction rather than by
    coincidence. `deliveryTargets` walks `program.Statements` (the
    file's own top level) rather than every nested block — `delivery`
    is syntactically legal anywhere a statement is, but every real use
    in this codebase and SPEC.md §10's own examples puts it at top
    level, so a full recursive walk would cost more than the hint
    actually needs for real cRust code.
  - **Paths resolve the exact same way `evalDeliveryStatement` resolves
    them at runtime** (`internal/interpreter/delivery.go`: relative to
    the delivering file's own directory) — since `listCrustFiles`
    already scopes everything to one directory, a resolved target
    either matches one of those files by base name or refers to
    something outside the listing entirely, which is exactly the
    distinction the usage count needs. A file delivering itself (a
    corner case the interpreter's own already-delivered guard already
    tolerates at runtime) is explicitly excluded from its own count —
    "used by N *other* files" should mean exactly that.
  - **Computed alongside `navFiles` in `refreshNavFiles`, not cached
    across the session** — same reasoning `navFiles` itself already
    documents: directory contents (including which files deliver
    which) can change between visits to the tab, and re-parsing a
    handful of small files on every tab switch is cheap enough that
    there's nothing to gain from a staler cache.
  - A parse failure in any one file (`deliveryTargets` returning nil)
    doesn't stop every other file's usage hint from showing — the same
    "one bad file shouldn't break the whole tab" posture
    `listCrustFiles`'s own directory-read failure handling already
    takes.
  - Verified with table-driven Go tests (`deliveryTargets` against a
    real multi-import file, no-imports, a parse error, and a missing
    file; `computeDeliveryUsage` across several files including the
    self-delivery exclusion; `refreshNavFiles` populating the new
    field; `viewNav`'s rendered hint text, singular/plural wording, and
    the no-hint case) and a real pty session driving `crust develop`
    against a small `grid_utils.crust` + two importing day files,
    confirming "(used by 2 other files)" renders exactly where expected
    and files with no importers show no hint at all.

- **Bench tab two-entry-point diff ('c')** (`cmd/crust/debug_bench.go`'s
  `benchCompareSnapshot`/`benchCompareSnapshotFrom`/
  `viewBenchCompare`), the last item in the batch — on direct request:
  "diffing two entry points, not just one against a saved baseline —
  useful once a day has both a brute-force and optimized
  `store_part2`."
  - **Reuses the baseline feature's own shape rather than inventing a
    second one** — `benchCompareSnapshot` mirrors `benchBaseline`
    (average duration/memory, computed via the exact same
    `benchBaselineFrom`), and `viewBenchCompare` mirrors
    `viewBenchBaseline`'s signed-percentage-diff format
    (`benchDiffPct`, unmodified). The one real difference: a two-
    entry-point compare has no single fixed "the" baseline the way a
    per-file regression baseline does, so `viewBenchCompare` labels
    *both* sides (`"compare: part2 (10 runs) vs captured part1 (10
    runs) — ..."`), where `viewBenchBaseline`'s own `"vs baseline:
    ..."` can safely leave the comparison target unlabeled — there's
    only ever one baseline, always for this same file.
  - **Deliberately session-only, never persisted to
    `debug_state.json`** — the opposite call from 'b's own baseline,
    and for a reason specific to what each feature is actually for:
    'b' tracks drift against one fixed comparison point across
    sessions/days, so persisting it is the whole point; 'c' is for
    A/B-ing two entry points (or two rewrites of the same one) that
    are both still open and being actively iterated on *right now* —
    persisting a capture across a restart would just mean comparing
    against a stale entry point nobody's touching anymore by the next
    session. `benchCompare` starts nil at every `newDebugModel` and is
    never written to disk, confirmed directly by
    `TestBenchCompareIsNilAtStartup` rather than left as an assumption.
  - **The workflow crosses tabs by design, not by accident**: capture
    on the Bench tab ('c'), switch the entry point on the *Run* tab
    (the same selector `benchCmd` itself already reads from —
    `m.selectedRunEntry()`), then run a fresh batch back on Bench. No
    new entry-point picker was added directly on the Bench tab itself
    — the Run tab's selector is already the single source of truth
    every batch reads from (the file's own top doc comment: "whichever
    entry point... the Run tab currently has selected"), so a second
    picker here would just be two controls that could disagree about
    which entry point a batch actually used.
  - `EntryLabel` is captured pre-formatted (`"(default)"` for the bare
    `store`, matching `renderEntryOptions`' own convention) rather than
    the raw `--store` value, so `viewBenchCompare` never has to
    re-derive the same display rule a second place.
  - Verified with table-driven Go tests (`benchCompareSnapshotFrom`'s
    averages and default-label case, 'c' requiring at least one run the
    same way 'b' does, `viewBenchCompare`'s both-sides-labeled diff
    text and its hidden-until-captured case, `handleBenchResult`
    clearing the status message but keeping the capture, and the
    nil-at-startup case) and a real pty session: a file with a fast
    `store_part1` and a deliberately slow `store_part2` (a tight
    counting loop, 5,000 vs 500,000 iterations), captured `part1` on
    Bench, switched to `part2` on Run, ran a fresh batch back on Bench,
    and confirmed the rendered line — `"compare: part2 (10 runs) vs
    captured part1 (10 runs) — runtime avg +9152.7% (vs 858.086µs),
    memory avg +8599.4% (vs 90.1KiB)"` — matched the two batches'
    actual, very different numbers.

- **`--year` on `crust develop`/`crust new`**, from a direct follow-up
  question after the stretch-goal batch above shipped: `crust fetch`/
  `crust done`/`crust submit` already accepted `--year` per call, but
  `crust develop`'s own auto-fetch, timer, and header stopwatch
  (`debug_aoc.go`) were still hardcoded to `defaultAoCYear` everywhere
  — a solver working a past AoC year through `develop` would have had
  its auto-fetch silently reach for the wrong event's input, and its
  timer silently key off the wrong year's record, with no flag able to
  fix either one. README's own `--year` documentation had scoped the
  flag to just fetch/done, confirming this wasn't an oversight in the
  docs — the feature genuinely didn't exist yet for `develop`.
  - **`debugOptions.Year` resolves once, early, and every downstream
    reader trusts it** — `applySavedOptions` (renamed from
    `applySavedStore`, now resolving Store *and* Year in one pass) runs
    before `maybeAutoFetchInput` in `runDebug`, specifically so the
    auto-fetch itself — not just the header, after the fact — reaches
    the right year. It always leaves `opts.Year` non-zero (falling back
    to `defaultAoCYear` itself if neither an explicit flag nor a saved
    value exists), so `maybeAutoFetchInput`/`aocTimerRunning`/
    `aocStatusLine` never need their own zero-value handling — except
    `aocYear()` still adds one anyway, purely as a second line of
    defense for any `debugModel` built without going through
    `runDebug` at all (most existing tests construct one directly via
    `newDebugModel`), so the invariant holds even for code paths that
    never got the memo.
  - **An explicit `--year` persists immediately, unlike `--store`.**
    Every other per-file setting (`Store`/`Input`/`RunAll`) is saved
    only once something's actually *run* from the interactive Run tab
    — there's no equivalent "run" action for a year, since nothing in
    the TUI ever sets one interactively. The CLI flag is the only way
    Year is ever set at all, so it has to stick the moment it's given,
    or a solver working a past year would be back to retyping `--year`
    on every single invocation — exactly the friction being removed.
    `saveYearBestEffort` mirrors `saveBenchBaselineBestEffort`'s own
    read-mutate-write-preserve-everything-else shape. The common case
    (no `--year`, this year's puzzle) never writes a `Year` entry at
    all (`omitempty`, and `applySavedOptions`'s own defaulting-to-2026
    step happens *after* the explicit-or-not check that gates saving)
    — `develop_state.json` doesn't get a `"year": 2026` line for every
    ordinary file that never needed one.
  - **`crust new` got the same flag for the same reason, one layer
    earlier** — `crust new 7 --year 2020` stamps 2020 into the file's
    own header comment *and* calls the same `saveYearBestEffort`
    immediately, so the very first `crust develop day07.crust`
    afterward already resolves to 2020 with no `--year` of its own.
    Closes the loop end-to-end: past-year setup is genuinely one flag,
    once, not one flag repeated on every subsequent command.
  - **The header now shows the year, not just the day**
    (`"⏱ day 7, 2020: 11s (running)"`, was `"⏱ day 7: ..."`) — day
    numbers repeat across every AoC event, so once a file's year can
    actually differ from 2026, leaving it out of the one place solvers
    glance at to check the clock would be genuinely ambiguous about
    which year's timer that even is.
  - Verified with table-driven Go tests across every layer this
    touches (`parseDebugArgs`'s `--year` parsing in both forms,
    `applySavedOptions`'s explicit-wins/fills-in/defaults-to-2026
    cases for Year specifically, `saveYearBestEffort`'s persistence
    and preservation of other settings, `maybeAutoFetchInput` actually
    fetching under a passed year — not 2026 — confirmed against both
    the request path (`/2020/day/3/input`) and that 2020's and 2026's
    timer records stay genuinely independent, `aocYear`'s fallback,
    `aocStatusLine`/`aocTimerRunning` reading `opts.Year`, and
    `runDebug`'s own persist-on-explicit/reuse-when-saved round trip)
    plus real end-to-end verification against the actual built binary:
    `crust new 7 --year 2020` (confirmed the stamped header and the
    saved `develop_state.json` entry), then a real pty session driving
    `crust develop day07.crust` with a pre-seeded 2020 timer record and
    *no* `--year` flag on that invocation at all, confirming the header
    rendered `"⏱ day 7, 2020: 11s (running)"` purely from the
    remembered state — and a follow-up `--year 2026` invocation
    confirmed the explicit-override-and-re-persist path too.

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
  reopening that here. [`docs/BYTECODE_VM_SCOPING.md`](./BYTECODE_VM_SCOPING.md)
  is a feasibility study (on direct request, "scope out exactly what
  that would entail... without breaking things") of what building one
  would actually cost, written before any decision to build it —
  including the same slot-resolution tension the bullet above
  describes, plus two more cRust-specific hard parts (reproducing
  `crust develop`'s per-statement tracing against compiled bytecode,
  and `crust repl`'s persistent-session state) and a concrete,
  strictly-additive path for building one without touching anything
  that exists today. Still gated on the same "profile first" call —
  the study doesn't change that, it just prices out what "go" would
  cost once real AoC 2026 input exists to profile against.

### When to revisit this list

`TODO.md`'s Phase 7 already has "Benchmark against a real prior-year
AoC puzzle for performance sanity" as a checklist item. That's the
right moment to profile (`go test -bench` + `pprof`) rather than
extending the deferred list above by guesswork — if `Environment`
lookups or GC pressure actually show up as the bottleneck there, that's
the point to reconsider slot resolution or arenas, backed by a number
instead of a hunch.
