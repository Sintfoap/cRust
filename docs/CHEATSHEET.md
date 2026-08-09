# cRust Cheat Sheet

One page, for glancing at mid-puzzle. Full explanations live in
[SPEC.md](./SPEC.md); this is just the lookup table. `crust bake
documentation` serves a browsable version of the same material if you'd
rather have it in a tab. Starting a new day? Copy
[`examples/dayNN_template.crust`](../examples/dayNN_template.crust)
instead of a blank file.

## Keywords

| Keyword | Standard equivalent |
|---|---|
| `recipe` | function definition |
| `order` / `combo` / `special` | `if` / `else if` / `else` |
| `knead (i = 0; i < n; i++) { }` | counted `for` |
| `knead item in collection { }` | for-each `for` |
| `bake (cond) { }` | `while` |
| `burnt` | `break` |
| `flip` | `continue` |
| `serve [expr]` | `return [value]` |
| `stuffed` / `thin` | `true` / `false` |
| `nobox` | `nil` / `null` |
| `toppings{ }` | Set literal |
| `with` / `or` / `hold` | `&&` / `\|\|` / `!` |

## Operators

| | |
|---|---|
| `+  -  *  /  %` | arithmetic — `+` also concatenates String/List/Tuple; `/` always widens to Float; `%` is Integer-only |
| `..  .<` | range, inclusive/exclusive — `1..5` → `[1,2,3,4,5]`, `1.<5` → `[1,2,3,4]` |
| `==  !=  <  >  <=  >=` | comparison |
| `++  --` | increment/decrement, statement-only, no old/new-value distinction |
| `cond (\| then \|) else` | ternary — chains as an else-if ladder, right-associative |
| `a ?: b` | elvis — `a` unless it's `nobox`, else `b`; right-associative |
| `x[i]`, `x[a..b]`, `x[a.<b]` | index / slice (List, Tuple, String; negative `i` counts from the end; not valid on Set) |

## Types & literals

```
42            Integer          "text"        String
3.14          Float            stuffed/thin  Boolean
[1, 2, 3]     List (mutable)   nobox         Nil
(1, 2, 3)     Tuple (immutable, Hashable elements only)
{"a": 1}      Map              toppings{1,2} Set (unordered, unique)
recipe(x) { serve x }          Function (first-class, closes over scope)
```

Only `thin` and `nobox` are falsy — `0`, `0.0`, `""`, `[]`, `toppings{}`
are all truthy.

## Assignment & unpacking

```
x = 1; x += 1; x -= 1; x *= 2; x /= 2; x %= 2

first, rest = [1, 2, 3, 4]     // first = 1, rest = [2, 3, 4] (List RHS: last target soaks up everything left over)
a, b = (1, 2)                  // a = 1, b = 2                (Tuple RHS: exact arity, no soak-up — mismatched count is a runtime error)
```

## Entry points

```
recipe store() { ... }             // crust run day01.crust
recipe store_part1() { ... }       // crust run day01.crust --store=part1
recipe store_part2() { ... }       // crust run day01.crust --store=part2
```

Top-level code (outside any `store`-family recipe) always runs first,
same as Python module-level code.

## Builtins by category

**I/O** — `deliver(...)`, `unbox()` / `unbox(path)`, `lines(s)`

**Type conversion** — `str(x)`, `int(x)`, `float(x)`, `bool(x)`

**String** — `chars(s)`, `ints(s)`, `split(s)` / `split(s, delim)`,
`join(list, sep)`, `trim(s)`, `replace(s, old, new)`, `upper(s)`,
`lower(s)`, `find(s, sub)`, `contains(s, sub)`

**Collections (List/Tuple)** — `slices(x)`, `push(list, item)`,
`pop(list)` / `pop(list, i)` (remove+return last / by index, no
negative wrap — see `pop` in Map/Set below for the other two forms),
`copy(value)`, `min(...)` / `min(list)`, `max(...)` / `max(list)`,
`pizzasort(list)`, `combos(list, n)`, `enumerate(list)`,
`map(iterable, fn)`, `filter(iterable, fn)`, `reduce(iterable, fn, init)`,
`find(collection, value)`, `contains(collection, item)`,
`wrap(collection, i)` (circular index — mods past-the-end/negative `i`
back into range instead of erroring), `wrapSlice(collection, start, end)`
(circular slice — `wrapSlice([0,1,2,3], 3, 6)` is `[3, 0, 1, 2]`),
`wrapReplace(list, start, end, value)` (write counterpart to `wrapSlice`
— same length as the span: position-wise write-back, even across a
wrap; different length: spliced in as a block, shrinking/growing `list`
in place. `wrapReplace(xs, 3, 5, wrapSlice(xs, 5, 3))` reverses positions
`3..5` in place)

**Map** — `keys(m)`, `values(m)`, `freq(x)` (List/Tuple/Set → Map of
counts), `pop(map, key)` (remove+return) — a missing key reads as
`nobox`, not an error

**Set** — `gather(list)`, `set(x)`, `sprinkle(set, item)`,
`scrape(set, item)` (remove, no return), `pop(set, item)`
(remove+return, or `nobox` if absent), `topped(set, item)`,
`combine(a, b)` (union), `shared(a, b)` (intersection), `strip(a, b)`
(difference)

**Conversions between collections** — `list(x)`, `tuple(x)`, `set(x)`

**Grid** — `grid(s)`, `newGrid()`, `at(g, pos)`, `setAt(g, pos, value)`,
`gridBounds(g)`, `neighbors4(pos)`, `neighbors8(pos)` — `pos` is always
a `(row, col)` Tuple

**Math** — `idiv(a, b)` (floor div), `abs(x)`, `pow(base, exp)`,
`sqrt(x)`, `gcd(a, b)`, `lcm(a, b)`

**Bitwise** — `band(a, b)`, `bor(a, b)`, `bxor(a, b)`, `bnot(x)`,
`shl(x, n)`, `shr(x, n)` (arithmetic — sign-extending) — all
Integer-only; `b`-prefixed as a family for consistency (`or` is the
only one of `and`/`or`/`not` that's actually a reserved word — cRust's
own logical ops are spelled `with`/`or`/`hold`)

**Base conversion** — `rebox(element, fromBase, toBase)` (String in,
String out, bases `2..36` — `rebox("255", 10, 2)` is `"11111111"`)

No `reverse` builtin — use slicing instead: `xs[-1..0]` reverses a List
or String.

## Idioms

```
// Enumerate with unpacking
knead pair in enumerate(xs) {
    i, x = pair
    deliver(i, x)
}

// Memoization (a missing Map key is nobox, not an error)
cache = {}
recipe fib(n) {
    cache[n] = cache[n] ?: (n < 2 (| n |) fib(n - 1) + fib(n - 2))
    serve cache[n]
}

// Parse a whole input file of numbers, one per line
nums = ints(lines(unbox()))

// Split a "key: value" line
parts = split(line, ": ")

// Grid puzzle: parse once, walk with at()/neighbors4()
g = grid(unbox())
knead n in neighbors4((row, col)) {
    order (at(g, n) == "#") { ... }
}
```

## Runtime errors

```
crust run: day06.crust:14:5: division by zero
    in divide(...), called from 9:12
    in process(...), called from 3:5
    in store(...)
```

The failing line prints first, same as always; a call chain (`crust
run`, `crust develop`'s Run tab, and `crust repl` alike) follows
underneath once the error unwound through more than one recipe call —
innermost call first. A single level of wrapping (the failure directly
inside `store()`'s own body) prints just the one line, as before.

## Debugging while you work

```
crust develop day01.crust          # interactive: Time/Memory/Stepper/Editor/Run/Bench/Files/Live tabs
crust develop day01.crust --plain  # same recording, printed as text (pipeable)
crust repl                         # scratch REPL, persistent state across lines
crust fmt -w day01.crust           # canonical formatting, in place
crust bake playground              # in-browser sandbox, runs client-side via WASM
```

On the Stepper tab: `/query` searches the tree (`n`/`N` repeats it
forward/backward), `f`/`F` jump straight to the next/previous failed
step — both auto-expand any collapsed loop fold in the way. `v` toggles
a panel showing every variable visible at the selected step; `o`
toggles a panel showing that step's full, untruncated result.

On the Bench tab: `e` exports the current batch to `<file>.bench.csv`
(run, duration_ns, alloc_bytes, failed — one row per run). `b` saves
the current batch's average as a baseline; later batches show `vs
baseline: ...` as a signed percentage until `b` is pressed again.

On the Live tab: a genuinely live, real interpreter you step through
one statement at a time — not the Stepper's after-the-fact recording.
`b` toggles a breakpoint on the source line under the cursor
(persisted per file); `r` runs (or restarts) with the Run tab's
current entry point and input file; once paused, `s` steps one
statement, `c` continues to the next breakpoint (or the end), and `x`
stops the run early.

## Puzzle input + timer (optional)

```
crust login            # save your adventofcode.com session cookie ($AOC_SESSION also works)
crust fetch <day>      # download dayNN_input.txt, start its timer (--year, --force, --out)
crust done <day>       # stop the timer, print elapsed (--year)
```

Entirely opt-in: with no session saved, none of this ever runs.
`crust develop dayNN.crust` auto-fetches the same way (if
`dayNN_input.txt` isn't already there) and shows a live `⏱ day N:
3m12s (running)` in its header for as long as the timer's going.
