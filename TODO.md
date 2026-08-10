# cRust Roadmap

Milestones for building cRust — a Go-based interpreter with a pizza-jargon
keyword set — in time for Advent of Code 2026 (Dec 1).

## Versioning

cRust is solidly in alpha — it's already being used to solve real Advent
of Code puzzles — so `0.1.0-dev` stopped being an honest description of
where the project stood. `v0.1.0` through `v0.1.68` are tagged
retroactively across the existing history, one per meaningful feature/fix
commit (purely cosmetic changes, merge noise, and doc typos were skipped);
`flake.nix`'s `version`/`ldflags` track the latest one. Going forward, bump
the patch number (`flake.nix`'s `version` and `-X main.version=`) and tag
the commit `v0.1.<n>` for each commit that ships a real feature or fix —
not every commit needs one, same judgment call the retroactive pass used.
No `1.0.0` cut yet; that's Phase 8's job once AoC-readiness is actually
confirmed, not before.

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

## Phase 5 — Standard Library (AoC-focused) ✅
- [x] Input: `unbox()`/`unbox(path)` (stdin or a file, whole contents
      as a String) + `lines(s)` (split into a List of lines)
- [x] `trim` (leading/trailing whitespace — kept distinct from `strip`,
      which is Set difference, to avoid a name collision)
- [x] `split` (`split(s)` whitespace-collapsing, `split(s, delim)`
      literal delimiter — CSV-style, preserves empty entries)
- [x] `join` (List of Strings + separator → String, `split`'s counterpart)
- [x] Strings: `contains`, `replace` (see the later, more detailed
      String helpers entry further down this phase's checklist)
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
- [x] Math: `abs`, `pow`, `gcd`, `lcm`, `sqrt` (standard names, not
      themed). `abs`/`pow` are type-preserving the same way `+`/`-`/`*`
      are (Integer stays Integer unless it genuinely can't — a negative
      `pow` exponent or `sqrt` always widen to Float); `gcd`/`lcm` are
      Integer-only, like `idiv`
- [x] String helpers: `replace(s, old, new)`, `upper(s)`, `lower(s)`,
      plus extending the existing `contains`/`find` to accept a String
      (substring search / substring index, rune-indexed like every
      other cRust string position)
- [x] `idiv` (integer division — SPEC.md §6, `/` always produces a Float)
- [x] `push` (append to a List in place) and `+` extended to List/List
      and Tuple/Tuple concatenation (always a new value, never
      mutating either operand — `push` is the in-place counterpart)
- [x] `map(iterable, fn)` — applies fn (a recipe or builtin) to every
      element of a List/Tuple, collecting the results. Composition of
      multiple steps is via a lambda (`map(xs, recipe(x) { serve g(f(x)) })`),
      not a List of functions
- [x] Collections: sort (`pizzasort`), `filter(iterable, fn)`, and
      `reduce(iterable, fn, init)` — `filter`/`reduce` follow `map`'s
      exact shape: fn (recipe or builtin) invoked through the same
      injected `Call` callback, short-circuiting on the first error fn
      produces. `init` is a required third argument to `reduce` (unlike
      Python's optional-with-a-runtime-error-on-empty
      `functools.reduce`), so `reduce([], fn, 0)` is just `0` instead of
      a special case to worry about
- [x] `slices` (length of a String/List/Map/Set)
- [x] `wrap(collection, i)` / `wrapSlice(collection, start, end)` —
      circular/modular indexing and slicing, on direct request: "can I
      create a circular list in crust? where if I index past the end
      it just auto mods and loops back to the beginning." Rather than
      a new type, both are plain builtins over List/Tuple/String:
      `wrap` reduces `i` modulo length before indexing (also the
      escape hatch for negative-index-from-the-end, which plain `[i]`
      deliberately doesn't support — SPEC.md §6); `wrapSlice` is the
      circular counterpart to `collection[start..end]`, wrapping each
      resolved position independently so a span can run past the end
      and keep going from the start, or lap more than once
      (`wrapSlice([0,1,2,3], 3, 6)` is `[3, 0, 1, 2]`, exactly the
      example asked for). Verified via the real `crust` CLI as well as
      Go unit tests
- [x] Sets: `gather`, `sprinkle`, `scrape`, `topped`, `combine`, `shared`, `strip`
- [x] Nil-handling: `sauce` (fallback-if-nobox)
- [x] Output: `deliver` *(no formatting verbs yet — space-joined
      `Inspect()` output plus a newline)*

  → Names for the items above are already locked in — see
  [docs/SPEC.md §7](./docs/SPEC.md#7-standard-library-builtins). Most
  of what's checked off landed ahead of schedule alongside Phase 4 —
  see ARCHITECTURE.md's Phase 5 section for exactly what's built.
  Phase 5's stdlib checklist is now fully complete.

- [x] Follow-up, on direct request: "Do we have a way to pop items out
      of collections?" — answered no (only `scrape`, which removes from
      a Set but doesn't return what it removed; List and Map had no
      removal at all), then added `pop(list)` / `pop(list, i)` /
      `pop(map, key)` / `pop(set, item)` as one polymorphic builtin, the
      return-value counterpart to `push`/`sprinkle`/`scrape`. List
      indexing doesn't wrap negative, matching plain `xs[i]`; a missing
      Map key reads as `nobox` rather than erroring, matching every
      other Map read, but a wrong-type key still errors like `map[key]`
      does; Set follows `scrape`'s permissiveness instead, with no type
      check on `item`. Added `object.Map.Delete` (mirroring `Get`/`Set`)
      since no removal method existed on Map before this. Verified via
      Go unit tests and the real `crust run` CLI

## Phase 6 — Tooling
- [x] CLI: `crust run <file> [--store=<name>]` (Phase 4's interpreter
      makes this real; the bare-file shorthand and `--store` entry-point
      selection both work — see SPEC.md §9)
- [x] REPL mode (`crust repl`, `cmd/crust/repl.go`) — one persistent
      Interpreter/Environment for the whole session, so state (variables,
      recipes) carries across lines. Multi-line continuation (an unclosed
      `{`/`(`/`[`, or a trailing operator) detected by checking whether
      the parser's own last error mentions EOF, rather than a hand-rolled
      bracket counter -- reuses the real parser's own notion of
      "incomplete" instead of a second, approximate one that could
      disagree with it. Only a bare expression statement's value
      auto-echoes (matching most REPLs' convention); a runtime error is
      reported but doesn't end the session. Known limitation: `unbox()`
      (no argument) still reads from the same stdin the REPL's own line
      scanner consumes -- fine on a real interactive terminal (each read
      only sees what's currently typed), but piped/redirected input can
      have already been buffered ahead by the scanner. `crust run` is
      the intended way to process real stdin input; the REPL is for
      trying things out.
- [x] Source formatter (`internal/format`, `crust fmt`, and
      `crust lsp`'s `textDocument/formatting`) — an AST-based
      pretty-printer, not a token-stream reflow: reprints the parsed
      `*ast.Program` with one canonical style (consistent spacing,
      `order (...) {` / `} combo (...) {` / `} special {` cuddled-brace
      layout, 4-space indentation) and recomputes the minimal-but-
      correct set of parens from operator precedence/associativity
      rather than preserving whatever parens the source happened to
      have (`internal/parser` drops grouping parens entirely, so there
      is nothing to preserve — every paren in the output is derived,
      traced per node type against the parser's own precedence-
      climbing behavior). Comments are invisible to the AST
      (`internal/lexer` strips them before the parser ever sees a
      token), so `internal/format/comments.go` runs a second,
      standalone, string-literal-aware scan over the raw source to
      recover them and re-interleave them (plus original blank-line
      paragraph breaks) into the printed output. Verified by running
      every real file in `examples/*.crust` through both the original
      and the formatted-then-reparsed source and diffing `deliver()`
      output byte-for-byte (100% match) — the strongest evidence the
      formatter never changes a program's meaning, on top of the usual
      Go unit tests (100% package coverage). `crust fmt <file>` prints
      to stdout by default, `-w` writes in place — gofmt's convention,
      not rustfmt's. `crust lsp` advertises
      `documentFormattingProvider` and answers
      `textDocument/formatting` with a single whole-document `TextEdit`
      (empty, not an edit, when the document's already canonical, so
      format-on-save doesn't touch mtime/undo history for nothing) —
      verified against a real `crust lsp` subprocess over the wire, not
      just Go tests. See the Neovim snippet in README.md's
      [Language server](./README.md#language-server) section for
      wiring up format-on-save via `vim.lsp.buf.format()`.
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
- [x] (Stretch) Further LSP: incremental (as opposed to full) document
      sync, code actions — extending `internal/lsp` (above) rather than
      building from scratch, from the same direct request that asked
      for the AoC fetch/timer feature above it. **Incremental sync**
      (`sync.go`'s `applyContentChange`/`positionToByteOffset`): the
      server now advertises `TextDocumentSyncKind.Incremental` (2, was
      `Full`/1) in `initialize`, and `handleDidChange` applies each
      `contentChanges` entry — a Range plus replacement text — in
      order against whatever the document already is, splicing at a
      byte offset computed by reusing `position.go`'s existing
      `decodeOffset` (Position.Character, in the negotiated encoding,
      -> a rune count within that line) and `encodeOffset` in `"utf-8"`
      mode (that same rune count -> a byte offset), rather than adding
      new offset-math — the encoding-aware plumbing this needed already
      existed for hover/definition/etc., just not wired to convert a
      Range into a document-wide byte span. A `Range`-less entry (a
      full-document replace) is still accepted exactly as before,
      since not every client honors the server's advertised sync kind
      on every single edit — defensive, not a compromise. **Code
      actions** (`codeaction.go`'s `codeActionsFor`): `textDocument/
      codeAction` offers one action, "Format document" (kind
      `source.fixAll`), built by reusing `formatting.go`'s
      `formatDocument` directly — including its existing idempotency
      check, so an already-canonical file offers no action rather than
      a no-op edit, the same guarantee `crust fmt -w`/format-on-save
      already give. No new diagnostic-guessing logic: cRust's
      diagnostics are lex/parse errors with no mechanically safe
      generic fix, so the one action offered is deliberately the
      already-tested, always-safe one rather than a fragile guess at
      "what did you mean." Verified with table-driven unit tests
      (`sync_test.go`: insert/replace/delete, a multi-line span,
      sequential changes composing, utf-8 multi-byte columns, an
      out-of-range Position clamping instead of panicking;
      `codeaction_test.go`: offered/not-offered/unparseable), two new
      full-wire `Server.Run` tests (`lsp_test.go`:
      `TestServerIncrementalDidChange` inserting then deleting an
      illegal `@` character via real Range-based edits and checking the
      diagnostics actually track it; `TestServerCodeActionOffersFormat`
      confirming the wire-level JSON), and a real `crust lsp` subprocess
      session (a Python script speaking raw JSON-RPC/stdio, not Go
      tests) confirming `initialize` advertises both new capabilities
      and that incremental edits and the code action's returned
      `WorkspaceEdit` are both correct over real bytes end to end.
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
      per call site) + `crust develop <file> [--store=<name>] [--plain]
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
- [x] (Stretch) debugger UX follow-ons, from real usage: (1) a real bug
      — the KPI tab's two pie charts don't scale down for a small
      terminal, and with no alt-screen or height clamp in place, a
      window shorter than that content scrolled the tab bar and header
      (printed first) right off the top, so the tabs looked like they'd
      vanished; fixed with `tea.WithAltScreen()` plus a `clampHeight`
      that trims the whole rendered frame to the window's actual row
      count, replacing whatever's cut with a one-line note, so the tab
      bar is always the first thing on screen regardless of window size
      or which tab is showing. (2) Readability: a long recipe call or
      loop body can run for dozens of rows in the Stepper tree (or the
      `--plain` output), and indentation alone doesn't make it easy to
      tell where one ends — every frame/step with children now gets a
      `// end ...` row right after its subtree, at the same depth as
      its own opening row, mirroring matching braces. (3) A third
      **Editor** tab hands the whole terminal to a real `nvim` on the
      file being debugged (`tea.ExecProcess`, the standard bubbletea
      pattern for shelling out to a full-screen program) rather than
      rendering an editor pane inline — embedding one would need a full
      terminal emulator layered into this TUI just to draw nvim's own
      screen, a much bigger and more fragile build than reusing a real
      editor. Switching to the Editor tab launches nvim immediately, no
      enter needed; inside it, nvim behaves completely natively (`:w`
      saves and keeps editing) — an earlier version forced a quit after
      every save so it could refresh, which made `:w` indistinguishable
      from `:wq` and read as janky rather than "the debugger updates,"
      per direct feedback. Now the debugger only rechecks anything once
      nvim actually exits (however the user chose to: `:wq`, `:x`,
      `ZZ`, plain `:q`), tells "something was saved" apart from
      "nothing was" by comparing the file's mtime before/after, and — on
      a save — reruns the file, refreshes the Time/Memory/Stepper tabs,
      and switches to the Time tab; it does not reopen nvim, respecting
      that the user already chose to quit. All of this verified via a
      real pty (a small terminal no longer loses the tab bar; `// end`
      markers appear in both the TUI and `--plain`; a real `nvim`
      session confirmed the tab-switch auto-launch, that `:w` alone
      leaves nvim open, and that `:wq` persists the save, reruns the
      file, and lands on the Time tab), not just Go unit tests — see
      ARCHITECTURE.md's debugger section for the design notes,
      including why the Editor tab is a hand-off and not an embedded
      pane.
- [x] (Stretch) two more debugger tabs, from direct user requests:
      (1) the KPI tab's two pie charts were split into their own **Time**
      and **Memory** tabs, each getting the full window instead of
      sharing one (the old tab also had width-dependent stacked/
      side-by-side layout logic for the two charts, which the split
      removed outright rather than kept unused) — Memory's overall
      numbers swap "total time"/"slowest statement" for "largest single
      value" (`largestValue`, `slowestStep`'s size-based counterpart),
      since a duration doesn't apply to a memory reading. (2) A new
      **Run** tab: type a path to an input file and press enter to run
      the file being debugged with it as stdin, showing the raw output —
      genuinely "the command line" (`runFile`, the same function `crust
      run` itself calls, not `debugger.Recorder` at all, so there's no
      tracing overhead or KPI bucketing in the way). The path field is
      hand-rolled (insert/delete/cursor around a rune slice) rather than
      pulling in a components library (e.g. `charmbracelet/bubbles`)
      for a single-line text box — a third TUI dependency, after
      bubbletea + lipgloss, wasn't worth it for this. Getting the field
      to work at all meant a real design point: on every other tab, `q`
      quits and `h`/`j`/`k`/`l` navigate, but a file path can contain
      any of those letters, so the Run tab needed its own key handler
      (`handleRunTabKey`) that gives the field almost everything typed
      and reserves only Tab/Shift+Tab (switch tabs), Enter (run), and
      Ctrl+C/Esc (quit) — verified via a real pty typing a path
      containing every one of those letters and confirming none of them
      quit or navigated.
- [x] (Stretch) Run tab entry-point selector, from a follow-up request:
      a file can declare more than one `store`/`store_<name>` recipe
      (SPEC.md §9), and the Run tab had no way to pick which one to
      call — it always ran with whatever `--store` the original `crust
      debug` invocation used. Added a second row below the input-file
      field, populated by scanning the file for its entry points
      (`debug_view.go`'s `scanEntryPoints`, reusing `run.go`'s
      `collectEntryPoints` — the exact same question `crust run`
      itself answers when picking a default) and listing them (`""`,
      the bare `store`, labeled `(default)`); the row is hidden
      entirely for the common case of a file with no store recipes at
      all. ↑↓ moves focus between the input field and this row; ←→
      cycles the selection when the row has focus (and still moves the
      text cursor when the field does — same keys, different meaning
      depending on which row is active) or lets Enter run without ever
      touching it, defaulting to whichever entry point the rest of the
      TUI is already showing. Re-scanned after every Editor-tab reload,
      since a save could have added, renamed, or removed a `store_`
      recipe. Verified via a real pty: switching focus to the row,
      cycling to a second entry point, and pressing enter actually ran
      that recipe's own output, not the default's.
- [x] (Stretch) `enumerate(list)` builtin, for an easy enumerated loop:
      pairs each element with its 0-based index, as a `(index, value)`
      Tuple, so `knead pair in enumerate(xs) { i, x = pair; ... }` gets
      both using the tuple-unpack assignment sugar (§3.1) that already
      existed — no new loop syntax needed. Deliberately a Tuple, not a
      two-element List: List-unpack's "last target catches everything
      left over as its own List" rule (right for a variable-length
      remainder) would silently wrap the value in `i, x = pair` as
      `[value]` rather than binding `x` to the bare value, since a
      2-element List unpacked into exactly 2 targets isn't the same
      case list-unpack's rule was built for; a Tuple's exact-arity
      unpack avoids that trap. That does mean every element must be
      Hashable, checked up front the same way `combos()` already checks
      it (a Tuple's `HashKey()` does an unchecked type assertion on
      each element, so an unhashable one would panic rather than error
      gracefully the moment the pair was ever used as a Set element or
      Map key) — so `enumerate()` can't pair positions with a List/Map/
      Grid value directly, which is fine, since cRust already has an
      ordinary counted loop (`knead i in 0.<slices(xs)`) for indexing
      into exactly that kind of collection. Verified both the happy
      path and the unhashable-element error via a real `crust run`
      subprocess, not just Go unit tests.
- [x] (Stretch) `contains(collection, item)` builtin, for a general
      membership test — asked as "an `in` keyword for dictionaries and
      lists and sets," a real `in` infix expression was offered as the
      fuller option, and a plain function was picked once asked, since
      it needs zero parser changes (`in` keeps its one existing role,
      the `knead item in collection` loop header). Covers List/Tuple
      (linear scan, compared with the same value-equality `==` uses),
      Set (`topped`'s own `Has`, reused rather than duplicated), and
      Map (membership by *key*, matching Python's `k in dict`
      convention). Building this is what moved the interpreter's
      `==`/`!=` equality logic out of `internal/interpreter` and into
      `internal/object` as an exported `object.Equal` — `contains()`
      needed the identical rule `==` already implements, and
      `internal/builtins` can't import `internal/interpreter` to reach
      it (the dependency runs the other way), so the choice was
      reimplement-and-risk-drift vs. relocate-and-share; relocating
      won. A pure move, not a behavior change — every existing
      `==`/`!=` test still passes unmodified, and `internal/object`
      picked up its own direct test suite for the relocated logic to
      keep that package back at 100% coverage. Verified via a real
      `crust run` subprocess across List/Tuple/Set/Map, including that
      a Map's *values* don't register as members, only its keys.
- [x] (Stretch) `find(collection, value)` builtin — first built as a
      predicate search (`find(iterable, fn)`, JS's `Array.find` shape)
      from "a library function that searches for an element in a list
      and returns the one at the lowest index that matches," then
      corrected once the actual call shape was spelled out
      (`find(collection, value_of_element_to_find)`): a plain value to
      search for, and the *index* as the answer, not the element —
      `contains`'s positional counterpart ("is `value` here" vs. "where
      is `value`"). No longer calls back into user code at all; shares
      `indexOfElement` with `contains`'s own List/Tuple case, so
      there's one definition of "where does this value live," not two.
      Only List/Tuple (a Set has no position to report; a Map's
      iteration order isn't meaningful the way an index promises).
      `nobox` on no match or an empty collection. The `object.IsTruthy`
      relocation the predicate version needed stayed in place even
      after `find` stopped needing it — the interpreter's call sites
      were already switched over, no reason to move them back. Verified
      via a real `crust run` subprocess: exact value search on a List
      and a Tuple, value equality treating `2`/`2.0` as the same number
      (matching `==`), no match, and an empty collection.
- [x] (Stretch) `pizzasort(list)` builtin, asked for as "whatever
      sorting method is smartest" — three implementation approaches
      were pitched before writing any code: hand-roll an introsort,
      detect the input's shape and dispatch to a specialized algorithm
      per shape (e.g. counting sort for narrow-range integers), or
      delegate to Go's own `slices.SortFunc`, which since Go 1.19
      already *is* pattern-defeating quicksort — insertion sort for
      small partitions, a heapsort fallback bounding the worst case,
      cheap detection of already-sorted/reverse-sorted/many-duplicate
      input. The third won: it delivers everything "smartest" implies
      without cRust owning a sort algorithm, matching the same
      reuse-well-tested-machinery instinct that already shaped `min`/
      `max` and the equality/truthiness relocation two entries above.
      A follow-up question — natural order only, or a comparator/key
      function for custom ordering (descending, sort-by-field) — was
      asked directly rather than guessed at; natural-order-only (same
      rule `<`/`min`/`max` already use: numbers freely mixed, or
      Strings, never both) is what shipped, with a comparator-function
      variant flagged as a real, larger follow-on if it's ever needed.
      Reuses `compareTwo` (the exact comparison `min`/`max` already
      needed) rather than adding a second helper; every element is
      checked against one shared reference up front, before sorting
      starts, so the actual sort pass never needs its own error path.
      Always returns a new List regardless of whether the input was a
      List or Tuple, the same convention `map`/`combos`/`enumerate`
      already settled on — a Tuple can't be sorted in place anyway, and
      a List's own contents shouldn't get silently rewritten by
      something that reads like a question, not an action. Verified via
      a real `crust run` subprocess: integers, strings, mixed Int/Float,
      empty and single-element inputs, and a clean runtime error (not a
      panic) for a genuinely incomparable mix.
- [x] (Stretch) `copy(value)` builtin, asked for as "a copy function so
      items don't continue being modified by reference" — List/Map/Set/
      Grid are all Go pointer types with reference semantics, so
      assignment/passing today shares the same underlying object and
      mutations (`push`, `setAt`, index assignment, `sprinkle`/`scrape`)
      show up through every reference. The recursion turned out to be
      narrower than "copy everything": Tuple elements, Set elements, and
      Map keys must all be `Hashable`, which already rules out any
      mutable container living inside one of those, so a Tuple/Set/Map's
      own keys are already as independent as a copy could make them —
      only List elements, Map *values*, and Grid cells can hold an
      arbitrary Object (including another List/Map/Set/Grid) and
      actually need recursive copying. Lives as `object.DeepCopy` in
      `internal/object`, the same location `object.Equal`/
      `object.IsTruthy` already established for logic both
      `internal/interpreter` and `internal/builtins` need; `copyFn` is a
      one-line wrapper over it. Cycle-safe via a `seen map[Object]Object`
      memo (original -> its already-built copy), registered right after
      a new container is allocated and before its contents are filled
      in, so a self-referential structure (`xs = [1]; push(xs, xs)`)
      reuses the in-progress copy instead of recursing forever — the
      same technique Python's `copy.deepcopy` uses, with the side effect
      of preserving shared substructure within one copy. Verified via a
      real `crust run` subprocess: a plain List, a List nested in a
      List, a List value inside a Map, a List cell inside a Grid, a
      self-referential List (doesn't hang), and pass-through for a Tuple
      and a scalar.
- [x] (Stretch) `list[start..end]` / `list[start.<end]` slice notation
      for List/Tuple/String, asked for alongside an explicit choice of
      syntax ("cRust range-flavored" `..`/`.<` reuse vs. Python's
      `:`/`::step`) — the range-reuse option won, needing zero
      parser/AST changes since `index = "[" expression "]"` already
      accepted any expression and `..`/`.<` were already registered
      infix operators, so `xs[0..4]` was already parsing as an index
      whose Index is a RangeExpression before this feature existed;
      the only gap was `evalIndexExpression` not knowing what to do
      with one (it would eagerly materialize `0..4` into a List of
      Integers via the range-statement path and then reject that List
      as "not an Integer," same as any other bad index). Fixed by
      detecting a RangeExpression index and routing to a new
      `evalSliceExpression` instead. Negative bounds count from the
      end (`-1` is the last element); unlike a bare range statement, an
      out-of-order slice doesn't come back empty — it reads backward,
      which is what makes `xs[-1..0]` mean "last element back to the
      first," exactly what was asked for. `.<` drops whichever end the
      walk is heading toward. Out-of-range bounds are a runtime error,
      not a silent Python-style clamp, matching how plain indexing
      already behaves. Only List/Tuple/String are sliceable (the same
      position-indexable types plain indexing already supports); no
      step component, flagged as a real, larger follow-on if it's ever
      needed. Verified via a real `crust run` subprocess: inclusive/
      exclusive forward slices, negative-bound forward and reversed
      slices, single-element inclusive/exclusive slices, a
      backward-exclusive slice, no mutation of the original, an
      out-of-range error, and a Set correctly rejected as unsliceable.
- [x] (Stretch) `list(x)`/`tuple(x)`/`set(x)` collection-conversion
      builtins, asked for as "a way to convert collections to other
      collection types... like a list to a tuple or vice versa" —
      answering the question first turned up partial, accidental
      coverage: `gather(list)` only ever went List -> Set, and
      `map`/`pizzasort` happen to always return a List regardless of
      List or Tuple input, a usable but unnamed Tuple -> List trick.
      The three new builtins round that out symmetrically: each
      accepts any of List/Tuple/Set and normalizes to its own target
      type, named to match the existing `str`/`int`/`float`/`bool`
      conversion builtins rather than a pizza pun. Share a new
      `asElements(x) ([]Object, bool)` helper for extracting a
      List/Tuple/Set's elements into a fresh slice (a Set drains its
      backing Go map into one, coming back in whatever order Go's map
      iteration happens to visit -- a faithful reflection of Set being
      unordered, not a limitation). `list`/`set` on their own type
      still build a new container (shallow copy, same convention as
      Python's own `list()`/`set()`) rather than handing back the same
      object -- deliberately not what `copy()` does, since `copy()`
      goes deep specifically to break reference-sharing at every level,
      while these only need the outer container to be independent.
      `tuple`/`set` reject an unhashable element with the same message
      shape `gather` already used. Verified via a real `crust run`
      subprocess: every List/Tuple/Set conversion direction (including
      Set -> List and Set -> Tuple, which had no route at all before),
      duplicate-dropping on `set()`, `list()`'s shallow-copy behavior,
      an unhashable-element error, and a wrong-type error.
- [x] `freq(x)` builtin, asked for directly: "a freq(l) function that
      gives me a dictionary of the frequency of items in a list" --
      Python's `collections.Counter` by another name. Reuses
      `asElements` (above) a fourth time, so it takes any of
      List/Tuple/Set rather than being List-only; a Set argument is a
      legal if uninteresting case (every count comes back 1, since a
      Set's own members are already unique). Every element must be
      Hashable to become a Map key, same requirement and same error
      message shape `gather`/`tuple`/`set` already use. Verified via a
      real `crust run` subprocess: counts across repeated Strings and
      Integers, a Tuple argument, a Set argument, an empty List, and an
      unhashable-element error.
- [x] `keys(m)`/`values(m)` builtins, asked for directly: "a keys and
      values function for dictionaries." The obvious first
      implementation -- each function independently ranging over the
      Map's own backing Go map -- has a real correctness trap: Go
      randomizes map iteration order per range statement, not per map,
      so two separate calls (`keys(m)` then a later `values(m)`)
      aren't guaranteed to visit entries in the same relative order --
      `keys(m)[i]`/`values(m)[i]` could silently end up as two
      different pairs. Fixed with a shared `sortedMapPairs(m)` helper
      that sorts by each key's `Inspect()` text -- a plain function of
      the Map's current contents, so both calls agree by construction.
      The sort order itself is arbitrary (lexicographic text, not
      numeric for Integer keys); the only property that matters is
      `keys`/`values` always agreeing with each other. `knead k in m`
      still iterates a Map's keys directly for an ordinary loop;
      `keys`/`values` are for wanting real Lists back, e.g. to zip by
      index. Verified via a real `crust run` subprocess, run 3 times in
      a row specifically to catch the flake this was built to prevent,
      confirming `keys(m)[i]`/`values(m)[i]` always matched a direct
      `m[keys(m)[i]]` lookup -- and a Go test
      (`TestKeysAndValuesCorrespondAcrossSeparateCalls`) makes the same
      check permanent rather than relying on manual reruns.
- [x] Fixed: a single-recipe program's whole `crust develop` run showed
      up in the KPI/stepper as one anonymous `call(...)` frame instead
      of the recipe's own name -- reported as "if I have a single
      store_part1 function it only lists that, not any of the inside
      parts." The loop/order breakdown was actually there, nested one
      level down; it was specifically the top frame (and its "by self
      time" table row) reading as a bare, generic `call(...)`. Root
      cause: `cmd/crust` resolving/running the `store`/`store_<name>`
      entry point (both `crust run` and `crust develop`) went through
      `Interpreter.Call`, the same method `map()`'s builtin uses for
      its per-element callback -- and `Call` hardcodes its frame label
      to `"call(...)"`, right for `map` (no reason to label each
      element call distinctly), wrong for an entry point that has a
      real name sitting right there in the already-resolved `target`
      variable. Fixed with a new `Interpreter.CallNamed(fn, args,
      name)`, used by `run.go`/`debug.go` in place of `Call`; `Call`
      itself is untouched. Verified via a real `crust develop --plain`
      subprocess: the tree and KPI table now read `store_part1(...)`;
      a second case with a helper recipe called inside a loop confirmed
      nested named calls were already breaking out correctly on their
      own, isolating the bug to the entry point's own frame.
- [x] `crust debug` renamed to `crust develop`, on direct request --
      CLI-facing only (dispatch string, usage/help text, error
      messages, doc comments naming the command). Internal Go
      identifiers (`debugOptions`, `runDebug`, `debugView`, ...) and
      the `debug*.go` file names stayed as-is -- a much bigger, purely
      cosmetic rename of implementation details nothing outside
      `cmd/crust` sees. Found and fixed a real bug in the same pass:
      `runDebugEntryPoint` silently did nothing when the entry point
      wasn't found -- exactly the confusion that prompted the rename
      request. A file with `store_part1`/`store_part2` but no bare
      `store`, run with no `--store`, recorded nothing but the
      top-level recipe declarations and looked like `crust develop`
      itself was broken rather than like "you forgot `--store`."
      `run.go`'s `runEntryPoint` already had the right three-way
      behavior for this (name the missing `--store=<name>`; list
      available entry points if none was given but some exist; stay
      silent if the file has no store-family recipe at all) --
      `runDebugEntryPoint` now returns the same errors, reusing
      `collectEntryPoints`/`storeFlags` rather than duplicating them,
      surfaced by `buildDebugView` the same way a parse error already
      is. Verified via a real `crust develop` subprocess reproducing
      the exact reported scenario before and after the fix; two new Go
      tests cover both new error paths.
- [x] Run tab's entry point now also drives Time/Memory/Stepper, on
      direct request: "make it so whatever store is selected in the run
      tab... is the one it does timings and memory checks and step
      debugging against." Before this, the Run tab's selector only
      affected the Run tab's own raw output -- the rest of the TUI
      stayed pinned to whatever `--store` the session started with.
      Tied to pressing enter (run), not to cycling the selector itself,
      since running is the only point an input file actually gets
      read -- retracing then means the KPI data reflects a real run
      against real input, not a phantom trace against nothing.
      `runProgramCmd` now reads the input file's bytes once and feeds
      two independent readers to two separate executions: the existing
      untraced `runFile` call for the Run tab's raw output (unchanged),
      and a second, traced run via `buildDebugView` (the same function
      the Editor tab's save-triggered reload already uses) whose
      recording replaces `m.view`. `m.opts.Store` gets updated too, so
      the selection stays "current" for the rest of the session,
      including later Editor-tab reloads. A failed retrace leaves the
      previous recording in place rather than blanking those tabs out.
      Verified with three Go tests: the retrace reflects the selected
      entry point's body and not the other one's, reads the same input
      file the raw run used, and a failed retrace leaves `m.view`
      untouched.
- [x] Per-file Run tab settings (store + input file) now persist across
      `crust develop` invocations, on direct request: "a saving of user
      settings for each develop on a crust file... what file is used as
      input... last entered by the user, and... which store it's
      using." One JSON file under the user's config directory
      (`os.UserConfigDir()`), keyed by each debugged file's absolute
      path -- not a dotfile next to every `.crust` file. Saving is tied
      to the Run tab's "run" action, the same action the previous
      entry's Time/Memory/Stepper linkage already hooks -- the point
      both values are simultaneously current. `runDebug`'s new
      `applySavedStore` fills in an unset `--store` from the saved
      value before the very first recording is built, so `--plain` and
      the TUI's initial tabs reflect it too, not just the Run tab after
      a manual re-run; an explicit `--store` flag still always wins.
      `runDebugTUI`'s new `restoreRunInput` pre-fills the Run tab's
      input field at startup. Writes go to a temp file then
      `os.Rename` into place (atomic, so an interrupted write can't
      corrupt the whole settings file), and every persistence call is
      best-effort -- a write failure or a missing/corrupted settings
      file never blocks or fails `crust develop` itself, since
      remembering settings is a convenience layered on top of the tool
      working. Verified via a real `crust develop --plain` subprocess
      (pre-seeding the state file, confirming an unset `--store` picks
      it up, and that an explicit `--store` still overrides it) plus Go
      tests covering the load/save round trip, multiple files
      coexisting, overwriting an entry, and config-directory/write
      failures (via a blocking file/directory in the way rather than
      permission bits, since this sandbox runs as root).
- [x] Fixed: `crust develop` crashed outright on some WSL setups --
      `error creating cancelreader: bubbletea: error creating cancel
      reader: add reader to epoll interest list`, reported verbatim.
      Traced to bubbletea's `github.com/muesli/cancelreader` dependency:
      on Linux it type-asserts its input to a `File` interface
      (`io.ReadWriteCloser` + `Fd()` + `Name()`), and when that
      succeeds (always true for `os.Stdin`) registers the fd with
      `epoll_ctl` so `Cancel()` can interrupt a blocking read --
      registration that fails outright under this user's WSL setup,
      with no fallback, before the TUI ever draws a frame.
      `runDebugTUI` was opting into this explicitly
      (`tea.WithInput(f)` on the raw `*os.File`, even though bubbletea
      already defaults to `os.Stdin` when no `WithInput` is given at
      all). Fixed with a `stdinOnlyReader{io.Reader}` wrapper: embedding
      only the `io.Reader` interface promotes just `Read`, so the
      wrapper has no `Fd`/`Name`/`Write`/`Close`, the type assertion
      fails, and cancelreader falls back to a plain blocking read (no
      epoll, works everywhere). The one thing that fallback can't do --
      interrupt a read that's already blocked, from outside the read
      call -- was checked against every way `crust develop` actually
      quits (a keypress, or handing off to nvim) and found to be a
      non-issue, since every quit path *is* the next keypress arriving.
      Verified the mechanism directly (can't reproduce a WSL-specific
      epoll failure in this environment): a Go test confirms `*os.File`
      satisfies a locally mirrored copy of cancelreader's own `File`
      shape while `stdinOnlyReader` wrapping that same file does not,
      plus a read-through test confirming the wrapper still works as an
      ordinary reader.
- [x] Fixed a deeper, related bug reported right after the above: "my
      input after running developer doesn't correctly input into the
      develop tool." Launching the interactive TUI still ran the
      program first, against real process stdin, before bubbletea ever
      took the terminal over for its own keyboard input -- any
      store/store_<name> recipe calling unbox() would silently consume
      the user's next keystrokes as puzzle input instead of them
      reaching the TUI. Built exactly the fix the user proposed: don't
      require --store/an input file up front -- start the TUI with "no
      data" in Time/Memory/Stepper, and only populate them once the
      user runs from the Run tab (which already reads an explicit input
      file, never real stdin). runDebug now branches before running
      anything: --plain/non-tty still runs eagerly via the existing
      buildDebugView (no Run tab to defer to there); the real-terminal
      case calls a new emptyDebugView instead, which parses the file
      (so a syntax error still surfaces immediately, and the Run tab's
      entry-point selector still works) but runs nothing. Empty-
      recording rendering needed no new UI work -- that shape was
      already handled and already tested. The Editor tab's
      save-triggered reloadCmd had the identical latent exposure (a
      save still re-ran the program against real stdin) and got the
      same fix while already in this code: it now reads stdin from the
      Run tab's own input-file field instead, read fresh each reload;
      an unreadable input path degrades to empty input rather than
      failing the reload, since an unrelated code edit shouldn't be
      blocked by a stale path. Verified via a real crust develop
      --plain subprocess that the eager-run path is completely
      unchanged. Go tests cover emptyDebugView directly and reloadCmd's
      new input source (the Run tab's file content reaching the
      retrace, a missing path degrading gracefully, the no-input-file
      case still producing a real recording).
- [x] Fixed a regression the `stdinOnlyReader` fix above introduced:
      the user's next report showed the TUI's own help text and their
      raw typed keystrokes (h/j/k/l, arrows) both garbled together in
      the rendered output -- classic terminal echo, meaning the
      terminal was never put into raw mode. `stdinOnlyReader` hides
      `Fd()` to defeat cancelreader's `File` type assertion (the epoll
      fix above), but bubbletea's own `initInput` (tty_unix.go) uses a
      second, separate type assertion to a narrower `term.File`
      shape (`io.ReadWriteCloser` + `Fd()`, no `Name()`) to decide
      whether to call `term.MakeRaw()` at all -- hiding `Fd()` defeated
      that check too, so raw mode (no local echo, no line buffering)
      never engaged. Fixed by replacing `stdinOnlyReader` with
      `stdinNoNamer{f *os.File}`: a named (non-embedded) field with
      explicit forwarding methods for `Read`/`Write`/`Close`/`Fd()` but
      not `Name()` -- satisfying `term.File` (raw mode engages again)
      while still failing cancelreader's stricter shape that also wants
      `Name()` (epoll still avoided, so the original crash stays fixed).
      Verified with Go tests asserting `stdinNoNamer` satisfies a
      mirrored `term.File` shape and fails a mirrored `cancelreader.File`
      shape, plus a read-through test; confirmed via grep that
      bubbletea never calls `.Close()` on the input directly, so the
      wrapper's `Close` passthrough is safe. No real pty in this
      sandbox to confirm raw mode engaging end-to-end, so this rests on
      the interface-shape tests and reading bubbletea's/cancelreader's
      source directly, same as the original epoll fix.
- [x] Added: Run tab entry-point list now refreshes on every clean
      editor exit, not only after a detected save. It already refreshed
      after a save (handleReload recalculated it against the freshly
      re-recorded view), but that path depends on an mtime-diff
      heuristic (openEditorCmd compares the file's mtime before/after
      nvim runs) that could in principle miss a save landing inside the
      same mtime-resolution window the file was opened in. Added
      debugView.refreshEntryPoints (clears entryPoints()'s cache so the
      next call re-reads and re-parses the file) and now call it --
      plus recompute runEntryIndex the same way handleReload already
      did -- unconditionally in handleNvimExit, before the msg.saved
      branch that decides whether to also do a full reloadCmd retrace.
      Kept the two independent on purpose: rescanning entry points is
      just a lex+parse, cheap enough to always do, while a full retrace
      stays gated on an actual save, since re-running the file's store
      recipe (and any unbox() calls in it) isn't free or side-effect-
      free. Verified with Go tests that edit the file on disk and call
      handleNvimExit directly with saved: false, confirming both the
      option list and the selected index update anyway, and that a
      clean saveless exit still doesn't trigger reloadCmd itself.
- [x] Adaptive pie chart radius on the Time/Memory tabs — one of six
      "what could I add to the develop tool" ideas offered on direct
      request, picked to start on along with a web playground. Fixed at
      7 before this; `pieRadius` now derives it from the window's own
      height/width and the current KPI count (one legend line each),
      the same "how much is already spoken for" reservation
      `stepperBodyHeight`/`benchChartHeight` already do for their own
      tabs — shrinks down to a floor of 3 on a short terminal instead
      of `clampHeight` clipping the chart's bottom rows off, never
      grows past the original 7 (nothing asked for a *bigger* chart on
      a roomy terminal). Verified with table-driven Go tests and a real
      pty-driven session comparing a 50-row and a 15-row terminal: the
      short one's circle visibly shrank and the legend/stats stayed
      fully on screen, where before this the tab's own content would
      have run past the bottom.
- [x] Export Bench results to CSV (`e`) — another of the same six
      ideas. Writes the current batch to `<file>.bench.csv` (swapping
      the debugged file's own extension, so `day01.crust` exports to
      `day01.bench.csv` right next to it, overwritten on each export —
      a snapshot of "the last batch I ran," not an accumulating
      history). One row per run: its 1-based index, raw duration in
      nanoseconds, bytes allocated, and whether it failed.
      Deliberately not the chart's own already-computed average/
      median/max/min — a spreadsheet's own formulas already derive
      those from the raw column, and repeating them here would just be
      a second copy that could drift out of sync with the tab's own
      `benchAvgOf`/etc. A one-line status ("exported to ..." or
      "export failed: ...") shows the outcome, cleared on the next run
      since it describes a batch that's no longer the one on screen.
      Verified with Go tests (the path derivation, the exact CSV
      content against known input, an unwritable-directory error path)
      and a real pty-driven session confirming the exported file's
      numbers matched the chart's own.
- [x] A variable/environment watch panel on the Stepper tab (`v`) — the
      last of the six ideas, and the one expected to matter most day to
      day: a step's own "out" column only ever showed what that one
      statement itself evaluated to, never the full variable state
      around it. Required real plumbing, not just a UI change:
      `object.Environment` gained a `Snapshot()` method flattening the
      whole scope chain (inner shadows outer, the same rule `Get`
      already resolves by) into one point-in-time `map[string]Object`;
      `trace.StepEvent` and `debugger.Step` each gained an `Env` field
      carrying that snapshot, taken *after* `Eval` returns (the same
      "after the fact" timing `Out`/`Dur` already use) so a step's own
      effect — a new assignment, a loop variable's binding — is
      included in its own snapshot rather than left for the next step
      to pick up. Deliberately shallow: `Snapshot()` copies which
      Object each name points to, not a recursive deep copy of every
      value's own contents (`object.DeepCopy`, `copy()`'s own
      implementation) — cheap enough to take on literally every traced
      statement (confirmed via `BenchmarkTraced`: adding it moved the
      benchmark by only noise, since tracing was already several times
      slower than untraced from the timing/size-computation overhead
      already there). The one real tradeoff that comes with going
      shallow: a later *in-place* mutation of a shared List/Map/Set/
      Grid (`push`, `setAt`, ...) is still visible through an old
      snapshot, since only the *binding* was frozen, not the value's
      own contents — Integer/Float/String/Boolean/Tuple are all
      immutable in cRust, so this only ever affects those four
      container types, and only when something still holds a
      reference to the same one after the snapshot was taken.
      `viewStepperWatch` (`cmd/crust/debug_tui.go`) renders the
      selected step's own `Env`, sorted by name, capped at 8 lines with
      a "+N more" note past that (a deep call stack can have dozens of
      visible names) — a frame or closing row has no `Step` of its own
      to read from, so the panel says so instead of showing the wrong
      row's data or silently nothing. `stepperExtraLines` grows to
      reserve room for it, the same pattern search/status already
      established. Verified with Go tests at every layer (`Snapshot`'s
      point-in-time behavior and shadowing in `internal/object`, the
      interpreter reporting each step's own effect in `internal/
      interpreter`, `Step.Env` landing correctly in `internal/
      debugger`, the panel's rendering/truncation/placeholder and key
      handling in `cmd/crust`) and a real pty-driven session: watched a
      top-level call's variables after it returned, then stepped inside
      the recipe itself and confirmed the panel correctly showed the
      local `x`/`y`/`total` *and* the enclosing `a`/`b`/`combine`, while
      correctly *not* showing the caller's own `c` — not yet assigned
      at that point in the run.
- [x] A full-value detail panel on the Stepper tab (`o`) — the sixth
      and last of the same batch of ideas. The table's own "out" column
      runs every value through `shortInspect`, which caps a value's
      text at `maxInspectRunes` to keep the column a fixed width —
      exactly right for scanning the tree, but the tail of a genuinely
      large List/Map is simply gone from the row itself. `o` shows the
      selected step's *untruncated* `Inspect()` text instead, in its
      own panel below the table, hard-wrapped across the panel's own
      width (`wrapRunes`) rather than the table's fixed column, capped
      at `maxDetailLines` (10) the same fixed-worst-case reservation
      shape the watch panel's own `maxWatchLines` already established.
      Same frame-row guard as the watch panel: no `Step`, no value, so
      it says so instead of showing the wrong row's data. Verified with
      Go tests (`wrapRunes`'s own wrapping/edge cases, the panel
      showing a value long enough that `shortInspect` would have
      truncated it, the frame-row placeholder, a nil `Out` reading as
      `nobox`) and a real pty-driven session: a 40-element List
      genuinely truncated with `…` in the table's own "out" column,
      then `o` showed the full 150-character value wrapped cleanly
      across several lines.
- [x] Bench regression baseline diff (`b`) — the last of the six
      ideas. `b` saves the current batch's average duration/memory as
      a named baseline, persisted per file the same `develState`
      mechanism `benchCount` already uses (`benchBaseline`,
      `restoreBenchBaseline`/`saveBenchBaselineBestEffort`); every
      batch after that (including the one that just saved it, showing
      +0.0%) prints how far its own average has moved from it as a
      signed percentage, until `b` is pressed again to replace it.
      Deliberately scoped to average only, not all four reference
      lines (average/median/max/min) the chart itself already tracks —
      median/max/min stay visible as absolute numbers on the current
      batch's own legend, so a diffed copy of those too would answer a
      question ("did the typical run get faster") the average alone
      already does. The saved baseline itself and the "baseline saved"
      confirmation toast are two different lifetimes: the baseline
      value has to outlive the run being compared against it, so it's
      untouched by a fresh batch, while the toast is cleared on one
      (the same event that already clears the CSV-export status),
      since it specifically means "just saved," not "a baseline
      exists." Verified with Go tests (the average computation, the
      percentage math including the zero-baseline no-op case, the
      persistence round trip preserving every other remembered
      setting, the toggle requiring at least one run, the two
      different clear-on-new-batch behaviors) and a real pty-driven
      session: ran a batch, saved it as the baseline (showed `+0.0%`,
      comparing it against itself), ran a second batch, and confirmed
      the diff (`+22.3%` runtime, `+12.5%` memory) matched the actual
      before/after averages by hand.
- [x] A richer Stepper tab, on direct request: "can you do richer
      stepper inside the develop tool?", narrowed via a clarifying
      question to three picks: search/filter the tree (this closes out
      the stretch item above), jump to next/prev failed step, and show
      each row's source line number. All three needed the same real fix
      first: the visible rows already stop descending into a collapsed
      "N iterations" fold, so a match or failure sitting inside one
      (exactly where it's likely to be — a real failure tends to show
      up on a *later* lap, not the first few still shown before folding
      kicks in) was invisible to a naive scan. `flattenAll` walks the
      whole tree regardless of fold state to actually find a target;
      `expandPathTo` marks every folded ancestor on the path to it as
      expanded; `jumpToNode` (shared by both features) expands, rebuilds
      the row list, and scrolls the cursor onto the now-visible row.
      `/` opens a search field (n/N repeats the same query forward/
      backward, vim-style); `f`/`F` jump to the next/previous failed
      step. One real surprise while testing against a real failing
      program: an error bubbles up through *every* enclosing
      statement's own result, so a loop and the `order` wrapping the
      real culprit both read as "failed" too, not just the one
      statement that actually raised it — already true of how those
      rows were colored before this feature existed, so `f`/`F` staying
      consistent with that was the right call, not a bug to paper over.
      See ARCHITECTURE.md's Phase 6 notes for the rest of the design
      (findNext's wraparound search, the line-number column, the
      reserved-line-height bookkeeping). Verified with table-driven Go
      tests and a real pty-driven session: ran a 5-lap loop whose 4th
      lap divides by zero, confirmed `f` expanded the fold and landed
      exactly on the failing statement, `/idiv` found the next match,
      and `F` stepped back to the original failure.
- [x] `crust bake documentation` (`internal/docsite`) — a browsable,
      pizza-themed reference site over local HTTP, in the shape of
      RFuller25/domainlang's `domain expansion: documentation` (embedded
      site + local HTTP server + best-effort browser launch), themed
      for cRust. Server-rendered Go html/template pages rather than
      domainlang's client-side JS app, since that's httptest-able and
      needs no hand-written Markdown parser (the doc strings only ever
      use backtick code spans, which renderInlineDoc handles directly).
      Keywords and Standard Library pages are read live from new
      lsp.KeywordDocs()/lsp.BuiltinDocs() exports (re-keyed copies of
      crust lsp's own hover tables) rather than a third hand-maintained
      copy of the vocabulary -- inheriting internal/lsp's own
      TestBuiltinDocsCoversEveryRealBuiltin guarantee for free, so this
      site can't go stale the way a generated-and-committed JSON file
      could. Light/dark pizza-crust theme via prefers-color-scheme, no
      JS beyond a small per-table filter box. `-p`/`--port` flag,
      Ctrl+C to stop. See ARCHITECTURE.md's Phase 6 notes for the full
      design writeup (per-page template parsing to dodge html/template's
      redefinition error, the listener-injection trick that makes the
      server itself testable, etc.) and verification (Go tests, a real
      subprocess + curl, and real headless-Chromium screenshots of both
      themes).
- [x] (Stretch) Run tab "run all stores" option, from a follow-up
      request: Ctrl+R toggles running every `store`/`store_<name>`
      entry point in sequence against the same input file instead of
      just the selector's current pick, concatenating output (a `===
      <name> ===` heading per store) and merging every store's timing/
      debug info into one combined Time/Memory/Stepper recording.
      `debugger.Recorder.Merge(label, other)` (`internal/debugger/
      recorder.go`) is the enabling piece: wraps another Recorder's
      `Roots()` under one new synthetic frame node, letting several
      independent recordings combine into one Stepper tree and one
      shared `Timing()` KPI pass with zero changes to either — `Timing`/
      `buildRows` both already walk purely by tree structure with no
      notion of "one run" baked in. Each store still gets its own full,
      independent `runFile`/`buildDebugView` call (own Interpreter,
      environment, `bytes.NewReader(data)`) rather than being chained
      through one shared Interpreter across the loop: `unbox()` with no
      argument reads whichever stdin was bound at construction
      (`internal/builtins.New`'s doc comment), so a shared Interpreter
      would leave every store after the first reading an
      already-drained reader instead of the fresh, full view of "the
      same input file" this option promises — exactly what separately
      typing `crust day01.crust --store=part1 < input.txt` and `...
      --store=part2 < input.txt` at a real shell would each get, which
      is what this reproduces. The toggle is remembered per-file
      alongside the existing entry-point/input-path settings
      (`debug_state.go`'s `develState.RunAll`). Verified with unit tests
      (`internal/debugger`'s `Merge`/merged-`Timing` tests, `cmd/crust`'s
      `runAllStoresCmd`/toggle/persistence tests) and a real pty-driven
      `crust develop` session: typed an input path, toggled Ctrl+R (hint
      text flips to "ON", instructions update), pressed enter, and
      confirmed both `store_part1`/`store_part2`'s output appeared
      under their own headings against the same input file, with the
      header's step count updating from "0 steps" to the real merged
      total.
- [x] (Stretch) Files tab (`debug_nav.go`), from a follow-up request:
      browse and switch to another `.crust` file alongside the one
      currently open, without leaving `crust develop` and relaunching
      it against a different path — the natural next thing to want on
      a directory full of `day01.crust`..`day25.crust`-style AoC
      solutions. Appended as a sixth tab (`tabNav`, after `tabRun`) so
      no existing tab's `const` value shifts under it; `↑↓` moves a
      cursor over `listCrustFiles`' alphabetically-sorted listing of
      every `.crust` file in the current file's own directory
      (including it, marked `(current)`, with the cursor starting
      there), enter switches. `switchToFile` reuses the exact "parsed
      but not yet run" starting point the whole session begins with
      (`emptyDebugView` — the same reason the interactive TUI never
      eagerly runs a file: touching real process stdin before
      bubbletea owns the keyboard would let a `store` recipe's
      `unbox()` silently eat a keystroke meant for the TUI), and
      reloads the target file's own remembered store/input/run-all
      settings (`debug_state.go`) rather than carrying over whatever
      the file being left had, matching what a fresh `crust develop
      otherday.crust` invocation would start with. A parse failure in
      the target file surfaces on the Files tab (`navErr`) without
      disturbing the still-valid current recording, the same
      "leave the last good state alone" choice the Editor tab's failed
      reload already makes. The listing is rescanned on every tab
      switch that lands on Files rather than cached for the session —
      directory contents can change between visits, and an
      `os.ReadDir` is cheap enough not to bother caching — which
      surfaced a real gap while wiring it up: reaching Files by
      tabbing *forward* from Run goes through `handleRunTabKey`
      (`debug_run.go`'s own separate key handler, not the shared
      `handleKey` switch), so that handler needed its own
      `maybeRefreshNav()` call too, caught by
      `TestTabFromRunReachesFilesTabAndRefreshesIt`. Verified with unit
      tests (100% coverage on every function in `debug_nav.go`) and a
      real pty-driven `crust develop` session: opened `day01.crust`
      alongside a `day02.crust`, confirmed the Files tab listed both
      with `day01.crust` marked current, moved the cursor down and hit
      enter, and confirmed the header switched to `day02.crust` and a
      return trip to the Files tab now marked *it* current instead.
- [x] `crust develop <file.crust>` auto-creates a missing target file
      instead of erroring out, on direct request: "if I `crust develop
      file.crust` where file.crust doesn't exist, [make] it create it
      and then enter the develop tool on that file" — starting a new
      AoC day's file is the single most common reason to point
      `develop` at a path that isn't there yet. `ensureFileExists`
      (`cmd/crust/debug.go`) runs first, ahead of even the saved-store
      lookup, using `O_CREATE|O_EXCL` so the existence check and the
      create are atomic; only ever creates the file itself, never a
      missing parent directory (a missing directory reads as a typo,
      not a scaffolding request), and notes the creation on stderr so
      it's never silent. From there `runDebug` proceeds exactly as it
      would for a file that already existed. Verified with unit tests
      and a real pty-driven `crust develop` session against a path
      that didn't exist: file created empty, TUI came up normally on
      the Time tab, and tabbing to Editor opened real `nvim` on the
      freshly-created file — see ARCHITECTURE.md's Phase 6 notes for
      why that pty check mattered (it caught an inaccurate doc comment
      claiming auto-create landed straight in the Editor tab).
- [x] The Files tab can create a new `.crust` file directly, on direct
      request: "can you make it so I can create a new crust file from
      the file tab in the develop tool?" Press `n` on the Files tab for
      a one-line name prompt (`.crust` appended automatically if
      omitted); enter creates the file (empty, `O_CREATE|O_EXCL` for
      the same atomic existence check `ensureFileExists` already uses)
      alongside the one currently open and switches straight to it,
      same as any other Files-tab switch. Unlike `ensureFileExists`, a
      name that already exists here is a reported error, not a silent
      no-op — this is an explicit "make something new" action, so
      switching to an existing file by habit would be a surprise, not
      a convenience. Esc cancels the prompt without quitting; Ctrl+C
      still quits the whole session, the same always-available hard
      exit the Run tab's own text field already keeps. Verified with
      unit tests and a real pty-driven session: pressed `n`, typed a
      name, hit enter, confirmed the file existed on disk (0 bytes)
      and the session had switched to it.
- [x] Fixed: the Run tab's entry-point selector sometimes silently
      failed to refresh after saving in the Editor tab, on direct
      request: "I'd like stores in the run tab to refresh when I exit
      from the editor." Root cause traced to a real, if intermittent,
      bug reproduced via a real pty session: `nvim` exiting cleanly
      (status 0) after a genuine `:wq`, but with `cmd.Run()` itself
      still returning a spurious `read /dev/stdin: resource temporarily
      unavailable` (EAGAIN) — `handleNvimExit` treats any non-nil err
      as a hard launch/exit failure and stops there, before reaching
      either the entry-point rescan or the save-triggered reload.
      Cause: `openEditorCmd`'s `exec.Command` left `Stdin` unset, so
      bubbletea's own `ExecProcess` filled it with its wrapped
      `p.input` (`stdinNoNamer` — deliberately not a real `*os.File`,
      for unrelated raw-mode/cancelreader reasons, see its own doc
      comment) — which forces Go's `os/exec` into an `os.Pipe`-plus-
      background-copy-goroutine path instead of handing `nvim` the fd
      directly, and it was that copier goroutine's own read of the
      real terminal racing `nvim`'s exit that produced the EAGAIN.
      Fixed by presetting `cmd.Stdin = os.Stdin` (a genuine `*os.File`)
      in a new `nvimCmd` helper, ahead of `tea.ExecProcess` — `os/exec`
      then dup's the fd straight into `nvim`, nothing left to race.
      Locked in with a direct Go test asserting `cmd.Stdin == os.Stdin`
      (not reachable through pty flakiness) plus the same real
      pty-driven session that first reproduced the bug, re-run and
      confirmed clean: `nvim` exits, no error shown, and the Run tab's
      selector picks up a newly-added `store_part1` recipe immediately.
- [x] `wrapReplace(list, start, end, value)` — the write counterpart
      to `wrap`/`wrapSlice`, on direct request: "can you make a
      function so I can assign to a circular list? ... something like
      wrapReplace(list, start, end, value) where value can shrink the
      list." Replaces the `wrapSlice(list, start, end)` span with
      `value`'s elements in place. Same length as the span: a pure
      position-wise write-back, `list[indices[k]] = value[k]`, even
      across a wrap — the case a knot-hash-style algorithm (pin a
      wrapping span, reverse it, repeat — AoC 2017 day 10) actually
      needs. Different length: no single position each new element
      belongs to, so the span is removed and `value` spliced in as one
      block at the span's first index instead (shrinks/grows `list`,
      an empty `value` deletes the span outright).
      **Bug found and fixed right after shipping**, reported directly
      with a worked example (`wrapReplace([2,1,0,3,4], 3, 6, [1,2,4,3])`
      should be `[4,3,0,1,2]`, not the `[0,1,2,4,3]` it actually
      produced): the first version used the block-splice rule for
      *every* length, which happens to coincide with position-wise
      assignment for a non-wrapping span — exactly what the original
      test suite covered — but silently reordered an untouched element
      the moment a span actually wrapped, since a wrapped span's
      positions aren't contiguous in `list`'s own order. Fixed by
      splitting same-length out into its own position-wise branch.
      Re-verified against the exact reported input/output, a real
      `crust run` reproduction, and — since the report came with a
      genuine AoC 2017 day 10 knot-hash implementation — that puzzle's
      own documented example (`3,4,1,5` on `[0..4]` → `12`), which now
      passes. Verified with table-driven Go tests (same-length in both
      a non-wrapped and a genuinely wrapped span, grow, shrink, delete,
      a wrapped different-length span, a full-wrap replace) and real
      `crust run` subprocesses reproducing both the original reversal
      example and the bug report end to end.
- [x] Bitwise builtins: `band(a, b)`, `bor(a, b)`, `bxor(a, b)`,
      `bnot(x)`, `shl(x, n)`, `shr(x, n)`, on direct request: "can you
      also create a xor and other bitwise functions for crust?"
      Integer-only, like `idiv`/`gcd`/`lcm`. `b`-prefixed as a family
      for consistency, even though `or` is the only one of
      `and`/`or`/`not` that's actually a reserved word (cRust's own
      logical AND/OR/NOT are the `with`/`or`/`hold` keywords, §4 —
      `and`/`not` were never claimed by anything, verified directly by
      declaring a variable named `and` and a recipe named `not` and
      running both). `shr` is arithmetic (sign-extending), not
      logical, since cRust has no unsigned Integer type to make a
      logical shift meaningful against; both shifts reject a negative
      count as a runtime error rather than letting Go's own `<<`/`>>`
      panic the whole process over it. Verified with table-driven Go
      tests and a real `crust run` subprocess exercising all six.
- [x] `rebox(element, fromBase, toBase)` — base conversion, on direct
      request: "can you make a base conversion std lib function with a
      fun pizza jargon name? something like fn(element, frombase,
      tobase)." Leans on the same "pizza box" metaphor `nobox` already
      established (§4): converting a number's base is repacking the
      same value into differently-sized boxes. `element` is always a
      String (never an Integer, even for `fromBase == 10`) and the
      result is always a String too (even for `toBase == 10`) — one
      predictable rule instead of a base-10 special case either
      direction. `fromBase`/`toBase` must be `2..36`; input digit
      letters accepted in either case, output always lowercase
      (matching Go's own `strconv.FormatInt` and, not incidentally,
      the lowercase hex AoC's knot-hash puzzles expect). Verified with
      table-driven Go tests (binary/hex/base-36 round trips, negative
      numbers, out-of-range bases, invalid digits for a base, wrong
      types) and a real `crust run` subprocess.
- [x] A Bench tab for `crust develop`, on direct request: "write a KPI
      for the develop tool that does x number of runs of the current
      settings of the run tab and shows a graph on the average
      runtime, the average memory, the max runtime and memory, the
      min memory and runtime, and any other stats that would be
      notable in the graph over those runs? Also make it so any of the
      lines on the graph can be enabled or disable[d] depending on
      what I'd like to compare." Type a run count, press enter, and
      the file being debugged runs that many times back to back with
      the Run tab's current entry point + input file — deliberately
      untraced (plain `runFile`, the same function `crust run` and the
      Run tab's own "run" already use), since `debugger.Recorder`'s
      per-statement tracing overhead would badly distort exactly the
      timing numbers this feature exists to measure. Wall-clock time
      (`time.Since`) and bytes allocated (`runtime.MemStats.TotalAlloc`
      deltas — cumulative, so unaffected by *when* a GC happens to run,
      the same technique `go test -bench -benchmem` itself uses) get
      charted as two small ASCII line charts, Runtime above Memory,
      each on its own Y-axis scale — the same reasoning that split the
      original KPI tab into Time and Memory tabs in the first place
      (per direct user feedback back then): overlaying nanoseconds and
      bytes on one shared axis would make neither legible. Five
      toggleable lines per chart — raw per-run values, average,
      median, max, min — one shared toggle set applied identically to
      both charts (`r`/`a`/`m`/`x`/`n` mnemonics, chosen specifically
      to avoid colliding with the numeric run-count field). Median
      included as the "other stat that would be notable" the request
      asked for — more resistant than the mean to one slow outlier run
      skewing the picture. A run count over 1000 is rejected outright:
      `benchCmd` runs synchronously inside one blocking `tea.Cmd` (the
      same shape "run all stores" already uses), and bubbletea can't
      process a quit keypress until that Cmd returns, so an accidental
      extra zero on the typed count would otherwise hang the whole
      session with no way out. A real pty-driven session caught a
      genuine bug in the first draft: with fewer runs than chart
      columns, the raw line only occupied its first N columns instead
      of spanning the chart the same full width the reference lines
      always do — fixed by resampling (linearly interpolating, not
      just bucket-averaging) the raw series across the full width
      regardless of how few runs there were. Verified with table-driven
      Go tests (the stats helpers, the resampling/downsampling logic,
      key handling, a real multi-run `benchCmd` execution against a
      real file on disk, error cases) and that same real pty-driven
      session, re-run clean after the fix: both charts rendered with
      real data, and toggling a series off visibly changed the chart.
      **Follow-up, on direct request**: "can you add values for the
      average and median lines?" — added the actual computed value to
      all four reference lines' legend entries (max/min too, not just
      the two named), which meant splitting the one shared legend
      below both charts into one legend per chart, each printed with
      that chart's own formatted value (`231.461µs` vs `55.0KiB` for
      the identical `average` series) — the single shared legend
      couldn't have shown a correct number for both metrics at once.
      Verified with a dedicated test pinning the exact expected
      average/median/max/min strings against known input, and the
      same real pty session confirming both legends render correctly
      under their own chart.
      **Follow-up, on direct request**: "is there a way for the bench
      tool that we could somehow navigate inside the individual graphs
      and look at the stats of individual runs?" — added inspect mode:
      `i` toggles it on, `h`/`l` step to the previous/next run, `g`/`G`
      jump to the first/last, and both charts highlight a caret under
      the column that run's own data landed on while a readout line
      below prints that run's exact duration, memory, and pass/fail —
      none of it blended through resampling the way the aggregate
      lines and even the raw line itself (bucketed or interpolated
      depending on run count vs. chart width) both are. The one real
      design problem: the chart's x-axis isn't 1:1 with run index once
      resampling kicks in, so a new `columnForRun` helper inverts
      whichever of the two mappings applied — bucket math when there
      are more runs than columns, the interpolation vertex formula
      when there are fewer — so the cursor always lands on the exact
      column the selected run's own value was actually drawn at.
      Verified with table-driven Go tests for `columnForRun` against
      all three cases (fewer/equal/more runs than width) and cursor-
      clamping edge cases, and a real pty-driven session exercising
      `i`/`h`/`l`/`g`/`G` end to end against live data.

## Phase 7 — Testing & Quality
- [x] Unit tests across lexer/parser/interpreter — not a separate
      one-time pass but grown incrementally alongside every language
      feature this whole project added (table-driven, input-in/
      AST-or-value-out, matching `lexer_test.go`/`parser_test.go`/
      `interpreter_test.go`'s established shape throughout). Coverage
      as of this writing: `internal/lexer` 99.4%, `internal/parser`
      93.0%, `internal/interpreter` 95.2%, `internal/ast` 100%.
- [x] Integration tests: sample `.crust` programs with expected output
      — `cmd/crust/examples_test.go`'s `TestRealExamplesProduceExpectedOutput`,
      deliberately the opposite of `TestRunFile`'s (main_test.go)
      synthetic-snippet dispatch/error-handling coverage: real shipped
      programs (all ten `examples/aoc2020/day01.crust`..`day05.crust`
      part1/part2 runs, plus the two pre-existing AoC-shaped examples)
      run through the actual `runFile` CLI pipeline, output pinned to
      values verified independently while building each program (the
      AoC ones against the puzzle's own documented example answers —
      see Phase 8's dry-run entry below). An initial, real slice, not
      exhaustive — the rest of `examples/*.crust` isn't pinned yet.
- [x] Benchmark against a real prior-year AoC puzzle for performance
      sanity — `cmd/crust/benchmark_test.go`'s `BenchmarkAoC2020Day1Part1`/
      `Part2`, run through the same real `runFile` CLI pipeline (not an
      isolated interpreter microbenchmark) against a deterministic
      (fixed-seed), puzzle-realistic-scale synthetic input: 200 distinct
      entries, the same size real AoC 2020 Day 1 personal inputs use —
      the puzzle's own documented example (6 entries) is far too small
      to be a meaningful stress test. Day 1 specifically, since its
      two solutions are the closest thing among `examples/aoc2020/` to
      a worst-case loop/index workload (part 1's O(n²) pair search,
      part 2's O(n³) triple search) — the one most likely to notice a
      regression in identifier lookup, index-expression evaluation, or
      loop overhead. The answer pair/triple is placed last in the
      generated input on purpose, so the early-exits-on-first-match
      nested search does close to its full work rather than getting
      lucky early. Baseline on this session's hardware: ~2.1ms/op
      (part1), ~9.0ms/op (part2) at `-benchtime=1s`.

## Phase 8 — AoC 2026 Ready
- [x] Confirm day-1 essentials all work end-to-end: file I/O, arithmetic,
      strings, loops, lists, maps. `examples/day1_essentials.crust`
      (+ `..._input.txt`) exercises every category in one real,
      shipped, permanently-tested program rather than a one-off manual
      check: `unbox()` off real stdin; `+ - * / %`, `idiv`, and `/`
      always widening to Float; string concatenation, `split`/`join`,
      `upper`/`lower`, `contains`; a counted `knead` with `flip`
      (continue), a for-each `knead` with `burnt` (break), and a `bake`
      while-loop; List literal/`push`/slicing/`map`/`filter`/`reduce`;
      Map literal/index/a missing key reading as `nobox`/`pop`. Wired
      into `TestRealExamplesProduceExpectedOutput`
      (cmd/crust/examples_test.go) with its exact real output pinned,
      the same "real program, real output, byte-for-byte" bar every
      other shipped example there already meets — this is a permanent
      regression check, not a one-time confirmation.
      Along the way, `TestFormatIsIdempotent` caught a real formatter
      bug this file happened to trip over on its very first draft: the
      three `deliver(map/filter/reduce(..., recipe(x) { ... }))` lines,
      written compact on one line each with no blank line between them
      (a perfectly ordinary way to pass a short callback), formatted
      once with no blank lines inserted (correct) but formatted *again*
      picked up a spurious blank line after each one.
      `internal/format`'s `lastLine`/`blankLineIfGap` mechanism decides
      whether to preserve a blank line between two statements by
      comparing an estimate of where the first one's source "ends"
      against the second one's own start line — but that estimate
      (`lastLine`) only accounted for a statement that is *itself* a
      block header (`order`/`knead`/`bake`), not a plain-looking
      `ExpressionStatement` that merely *contains* one, arbitrarily
      deep, via a `recipe(...) { ... }` argument — the one expression
      node with a block body of its own. The printer always expands a
      recipe body onto multiple lines regardless of how compact the
      source was, so on a second pass — now reformatting output where
      that expansion had already happened — the stale estimate
      undercounted how many lines the previous statement actually
      spanned, making the next statement look further away than it
      source-really was, i.e. "there must have been a blank line here."
      Fixed with `lastLineOfExpr`, a recursive walker mirroring
      `lastLine`'s own shape one level down into expressions — the same
      "how many lines does printing this actually take" question, just
      answered for `ast.Expression` instead of `ast.Statement`, checked
      at every place a `FunctionLiteral` could be hiding (a call's
      arguments, an infix/index operand, a collection literal's
      elements, a map's keys/values, ...). Regression-tested directly
      (`TestFormatIdempotentWithInlineFunctionLiteralArgument`,
      `TestFormatIdempotentWithFunctionLiteralNestedDeeper` covering a
      recipe literal buried in a List element and an infix operand, not
      just a call argument) in addition to being caught by the
      pre-existing glob-based `TestFormatIsIdempotent` the moment the
      new example file existed.
- [x] Per-day solution template (`examples/dayNN_template.crust`), on
      direct request, picked as the smaller of two remaining TODO items
      after the develop-tool ideas and the web playground were both
      done. A `store_part1`/`store_part2` skeleton, each already
      reading its input via `lines(unbox())` — the day-to-day AoC
      workflow (copy the template to `dayNN.crust`, save the puzzle's
      input alongside it, `crust develop`/`crust run`) lives in its own
      header comment rather than a separate doc page, so it's right
      there the moment the file is opened. Named `dayNN` (not a real
      day number, and not a `dayNN/` subdirectory some earlier phrasing
      of this item suggested) since it's one reusable starting point
      copied fresh for whichever day, not a real day's own solution —
      the existing `examples/aoc2020/dayNN.crust` files already cover
      what an actual solved day looks like. Deliberately not wired into
      `ensureFileExists`/`createNavFile` (`crust develop`'s auto-create
      and the Files tab's "new file"), which both stay tested and
      documented as creating an empty file — auto-injecting template
      boilerplate into every new file would be a real, surprising
      behavior change for anyone not expecting it, not just an
      additive one, so this stays an explicit `cp` a reader chooses to
      run. Referenced from README.md (the `crust day01.crust
      --store=part2` example) and docs/CHEATSHEET.md's opening
      paragraph. Verified for real: both entry points run cleanly
      against sample stdin through `crust run`, `crust parse` accepts
      it, and `crust fmt` reports it already canonically formatted.
- [x] Keyword cheat-sheet doc for quick reference during the event —
      `docs/CHEATSHEET.md`, one page: the keyword/operator tables,
      types/literals, assignment/unpacking (including the List-vs-Tuple
      "does the last target soak up the remainder" gotcha), entry
      points, every current builtin grouped by category, and a handful
      of idiom snippets (enumerate+unpack, memoization via `?:` on a
      missing-key-reads-as-nobox Map, input parsing, grid neighbor
      walks) — every snippet actually run through `crust run`/the REPL
      while writing this, not just transcribed from SPEC.md. Linked
      from README.md alongside SPEC.md/ARCHITECTURE.md.
- [x] Dry run: solve an old AoC day 1-5 in cRust before Dec 1, 2026 —
      AoC 2020 days 1-5 (`examples/aoc2020/day01.crust`..`day05.crust`,
      each with its own `store_part1`/`store_part2` and the puzzle's
      own small example input as a companion `.txt` file), verified
      against the documented example answers via a real built
      `crust run` subprocess for both parts of every day. Genuinely
      exercised real puzzle shapes the earlier "shape only" examples
      (`day01_find_pair.crust`) hadn't: multi-record text parsing
      (Day 4's blank-line-separated, multi-line passport batches),
      character-class-style validation with no regex engine (built
      from `chars`/`contains` directly), grid wraparound via modular
      string indexing (Day 3), and binary-space-partitioning decode
      loops (Day 5). Found and fixed two real language bugs along the
      way — see ARCHITECTURE.md's Phase 8 notes for both:
      - **A slicing off-by-one**: `s[3.<5]` on a 5-character string
        errored "index out of range" instead of returning the last two
        characters — `sliceIndices` required an exclusive end bound to
        be a real dereferenced index (`< n`) the same as an inclusive
        one, when it should allow `== n` (one past the end, never
        itself dereferenced walking forward).
      - **A parser precedence bug**: `s[0 .< slices(s) - 2]` (no
        parens around the compound end) misparsed as
        `s[0 .< slices(s)] - 2` instead of `s[0 .< (slices(s) - 2)]` —
        `parseRangeExpression` parsed its End at `SUM` precedence,
        which (correctly, for an *ordinary* infix operator's right
        operand) stops right before a trailing same-precedence "+"/"-"
        so an outer loop can pick it up — except a range's End has no
        such outer loop to hand it to. Fixed by parsing at `SUM-1`
        instead, letting End swallow its own full term.
      Both are exactly the kind of bug that a hand-written unit test
      suite tends to route around without ever hitting (every existing
      slice test happened to parenthesize a compound end bound) but a
      real, independently-authored program trips over immediately —
      the whole reason this checklist item exists. Both fixes shipped
      with dedicated regression tests at the parser and interpreter
      level, not just a fixed example.

- [x] Web playground (`crust bake playground`), on direct request
      ("start on those and the web playground") after a develop-tool
      brainstorm. Runs cRust entirely client-side via WebAssembly —
      `cmd/wasm` (a `js && wasm`-build-tagged `package main`) exposes a
      `crustRun(source, storeFlag, stdin)` global through `syscall/js`,
      built to `crust.wasm` by `scripts/build-wasm.sh` (which also
      copies the matching `wasm_exec.js` straight off the Go toolchain,
      so the runtime glue can never drift out of version with the
      compiled artifact) and checked into
      `cmd/crust/playground_assets/` alongside a static `index.html` UI
      so `go build ./...` keeps working out of the box for a fresh
      clone with no separate pre-build step. `cmd/crust/playground.go`
      embeds all three via `go:embed` and serves them at the site root
      through `http.FileServer(http.FS(...))`, mirroring
      `bake.go`/`internal/docsite`'s embedded-FS + local-HTTP-server +
      best-effort-browser-launch shape (including the same
      `runBakePlayground`/`servePlayground` split for ephemeral-port
      testability). The real design work was on the Go side, not the
      browser side: both `crust run` and the playground now share one
      `internal/runner.Run` pipeline (extracted verbatim from
      `run.go`'s `runFile`/`runEntryPoint`/`collectEntryPoints`/
      `storeFlags`/`reportRuntimeError`, which `debug.go` and
      `debug_view.go` also called directly and were repointed at the
      new package's exports) so the playground can't silently drift
      from actual CLI behavior on an edge case like "no default entry
      point, multiple named ones exist." `Run` takes a `msgPrefix`
      parameter instead of hardcoding `"crust run: "`, so `run.go`
      keeps its exact existing wording while the WASM side passes ""
      and reports bare `line:col: message` instead — the one
      intentional behavior difference, and it's cosmetic. One caveat
      that's inherent to the platform rather than a bug: `crustRun`
      runs synchronously on the browser's main thread (the standard
      `syscall/js` shape), so a program with an infinite loop hangs the
      tab until reloaded — same tradeoff as any other in-browser code
      sandbox that isn't running the interpreter in a Worker. Verified
      with Go tests (`internal/runner`'s own suite covering every
      error/success path byte-for-byte against the old inline
      behavior, plus `playground_test.go` mirroring `bake_test.go`'s
      port-parsing and real-HTTP-serving conventions) and, since this
      is the first genuinely browser-side feature in the project, real
      Playwright-driven verification against the pre-installed
      Chromium: loaded the served page, confirmed the WASM finished
      loading (`#status` flips to "ready"), ran the default program
      (`hello, pizza`), ran a `--store=part1` program reading real
      stdin, and ran a `1 / 0` program to confirm the runtime-error
      path reports `1:7: division by zero` with no path prefix — all
      three matched what the same source produces through `crust run`
      itself.
- [x] Live breakpoint / step-through debugging (`crust develop`'s new
      Live tab), on direct request: "go ahead" to the last of the
      original six develop-tool ideas, once the web playground was
      done. Genuinely *live*, unlike the Stepper tab's after-the-fact
      recording: `internal/debugger.LiveTracer` (`live.go`) is a
      `trace.Tracer` whose `Step` call *blocks* — on the interpreter's
      own goroutine, not the TUI's — at a breakpoint or in single-step
      mode, until the UI sends what to do next on its own `Resume`
      channel. "Paused" is not a UI flag tracked independently of
      reality; it *is* that goroutine sitting blocked inside `Step`.
      `LiveTracer` starts in single-step mode, so the very first
      traced statement always pauses — a live session should show you
      the first thing about to run, not silently execute an unknown
      amount of the program before you get a say. `RequestStop` (an
      `atomic.Bool` `Step` checks on *every* call, not only while
      paused) covers stopping a run mid-flight between breakpoints,
      which `LiveStop` sent on `Resume` alone can't reach — that only
      unblocks a `Step` call already waiting for it.
      `cmd/crust/debug_live.go` wires this into a new Live tab (`b`
      toggles a breakpoint on the source line under the cursor,
      persisted per file the same `develState` mechanism
      `benchBaseline` already uses; `r` runs/restarts with the Run
      tab's own currently-selected entry point and input file, the
      same settings-reuse the Bench tab already established; `s`/`c`/
      `x` step/continue/stop once paused) via `startLiveRun` (mirrors
      `buildDebugView`'s "parse, build an Interpreter, run it" shape,
      just with a pausable Tracer instead of a Recorder, on its own
      goroutine) and a `liveGen` counter stamped onto every message a
      run produces, so a stale pause/done from a run abandoned by
      switching files (debug_nav.go's `switchToFile`) can never land on
      whatever's current. Verified with table-driven Go tests under
      `-race` (breakpoint pausing and single-stepping, `Env` snapshot
      timing — a pause reports *after* that statement's own effect,
      the same "after the fact" semantics `trace.StepEvent.Env` already
      documents — `RequestStop` both mid-flight and while paused,
      `SetBreakpoints` copying its input rather than aliasing it) and a
      real pty-driven session: ran a small program under `crust
      develop`, single-stepped through three statements watching `x`/
      `y` populate in the variables panel, set a breakpoint further
      down, continued straight to it, confirmed the breakpoint set on
      an already-passed line has no further effect (breakpoints only
      apply going forward), and confirmed via `develop_state.json` that
      the breakpoint set on the first run was still there — and still
      took effect — on a completely separate `crust develop` launch
      afterward.
- [x] Richer stack traces, on direct request — the most impactful of
      the three remaining stretch goals: modules are low-payoff for
      AoC's mostly-single-file solutions, and a bytecode VM was
      explicitly deferred until a real profile says the tree-walker is
      the bottleneck (see ARCHITECTURE.md's Performance Strategy);
      this one pays off on every runtime error, with no new syntax and
      no architectural rewrite. `object.Error` gained `Frames []Frame`
      (`internal/object/error.go`) — one entry per recipe call the
      error unwound through, innermost first, appended by
      `internal/interpreter`'s `applyFunction` exactly at the point
      each call returns an Error, which needs no separate call-stack
      bookkeeping at all: it falls out of Go's own call stack
      unwinding for free (and handles recursion correctly for the same
      reason — each recursive level contributes its own frame simply
      by being its own nested `applyFunction` call). A single level of
      wrapping (the ordinary case: an error directly inside a
      `store()` entry point, nothing nested beneath it) stays exactly
      as terse as before — `Error.FrameLines()` only ever renders
      output for two or more frames, since a lone frame adds nothing
      the primary `path:line:col: message` line doesn't already say.
      Deep but finite recursion hitting an error at the bottom is
      capped at 12 printed frames (`maxErrorFrames`) with a "... and N
      more frame(s)" tail, the same bounded-output shape
      `DefaultMaxSteps`/`maxWatchLines`/every other capped panel in
      this codebase already uses — the raw `Frames` slice itself stays
      uncapped, only the rendering is bounded.
      The one real design constraint was performance: this had to cost
      *nothing* on the success path, however deep or recursive the
      call chain, since that's the actual hot path for a real puzzle
      run. `applyFunction` already had a `label` parameter for the
      debugger's frame display, eagerly computed only when `i.Trace`
      is set (an ordinary `crust run` never sets it) — reusing that
      same gate for stack traces wasn't an option, since traces have
      to work on *every* run, traced or not. Solved by passing the
      unevaluated call-site `*ast.Expression` through
      (`calleeExpr`) instead of its precomputed `.String()`, and only
      stringifying it — `frameName` — inside the one branch that's
      already on the rare error path. Verified with
      `BenchmarkUntraced` (`internal/interpreter`, a recursive `fib`)
      and `BenchmarkAoC2020Day1Part1`/`Part2` (`cmd/crust`) before and
      after: both landed within normal run-to-run noise of the
      documented baseline, confirming the zero-cost claim rather than
      just asserting it.
      Wired into every place a runtime error already surfaced as text
      — `internal/runner.reportRuntimeError` (`crust run`, and
      `crust develop`'s Run tab through it) and `crust repl`'s own
      separate error-printing path — with the primary line's existing
      format completely unchanged, so every pre-existing
      substring-matching test kept passing without modification.
      Verified with table-driven Go tests (`internal/object`'s
      `FrameLines` rendering and capping; `internal/interpreter`'s
      frame construction through nested calls, recursion, a builtin
      error picking up a frame only once it propagates through an
      *enclosing* recipe rather than getting one of its own, and
      `Call`/`CallNamed`'s own generic/real-name fallback;
      `internal/runner` and `cmd/crust/repl_test.go` confirming the
      call chain actually reaches stderr) and real subprocess runs —
      a three-level nested-call program, a single-level program
      (confirming no added noise), and a 20-level recursive one
      (confirming the 12-frame cap: 21 `boom()` calls + 1 `store()`
      call = 22 frames, 12 shown + "... and 10 more frame(s)").

## Stretch Goals
- [x] Module/import system — `delivery "path.crust"` (SPEC.md §10),
      from a direct request. Deliberately the simplest thing that
      could work: no namespacing, no exports list, no separate module
      value — a delivery statement evaluates the target file's top
      level directly into the *current* scope, the same as pasting its
      text in at that line, and it's fully transitive (delivering a
      file that itself delivers another brings both files' names
      along). Delivered exactly once even from multiple places, and
      safe against a circular delivery (terminates rather than
      recursing). New keyword `delivery` (`SPEC.md` §4 had reserved
      this exact working name in advance), `ast.DeliveryStatement`,
      `parser.parseDeliveryStatement`, `Interpreter.BaseDir` +
      `evalDeliveryStatement` (`internal/interpreter/delivery.go` —
      the first time that package has ever needed to import
      `internal/lexer`/`internal/parser` itself, previously always
      handed an already-parsed `*ast.Program`), wired into every real
      caller with a backing file (`internal/runner.Run`, `crust
      develop`'s traced run and Live tab) via `filepath.Dir(path)`.
      `internal/format`/`internal/lsp` both needed only a small, local
      addition — no structural changes, and completion/the docs site
      picked the new keyword up automatically since both already read
      `token.Keywords()` live rather than hand-maintaining a copy.
      `crust develop`'s tracer needed zero changes to work correctly
      through a delivered recipe, verified with a real `--plain` run —
      falls out for free from the tree-walker sharing one `Eval`/one
      `Trace` hook regardless of which file a statement came from, the
      same property that made richer stack traces free earlier.
      Verified across every layer (lexer/parser/interpreter unit
      tests, a real two-file example wired into the pinned-output
      integration test table, and real subprocess verification of
      `crust run`/`fmt`/`tokens`/`parse`/`develop` plus the missing-
      file and circular-delivery error paths). See
      docs/ARCHITECTURE.md's Phase 4 section for the full design,
      including the one honest limitation: a runtime error inside a
      recipe *defined* in a delivered file reports an accurate line
      number but under the *importing* file's own name, since nothing
      in this codebase's error pipeline tracks which physical file a
      token came from — fixing that fully would mean threading a
      source-file identity through every token, a bigger change than
      this feature's own scope justified.
- [ ] Bytecode VM instead of tree-walking (perf) — scoped, not started:
      [docs/BYTECODE_VM_SCOPING.md](./docs/BYTECODE_VM_SCOPING.md) is a
      feasibility study on direct request ("scope out exactly what
      that would entail... how feasible it is without breaking
      things"), covering what it'd actually take (two new packages,
      `internal/object` almost entirely reusable), the three places
      cRust's own design makes it harder than a tutorial VM (no
      `let`-style declare keyword, `crust develop`'s tracing being
      built on a tree-walker's own statement boundaries, `crust
      repl`'s persistent session needing persistent compiler state
      too), and a strictly-additive path (new packages only, an opt-in
      flag, the existing test/example suite as a shared correctness
      oracle between both backends) that never touches anything that
      exists unless/until it's proven both correct and actually
      faster. Still gated on profiling real AoC 2026 input first —
      no puzzle exists yet to profile against.
- [x] (Stretch) Puzzle input auto-fetch + solve timer, from a direct
      request ("the puzzle input and timer sounds interesting"),
      resolved via `AskUserQuestion` into three concrete choices: fetch
      triggered both by an explicit subcommand and by `crust develop`
      auto-fetching; the session cookie stored in a crust-managed
      config file; the timer shown live during a run and saved at the
      end. `internal/aoc` (new package): `session.go` (`LoadSession`/
      `SaveSession`, a private `0600` file under
      `$XDG_CONFIG_HOME/crust/session`, `$AOC_SESSION` env var checked
      first — same override convention as `NO_COLOR`), `fetch.go`
      (`Client.FetchInput(year, day)`, an overridable `BaseURL` so
      tests point at an `httptest.Server` instead of the real
      adventofcode.com — no test in this feature, at any layer, ever
      makes a real request to it, both because there's no legitimate
      session cookie to test with here and out of respect for the
      site's own "don't hammer this endpoint" etiquette; sets a
      descriptive `User-Agent` per that same etiquette), `timer.go`
      (`StartTimer`/`StopTimer`/`Elapsed`, a shared `timers.json` under
      the same config dir, one entry per `<year>/<day>`, read-mutate-
      write-whole-file the same way `debug_state.go` does). Three new
      `cmd/crust` subcommands: `crust login` (prompts for the cookie on
      stdin — no input masking, since `golang.org/x/term` isn't a
      dependency this project otherwise needs, and the prompt says so
      up front), `crust fetch <day> [--year Y] [--force] [--out path]`
      (refuses to overwrite an existing `dayNN_input.txt` without
      `--force`, starts the day's timer on success), `crust done <day>
      [--year Y]` (stops the timer, prints elapsed) — a separate
      command rather than auto-stopping on a clean run, since nothing
      here checks a run's output against AoC's actual accepted answer,
      so "ran without crashing" was judged not to mean "solved."
      `crust develop dayNN.crust` auto-fetches the same input (skipped
      silently the moment `dayNN_input.txt` already exists, or no
      session is saved at all) and its TUI header shows a live-ticking
      `⏱ day N: 3m12s (running)` for as long as that day's timer runs,
      via a new `tea.Tick`(1s)-driven redraw that reschedules itself
      only while the timer's actually running and stays off entirely
      for anyone not using the feature. Verified real end-to-end
      subprocess behavior (help text, `crust login`'s saved-cookie file
      permissions, the no-session error path, `$AOC_SESSION` override)
      and a real pty session showing the header's stopwatch genuinely
      ticking up once a second in a live terminal — plus that a file
      opened with no session configured never touches the config
      directory at all, confirming the feature is fully inert by
      default.
- [x] (Stretch) Eight stdlib additions — `sum(x)`, `reverse(x)`,
      `sortBy(list, fn)`, `any(iterable, fn)`, `all(iterable, fn)`,
      `findInts(s)`, `zip(a, b)`, `manhattan(a, b)` — from a "look into
      what might be missing from the stdlib" request, followed by "go
      ahead with those" once a prioritized gap list came back. Each
      reuses existing internal plumbing (`asElements`, `compareTwo`,
      `gridPos`, the `object.Hashable` check `enumerate`/`tuple`/`set`
      already established) rather than a new pattern per builtin — see
      docs/ARCHITECTURE.md's Phase 5 section for the two design calls
      worth knowing about: `sortBy` is stable (unlike `pizzasort`, on
      purpose — a key function can tie without the elements themselves
      being equal) and `findInts`'s `-`-as-sign rule specifically
      avoids misreading a range like `"1-3"` as `[1, -3]`. A
      priority-queue/heap type — the recurring "Dijkstra pathfinding"
      AoC category — was named as a gap but deliberately left out of
      this batch as its own, bigger open design question. Verified
      with table-driven Go tests per builtin and a real `crust run`
      subprocess exercising all eight together.
- [x] (Stretch) `crust develop` Live tab watch-panel scrolling and Run
      tab output scrolling, from "flesh out navigation for the live
      tab... look through the variable watcher" plus "add navigation
      functionality to the output of the run tab... scrollable-ness on
      the output." Live tab: `w` toggles focus onto the watch panel
      when it has more variables than fit, then ↑↓/j/k scroll it
      instead of moving the source cursor, with "N more above/below"
      hints and a live "(1-6 of 13)" position readout in the title.
      Run tab: output taller than the panel scrolls with pgup/pgdn one
      page at a time, with the same above/below hint convention, and
      always resets to the top on a fresh run. See
      docs/ARCHITECTURE.md's Phase 5 section for why each tab picked
      the specific keys it did. Verified with table-driven Go tests and
      real pty sessions driving `crust develop` end-to-end for both
      features.
- [x] (Stretch) `heapify(list)`/`heapPush(list, item)`/`heapPop(list)`/
      `heapPeek(list)` — the priority-queue/heap type flagged earlier
      as its own open design question, now closed out: no new object
      type, any List becomes a min-heap the moment heap ops are called
      on it (Python's `heapq` idiom). `heapPush(pq, (dist, node))`
      orders by `dist` first, `node` as a tiebreaker, via lexicographic
      Tuple comparison — the standard Dijkstra/A* shape. See
      docs/ARCHITECTURE.md's Phase 5 section for why it's List-mutation
      rather than a fifth collection type, and why the sift-up/down was
      hand-rolled instead of using Go's `container/heap`. Verified with
      table-driven Go tests and a real `crust run` subprocess running a
      small Dijkstra shortest-path solver end-to-end.
- [x] (Stretch) `crust new <day>` — stamps out `dayNN.crust` from the
      same `store_part1`/`store_part2` shape
      `examples/dayNN_template.crust` already documented for manual
      copying, day number filled in automatically, refusing to
      overwrite an existing file unless `--force` is passed. Shows up
      in `crust develop`'s own Nav tab with no code change needed
      there — it already lists every `.crust` file in the directory.
      See docs/ARCHITECTURE.md's Phase 6 section for why it's scoped
      to just `dayNN.crust` (no custom `--out`) and why the Nav tab's
      own blank-file "n" key was deliberately left alone. Verified
      with table-driven Go tests — including one that runs the
      freshly-stamped file through the real interpreter, not just
      checking its text — and a real built binary.
- [x] (Stretch) `crust submit <day> [answer] --part=<1|2>` — closes the
      loop `crust fetch`/`crust done` left open: submits an answer to
      adventofcode.com and classifies the response (correct, wrong —
      too low/too high, rate-limited, already solved, or unrecognized)
      via plain substring matching on the response page, no HTML-parser
      dependency needed. Reads the answer from stdin when omitted, so
      `crust run day06.crust --store=part1 | crust submit 6 --part=1`
      pipes straight through. Exits 0 only on a correct answer, for
      shell chaining. Deliberately never touches the fetch/done timer —
      see docs/ARCHITECTURE.md's Phase 6 section for why. Verified with
      table-driven Go tests at both the `internal/aoc` (response
      classification against `httptest`-served AoC-shaped HTML) and
      `cmd/crust` (flag parsing, stdin fallback, exit codes, dispatch
      wiring) layers — never against the real adventofcode.com, same
      posture `crust fetch`'s own tests already take.
- [x] (Stretch) `crust develop`'s Files tab shows "(used by N other
      files)" next to any file at least one other file's top-level
      `delivery "..."` targets — the easy way to spot which shared
      helper (`grid_utils.crust`, a parsing routine) is safe to edit
      without checking every day file by hand. Reuses the real lexer/
      parser rather than a text scan, resolves paths the exact same way
      the interpreter's own `delivery` evaluation does, and recomputes
      alongside the rest of the tab's listing on every visit rather
      than caching. See docs/ARCHITECTURE.md's Phase 6 section for the
      full design reasoning. Verified with table-driven Go tests and a
      real pty session against a small helper-file-plus-two-importers
      directory.
- [x] (Stretch) Bench tab two-entry-point diff (`c`) — captures the
      current batch (entry point, run count, average runtime/memory);
      switch the Run tab's entry-point selector and run a fresh batch
      on Bench to see both sides labeled side by side with the same
      signed-percentage diff `b`'s regression baseline already uses.
      Deliberately session-only, never persisted — unlike the
      baseline's own "track drift across sessions" job, this is for
      A/B-ing a brute-force `store_part2` against an optimized rewrite
      (or `part1` against `part2`) while both are still open right
      now. See docs/ARCHITECTURE.md's Phase 6 section for the full
      design reasoning. Verified with table-driven Go tests and a real
      pty session: a fast `store_part1` vs a deliberately slow
      `store_part2`, captured/switched/re-run through the actual TUI,
      confirming the rendered diff matched the two batches' real
      numbers.
- [x] (Stretch) `--year` on `crust develop`/`crust new` — `crust
      fetch`/`crust done`/`crust submit` already accepted `--year`,
      but `crust develop`'s own auto-fetch/timer/header stopwatch were
      still hardcoded to 2026, so a past-year puzzle's auto-fetch
      would silently reach for the wrong event. `--year` on `develop`
      (or `new`) is now remembered per file the moment it's given —
      set it once, every later invocation already knows it, no need
      to repeat the flag. The header now shows the year too
      (`⏱ day 7, 2020: 11s (running)`), since day numbers repeat
      across every AoC event. See docs/ARCHITECTURE.md's Phase 6
      section for the full design reasoning. Verified with
      table-driven Go tests across every layer this touches, plus
      real end-to-end verification against the actual built binary:
      `crust new 7 --year 2020`, then a real pty session driving
      `crust develop day07.crust` (no `--year` on that invocation)
      confirming the header correctly read 2020 from the remembered
      state alone.
