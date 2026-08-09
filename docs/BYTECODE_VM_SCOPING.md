# Bytecode VM — Scoping Study

Not a design doc for something being built; a feasibility study for a
decision not yet made. TODO.md's Stretch Goals lists "Bytecode VM
instead of tree-walking (perf)" — this is what that would actually
entail, written down before committing to it, per direct request
("scope out exactly what that would entail... how feasible it is
without breaking things"). Nothing here has been implemented.

## Where performance stands today

cRust's interpreter is a tree-walker: `internal/interpreter.Eval`
recursively switches on the AST node's Go type and evaluates it
directly, no compile step. ARCHITECTURE.md's Performance Strategy
section already lists this as the deliberate baseline, with a
bytecode VM explicitly deferred "until a profile says otherwise." The
two real numbers on record (`cmd/crust/benchmark_test.go`, this
session's hardware):

```
BenchmarkAoC2020Day1Part1   ~2.1ms/op   (nested-loop pair search over a real puzzle input)
BenchmarkAoC2020Day1Part2   ~9.0ms/op   (same shape, one more nesting level — triple search)
```

Neither number has ever been shown to actually bottleneck a real
puzzle. No AoC 2026 input exists yet to profile against — the earlier
"defer until profiled" call still holds, and this study doesn't
change that; it just answers "if we did decide to build it, what
would that cost and where would it hurt."

## What a bytecode VM means concretely for cRust

Two new packages, mirroring the shape this project already resembles
(`internal/lexer`/`internal/parser`/`internal/interpreter` reads like
the "Writing an Interpreter in Go" lineage; a VM is that book's own
sequel, "Writing a Compiler in Go" — same author, same style, a
well-trodden path, not exotic):

- **`internal/compiler`** — walks the *same* `internal/ast` tree
  `internal/interpreter.Eval` already walks, but instead of evaluating
  each node, emits bytecode instructions into a flat `[]byte` stream
  plus a constant pool. Structurally a mirror of `Eval`'s own switch:
  one `compile(node)` case per AST node type, same exhaustive list
  `Eval`'s switch already has (`internal/interpreter/interpreter.go`'s
  `case *ast.XxxStatement`/`case *ast.XxxExpression` block is close to
  a checklist for this).
- **`internal/vm`** — a stack machine that executes that bytecode: a
  value stack (`[]object.Object`), a call-frame stack (return address
  + local-variable base per active call, replacing what Go's own call
  stack does implicitly for `applyFunction` today), and a fetch-decode-
  execute loop.

### What's reused unchanged

`internal/object` barely changes. `Integer`, `Float`, `String`,
`Boolean`, `List`, `Map`, `Set`, `Tuple`, `Grid`, `Error`, `NULL` are
just Go values with an `Inspect()`/`Type()` — a VM pushes and pops the
exact same pointers a tree-walker would produce. `object.Equal`,
hashing, `DeepCopy` — all evaluation-strategy-agnostic, all reusable
as-is. `internal/lexer`, `internal/parser`, `internal/ast`,
`internal/format`, `internal/lsp` never execute code at all — a
bytecode VM changes nothing for any of them; `crust tokens`/`crust
parse`/`crust fmt`/`crust lsp` are completely unaffected regardless of
which execution backend `crust run` ends up using.

`internal/builtins.New(output, stdin, call Call)` — `Call func(fn
object.Object, args []object.Object) object.Object` — is already an
injected callback, not a direct call into the tree-walker's own
internals. A VM only needs to supply its own `Call` implementation
(push a synthetic frame, run the VM loop until that frame pops,
return the top of stack) — the ~40 builtin functions themselves
(`deliver`, `map`/`filter`/`reduce`, the whole Set/Grid/string
family) need zero changes.

### What's genuinely new work

- **`object.Function`** currently holds `Parameters []*Identifier,
  Body *ast.BlockStatement, Env *Environment` — an AST subtree plus a
  captured environment pointer. A VM needs a `CompiledFunction`
  (bytecode instructions, local-variable count, parameter count) and
  a `Closure` wrapping one with its captured free variables — a
  well-defined, contained change, but every place that currently
  builds/reads an `object.Function` needs to either dual-support both
  shapes or switch entirely.
- **Every AST node needs a `compile()` case.** cRust's grammar is
  considerably larger than Monkey's own tutorial language: ranges
  (`..`/`.<`), ternary/Elvis, `knead`/`bake` (three loop shapes, not
  one `while`), tuple-unpack assignment, compound assignment
  (`+=`/`-=`/...), `++`/`--`, slicing, `toppings{}` Set literals,
  `burnt`/`flip` (break/continue) needing jump-target backpatching
  inside loop compilation. None of these are hard individually, but
  there are a lot of them, and each is its own source of "did I get
  this one case right" risk — this is where most of the actual
  implementation time goes, not the VM's core loop.

## The three places cRust's own design makes this harder than the tutorial version

Worth naming explicitly, since these are exactly the spots a generic
"how to write a bytecode VM" writeup won't warn about — they're
specific to choices cRust already made.

### 1. Assignment has no declare keyword

SPEC.md §3's whole rule — `x = value` mutates `x` wherever it's
already bound in the scope chain, or creates it fresh in the
*innermost* scope if it isn't bound anywhere — is what
`object.Environment.Set` implements today with a runtime chain walk
(`assignExisting`, walking `outer` pointers). The classic bytecode-VM
win for a `let`-style language (Monkey has one) is compile-time slot
resolution: a `SymbolTable` assigns each local a fixed array index
during compilation, turning a variable access into O(1) indexing
instead of a runtime map lookup — normally a mechanical translation,
since `let` tells the compiler exactly when a *new* binding is being
introduced.

cRust has no such signal. The compiler has to *statically replay*
`Environment.Set`'s own decision — "does this name already resolve in
an enclosing scope reachable from here?" — using its own symbol table
at compile time, mirroring exactly which constructs open a new scope
today (`NewEnclosedEnvironment`: only a `recipe` call; `order`/
`knead`/`bake` reuse the enclosing scope, per `NewEnclosedEnvironment`'s
own doc comment) and which don't. Get this wrong in some corner case
and a program's *observable behavior* changes — a variable that used
to shadow now silently mutates an outer one, or vice versa — which is
exactly the kind of bug that a `go test` suite comparing "does the VM
produce the same output as the tree-walker for every example program"
would need to catch (more on that below).

### 2. `crust develop`'s entire debugger is built on a tree-walker's own step boundaries

`internal/trace.Tracer` (`Step`/`PushFrame`/`PopFrame`) hooks
`internal/interpreter`'s `evalTracedStatement`/`evalFramed` — it
fires once per *AST statement* and once per *call/loop-lap frame*,
because a tree-walker naturally visits the program in exactly those
units. `internal/debugger.Recorder` (the Stepper/Time/Memory tabs'
data source) and the brand-new `internal/debugger.LiveTracer`
(breakpoints — `Step` literally blocks mid-execution until the UI
says to continue) both depend on that same granularity.

A compiled bytecode stream has no "statement" boundaries left by the
time it runs — `x = a + b * c` might be six instructions, and a
`knead` loop is a handful of jump targets, not a distinct "frame"
object. Reproducing the Stepper's per-statement rows, the KPI tabs'
self-time-excluding-nested-calls accounting, and Live's mid-statement
pause-and-resume would need either (a) an instruction-to-source-line
map plus VM-level hooks reconstructing "which statement is this,"
meaningfully harder to get right than the current Tracer interface,
or (b) accepting that `crust develop` keeps using the tree-walker
*permanently*, and a VM only ever powers `crust run`'s (and possibly
`crust repl`'s) hot path. **(b) is the realistic answer** — this
project has invested more design and testing effort in `crust
develop` than in almost anything else it ships (six-plus tabs, real
pty-driven verification on every one), and reproducing that fidelity
against compiled bytecode is a project unto itself, not a byproduct of
building a VM. Keeping two execution strategies alive long-term,
each serving a purpose the other doesn't, is a normal design (real
language runtimes do this routinely — an optimizing/fast path plus a
separate introspection-friendly one), not a stepping-stone that
implies deleting the tree-walker later.

### 3. `crust repl`'s persistent session needs persistent compiler state, not just a persistent VM

Each REPL line is parsed and `Eval`'d fresh against the *same*
`*object.Environment` today — trivial, since `Environment` is just a
map that keeps accumulating bindings. A VM-backed REPL needs the
*compiler's* `SymbolTable` and constant pool to persist across lines
too, not just the VM's own global slots — otherwise a recipe defined
on line 3 wouldn't resolve when line 4 references it, since the
compiler compiling line 4 would have no record that it exists. This
is a solved problem (the same book's REPL chapter walks through
exactly this), but it's additional, cRust-repl-specific plumbing, not
something that falls out of the VM for free.

## Feasibility verdict

Yes, buildable, and well-precedented — this isn't research-grade
work, it's a known pattern this project's own existing architecture
already resembles half of. The realistic cost centers are:

- **Broad, not deep.** The VM's core loop and the compiler's
  structural shape are both genuinely small. The bulk of the effort is
  linear in cRust's grammar size: one correct `compile()` case per AST
  node, one correct opcode implementation per case, times roughly
  30-something distinct node types (vs. Monkey's dozen or so) — each
  individually easy, collectively a lot of surface area to get exactly
  right.
- **The scope-resolution rule (§1 above) is the one place a genuine
  design decision has to be made and gotten right**, not just
  translated mechanically from a tutorial.
- **`crust develop` is realistically out of scope for a VM to ever
  power** (§2 above) — this should be decided *up front*, not
  discovered halfway through, since it changes what "done" even means
  (feature parity with `crust run` only, not with everything `crust`
  does).

## How to do this without breaking anything that exists

The existing test suite is the biggest asset here, and the right way
to spend it: `internal/interpreter`'s table-driven tests (`testEval`-
style: input source string in, expected `object.Object`/output out)
and the shipped `examples/*.crust` files (each with pinned expected
`deliver()` output, `cmd/crust/examples_test.go`) are both
*evaluation-strategy-agnostic by construction* — they assert on
observable behavior, never on `internal/interpreter`'s own internals.
If `internal/vm` exposes a comparable `Run(program) (output string,
result object.Object)` entry point, the exact same test *data* — not
new test code, the same tables — can run against both backends. That
turns "does the VM actually behave identically to the tree-walker"
from a hand-verification problem into an automated one, for free,
using assets that already exist.

Concretely, a non-breaking path looks like:

1. Build `internal/compiler` + `internal/vm` as strictly new,
   additive packages. Touch nothing in `internal/interpreter`,
   `cmd/crust`, or `crust develop` while building them.
2. Add a hidden opt-in (a `--vm` flag on `crust run`, or an env var —
   not a documented, committed-to interface yet) that runs the VM path
   instead of the tree-walker, purely for A/B testing. Default
   behavior for every existing command stays exactly what it is today.
3. Run the shared test/example corpus through both backends and
   require byte-for-byte agreement before trusting the VM on anything
   real — including the AoC 2020 dry-run days and
   `examples/day1_essentials.crust`, both of which already exercise a
   real breadth of the language end to end.
4. Only then benchmark the VM against `BenchmarkAoC2020Day1Part1`/
   `Part2` and `BenchmarkUntraced`'s recursive `fib` — if it isn't
   meaningfully faster on real AoC-shaped work (not just a microbench),
   there's no reason to promote it further, regardless of how correct
   it is.
5. Explicitly decide `crust develop`'s fate (§2) before, not after,
   calling this "done" — either it stays on the tree-walker forever
   (recommended), or someone signs up for the separate, harder problem
   of instruction-level tracing.
6. `crust run`'s *default* only ever changes if 3 and 4 both pass —
   until then the VM is available, tested, and inert.

## What would actually justify starting this

Real AoC 2026 puzzle input, and a `crust develop`-produced profile
(the Time tab already shows self-time per function/loop) pointing at
the interpreter loop itself rather than at algorithm choice — which,
for most AoC puzzles, is where the real slowdown usually is regardless
of language. That data doesn't exist yet. This document is what
"go" would actually cost once it does.
