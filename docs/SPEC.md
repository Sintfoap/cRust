# cRust Language Specification

This is the source of truth for cRust syntax and semantics. The parser's
structure should map onto the grammar here; when the two disagree, update
both in the same change.

## 1. Design Summary

- Dynamically typed, evaluated by a tree-walking interpreter (see
  [ARCHITECTURE.md](./ARCHITECTURE.md)).
- Brace-delimited blocks; statements end at a newline or `;`.
- No `let`/`const` — variables come into existence by assigning to them,
  Python-style. See [§3](#3-variables--assignment).
- Every structural keyword is pizza jargon — see
  [§4](#4-keyword-vocabulary).
- Unpacking assignment, integer ranges (`..` / `.<`), `++`/`--`, a
  ternary conditional (`(|`/`|)`), and an Elvis nil-coalescing operator
  (`?:`) round out the ergonomics that matter most for tight AoC loops
  — see [§3.1](#31-unpacking-assignment) and [§5](#5-operators).

## 2. Core Types

| Type | Example | Notes |
|---|---|---|
| Integer | `42`, `-7` | 64-bit signed |
| Float | `3.14`, `-0.5` | 64-bit |
| String | `"pepperoni"` | double-quoted, UTF-8, single-line (see escapes below) |
| Boolean | `stuffed`, `thin` | see §4 — no bare `true`/`false` |
| Nil | `nobox` | absence of a value |
| List | `[1, 2, 3]` | 0-indexed, ordered, heterogeneous, mutable |
| **Tuple** | `(1, 2, 3)` | 0-indexed, ordered, heterogeneous, **immutable** — see §2.3 |
| Map | `{"a": 1, "b": 2}` | Hashable keys (String/Integer/Float/Boolean/Tuple), mutable |
| **Set** | `toppings{1, 2, 3}` | unordered, unique — a pizza's toppings never repeat and don't have an order, so that's the name; see §2.2 |
| **Grid** | `grid(s)` / `newGrid()` | mutable 2D structure, `(row, col)` coordinates can go negative and grow via `setAt`; no literal syntax, not directly indexable/iterable — see §2.4 |
| Function | `recipe(a, b) { ... }` | first-class, closes over defining scope |

### 2.1 Strings and Comments

String literals are single-line and support a small set of backslash
escapes — enough for AoC-style text processing, not a general escaping
scheme:

| Escape | Meaning |
|---|---|
| `\"` | literal double quote |
| `\\` | literal backslash |
| `\n` | newline |
| `\t` | tab |
| `\r` | carriage return |

A raw (unescaped) newline inside a string, or the string running to
EOF without a closing `"`, is a lexer error (`unterminated string
literal`) rather than silently starting a multi-line string. Any other
`\x` is an error too (`unknown escape sequence \x`) rather than
passing the backslash through literally — an unrecognized escape is
much more likely a typo than an intentional literal backslash.

Comments start with `//` and run to end of line — used throughout
every example in this doc and under [`examples/`](../examples), even
though (correctly) there's no `comment` production in §8's grammar:
comments are stripped by the lexer before the parser ever sees a
token, the same way whitespace is. There's no block-comment syntax.

### 2.2 Sets (`toppings{...}`)

A Set holds unique values with no defined order — like the toppings on a
pizza: you don't have "pepperoni" listed twice, and it doesn't matter
what order they were added in.

```
favorites = toppings{"pepperoni", "mushroom", "olive"}
empty     = toppings{}
```

- Literal syntax is the `toppings` keyword directly followed by a brace
  list — this is what disambiguates a Set literal from a Map literal
  (bare `{...}`) at parse time; see the grammar in §8.
- Sets are **not indexable** (`s[0]` is a parse/runtime error — there's no
  "first" element). Use the builtins in §7, or iterate with a for-each
  `knead` loop (§8, `forEachHeader`).
- Equality between two Sets compares contents, ignoring order:
  `toppings{1, 2} == toppings{2, 1}` is `stuffed`.

### 2.3 Tuples (`(...)`)

A Tuple is a fixed-size, ordered, **immutable** sequence — List's
counterpart for values that shouldn't change after they're built, most
usefully because immutability is what makes a Tuple safe to *hash*: it
can be a `Map` key or `Set` element the way a `List` never safely could
(a `List`'s contents can change after it's inserted, which would leave
a stale hash behind; a `Tuple`'s can't).

```
point = (3, 4)
deliver(point[0])          // 3 -- indexable, like a List

seen = toppings{}
sprinkle(seen, (3, 4))     // works: Tuple is hashable
sprinkle(seen, [3, 4])     // error: List is not hashable

(1, 2) == (1, 2)           // stuffed -- contents, in order, like List
(1, 2) == [1, 2]           // thin -- different types never compare equal

(1, 2) + (3, 4)            // (1, 2, 3, 4) -- concatenation, a new Tuple (§5/§6)
```

- Written with the same `(`/`)` a grouped expression uses — `(x)` (no
  comma) is still ordinary grouping, unchanged; two or more
  comma-separated values inside the parens is what makes it a Tuple.
  There's no dedicated bracket pair the way `[...]`/`toppings{...}`
  each get one, since the parser only needs one token of lookahead
  (comma vs. `)`) to tell a Tuple from a grouped expression — see §8's
  grammar note on `tupleLiteral`.
- **Not mutable**: `t[0] = x` is a runtime error
  (`Tuple is immutable, does not support index assignment`). This is
  what makes hashing safe (above), not just a style preference.
- Indexable (`t[0]`) and iterable (`knead x in myTuple {...}`) exactly
  like a List; `slices(t)` works the same way too.
- **Every element must itself be hashable** — Integer, Float, String,
  Boolean, or another all-hashable Tuple. A List, Map, or Set element
  is a runtime error at the point the Tuple literal is evaluated
  (`unhashable type: ... cannot be a Tuple element`), the same
  "checked at construction" approach §7's `gather`/Set-literal already
  take for their own elements. This is what guarantees a Tuple's
  `HashKey` never has to handle an element that can't produce one.
- Unpacking assignment (§3.1) treats a Tuple specially: **exact arity**
  is required, and every target — including the last — gets its own
  bare value, unlike unpacking a List (where the last target always
  gets wrapped in a `List` of everything left over). Which rule applies
  is decided by the right-hand side's *runtime* type, so it works
  through any expression, not just a literal directly on the
  assignment's right — a ternary choosing between two Tuples, a
  function call returning one, anything:
  ```
  a, b = (1, 2)              // a = 1, b = 2 -- both bare
  a, b = [1, 2]               // a = 1, b = [2] -- List's own rule, unchanged

  x, y = (y, x)                // swaps correctly

  recipe minMax(a, b) {
      serve a < b (| (a, b) |) (b, a)
  }
  lo, hi = minMax(9, 3)        // lo = 3, hi = 9 -- unpacks the ternary's Tuple result
  ```

### 2.4 Grids (`grid(s)` / `newGrid()`)

A Grid is a mutable 2D structure whose `(row, col)` coordinates can be
negative and grow in any direction — `setAt(g, pos, value)` expands `g`
to include `pos` if it's currently out of range, instead of erroring,
which is what a plain List of Lists could never safely do (a List has
no room to remember "row 0 currently means logical row -3" between
calls — see §7's `setAt` for why that persistent offset is exactly the
reason Grid is its own type rather than nested Lists):

```
g = grid("ab\ncd")            // parse text: rows/cols start at (0, 0)
deliver(at(g, (0, 0)))        // "a"

setAt(g, (5, 5), "X")         // out of range -- g grows to include it
deliver(at(g, (5, 5)))        // "X"
deliver(gridBounds(g))        // (0, 0, 5, 5)

setAt(g, (-2, -2), "Y")       // negative -- g grows the other way too
deliver(at(g, (0, 0)))        // still "a" -- prior coordinates unaffected
deliver(gridBounds(g))        // (-2, -2, 5, 5)
```

- There's no Grid *literal* syntax — a Grid always comes from `grid(s)`
  (parsing text, one character per cell, at offset `(0, 0)`) or
  `newGrid()` (empty, for building one up entirely through `setAt`,
  e.g. a simulation that starts from a handful of live cells rather
  than a fixed-size input).
- **Not directly indexable or iterable** — `g[0]` and
  `knead row in g {...}` are both errors, unlike List/Tuple. This is
  deliberate, not an oversight: once a Grid's origin can shift, a raw
  index would either have to mean "position 0 in the backing storage"
  (which silently changes meaning after any expansion, wrong) or
  duplicate the same offset math `at`/`setAt` already do (redundant).
  `at(g, pos)`, `setAt(g, pos, value)`, and `gridBounds(g)` — the
  latter for learning where a Grid's edges currently are — are the
  whole interface.
- `at`/`setAt` take positions as `(row, col)` Tuples, same as
  `neighbors4`/`neighbors8` produce and consume — see §7.
- `==`/`!=` compare a Grid's full logical layout: same bounds *and* the
  same cell at every coordinate. A Grid with identical visible content
  but a different origin (e.g. one that was expanded negatively and one
  that wasn't) is **not** equal — the same coordinate would mean a
  different cell on each.
- Not `Hashable` (mutable, like List/Map/Set) — can't be a `Map` key or
  `Set` element.

## 3. Variables & Assignment

There is no `let`, `var`, or `const`. A variable comes into existence the
first time you assign to it:

```
count = 0
name = "pepperoni"
```

**Scoping rule**: assigning to `x` looks for an existing binding of `x`
starting in the current scope and walking outward through enclosing
`recipe` scopes to the global scope.
- If found, that existing binding is updated in place.
- If not found anywhere, a new binding is created in the *current* scope
  (the innermost enclosing `recipe`, or global if not inside one).

Only `recipe` calls introduce a new scope — `order`/`combo`/`special`,
`knead`, and `bake` blocks do **not** (this mirrors Python, where
`if`/`for`/`while` aren't scopes either). That's what makes ordinary
accumulator patterns work with zero ceremony:

```
recipe sum(nums) {
    total = 0
    knead i in nums {
        total = total + i   // updates the `total` above, not a new local
    }
    serve total
}
```

Because assignment just walks up and mutates whatever it finds, a
`recipe` can also update a variable captured from an enclosing scope
directly — no `global`/`nonlocal` keyword needed:

```
tally = 0
recipe bump() {
    tally = tally + 1   // mutates the outer `tally`
}
```

**Parameters are the one exception to the walk-up rule**: a `recipe`'s
parameters are always fresh bindings in that call's own new scope, even
if a variable of the same name already exists somewhere outward (the
global scope, most commonly). A parameter shadows; it never aliases:

```
g = 1
recipe touch(g) {
    g = 999   // reassigns the *parameter* g, not the outer one
}
touch(g)
g   // still 1
```

If parameters followed the same walk-up rule ordinary assignment does,
calling a function would risk silently overwriting any same-named
variable in every enclosing scope up to global — the opposite of what
"pass an argument" should ever mean.

Compound assignment (`+=`, `-=`, `*=`, `/=`, `%=`) follows the same rule
and is sugar for `x = x OP expr`. See §5 for the full operator list.

An assignment target (an *lvalue*) is either a bare identifier or an
index expression: `x = 1`, `list[0] = 1`, `map["key"] = 1`. Anything else
on the left of `=` is a parse error.

### 3.1 Unpacking Assignment

Two or more comma-separated identifiers on the left of `=` unpack a List
on the right: each target but the last takes one element positionally,
and the **last target always takes a List of everything left over** —
even if that's a single item, or none:

```
entries = [1, 2, 3, 4]
first, rest = entries
// first = 1
// rest  = [2, 3, 4]

a, b = [1, 2]
// a = 1
// b = [2]        <- still a List, not the bare value 2

a, b, c = [1, 2, 3, 4, 5]
// a = 1, b = 2, c = [3, 4, 5]
```

The last target is *always* a List, regardless of how many items land in
it — this is deliberate: code that reads `rest` shouldn't have to guard
against it sometimes being a bare scalar depending on the input's
length. If you specifically want the last *element* rather than the
list of everything after the first, index it: `list[slices(list) - 1]`.

Rules:
- The right-hand side must be a List (unpacking a Set, Map, or String
  directly is a parse/runtime error — Sets have no defined order to
  unpack by, and Maps/Strings would be ambiguous about what "an element"
  means here).
- Needs at least `N - 1` items for `N` targets (the last target is
  allowed to end up empty); fewer than that is a runtime error.
- Unpacking targets must be bare identifiers, not index expressions —
  `list[0], list[1] = pair` isn't supported. Use ordinary index
  assignment twice instead.
- Only plain `=` works with unpacking; there's no unpacking form of
  `+=` etc. — that wouldn't have a sensible meaning.

Unpacking a Tuple (§2.3) instead of a List follows a different rule —
exact arity, every target bare, no last-gets-a-list wrapping — see
§2.3 for the full explanation and examples; that section is the source
of truth for Tuple's own behavior, this one for List's.

For-each `knead` loops don't support multiple loop variables yet (e.g.
unpacking `[key, value]` pairs while iterating a Map). That's a natural
next question once this lands, but it's deliberately left open for now
rather than guessed at.

## 4. Keyword Vocabulary

The whole language is framed as running a pizza kitchen: a `recipe` is a
procedure, an `order` either gets made or falls through to today's
`special`, and a `toppings` set is exactly what it sounds like.

| Keyword | Standard equivalent | Why |
|---|---|---|
| `recipe` | `func` — function definition | a reusable procedure for making something |
| `order` | `if` | an order comes in; it's handled if the condition holds |
| `combo` | `else if` | a chained alternative order |
| `special` | `else` | today's special — the fallback branch |
| `knead` | `for` (counted *and* for-each loop) | repetitive, bounded action, like kneading dough |
| `in` | (for-each clause) | `knead item in collection` — plain English reads clearest here |
| `bake` | `while` (conditional loop) | keep going while the condition holds — "bake until done" |
| `burnt` | `break` | pull it out of the oven early |
| `flip` | `continue` | flip to the next iteration, skip the rest of this pass |
| `serve` | `return` | hand back the finished result |
| `stuffed` | `true` | crust is stuffed — full/true |
| `thin` | `false` | thin crust — empty/false |
| `nobox` | `nil` / `null` | an empty pizza box — nothing inside |
| `toppings` | Set literal/type | a pizza's toppings: no duplicates, no order — see §2.2 |
| `with` | `&&` / logical AND | "pepperoni **with** mushrooms" |
| `or` | `\|\|` / logical OR | plain English reads fine here, no jargon needed |
| `hold` | `!` / logical NOT | "**hold** the onions" |

`topping` (singular) and `sauce` no longer exist as keywords — cRust
dropped the `let`/`const` distinction entirely (§3). `sauce` wasn't
wasted, though: it's now a standard-library function (§7) — a base layer
under a value that might be `nobox`.

Everything that isn't in this table (`deliver`, `slices`, `sauce`,
`gather`, `sprinkle`, ...) is a **builtin function**, not a keyword — see
§7. That distinction matters mechanically: keywords are reserved words
the lexer always recognizes, so they can never be shadowed or used as a
variable/function name; builtins are just names predeclared in the
global scope, so user code is free to redefine them if it wants to.

Reserved for later phases, not yet implemented: a module-import keyword
(working name `delivery`) for Phase 5+ if a module system gets built
(see TODO.md stretch goals).

## 5. Operators

| Category | Operators | Notes |
|---|---|---|
| Arithmetic | `+  -  *  /  %` | `+` also concatenates strings, Lists, and Tuples (same-type only, always producing a new value); `/` always true-divides to a Float (§6); `%` requires two Integers |
| Range | `..  .<` | Integer-only, produces a List — see §5.1 |
| Unary | `-x`, `hold x` | numeric negate, logical not |
| Comparison | `==  !=  <  >  <=  >=` | see §6 for cross-type rules |
| Logical | `with` (and), `or` (or), `hold` (not) | keyword operators, not symbols |
| Assignment | `=  +=  -=  *=  /=  %=` | statement-level only (§3), not usable as a sub-expression — like Python's `=`, unlike C's |
| Increment/decrement | `++  --` | either side of the target (`i++` and `++i` are identical); statement-level only, no return value — see §5.2 |
| Ternary | `(\|`, `\|)` | `cond (\| then \|) else` — the two half-pizza glyphs as a matched pair, one job each — see §5.3 |
| Elvis / nil-coalesce | `?:` | `a ?: b` — a if it isn't `nobox`, else `b` — see §5.4 |
| Indexing | `x[i]` | List/Tuple (by position), Map (by key), String (by position); **not** valid on Set |
| Grouping | `( )` | expression grouping |

Set union/intersection/difference are deliberately **not** operators —
overloading `+`/`&`/`-` with a second meaning for one type isn't worth
the ambiguity. They're builtins instead: `combine`, `shared`, `strip`
(§7).

Precedence, low to high (unchanged shape, assignment/increment sit
outside this ladder as statement forms):

```
ternary (| |)  <  elvis (?:)  <  or  <  with  <  equality (== !=)
    <  comparison (< > <= >=)  <  range (.. .<)  <  sum (+ -)
    <  product (* / %)  <  unary (- hold)  <  call/index
```

This maps directly onto the Pratt-parser dispatch tables in
[ARCHITECTURE.md](./ARCHITECTURE.md#phase-3--parser-internalparser).

### 5.1 Ranges (`..`, `.<`)

```
1..5    // [1, 2, 3, 4, 5]   inclusive of both ends
1.<5    // [1, 2, 3, 4]      exclusive of the upper end
```

- Both operands must be Integers — a Float on either side is a runtime
  error (`range bounds must be Integers`). This is a deliberate
  restriction, not a current limitation: a float-stepped range invites
  off-by-epsilon bugs for no real AoC benefit.
- The result is an ordinary, immediately-materialized **List**, exactly
  as if you'd written it out by hand — not a lazy sequence. `slices(1..5)
  == 5`, `(1..5)[0] == 1`.
- If the start is greater than the end, the result is an **empty List**
  — there's no implicit reversal. Want it descending? Reverse the List
  explicitly (a `reverse`-style builtin lands in Phase 5).
- Range doesn't chain: `1..5..10` is a parse error, not
  `(1..5)..10` silently doing something odd — the grammar only allows
  one range operator per expression (§8).
- Binds tighter than comparison, looser than `+`/`-`:  `1..5 == [1,2,3,4,5]`
  parses as `(1..5) == [1,2,3,4,5]`, and `1+1..5+1` parses as
  `(1+1)..(5+1)`.

### 5.2 Increment/Decrement (`++`, `--`)

```
i++
++i     // identical to the line above
i--
--i     // identical to the line above
```

- `i++`, `++i`, and `i += 1` all do exactly the same thing: increment
  `i` by 1 in place. Neither form "returns" a value — unlike C, there's
  no prefix/postfix distinction (no old-value-vs-new-value footgun),
  because `++`/`--` are statements, not expressions, just like `=`
  (§3). `x = i++` is a parse error, not a way to read the old value.
  `--` is the symmetric decrement companion to the requested `++`.
- The target follows the same lvalue rule as assignment: a bare
  identifier or an index expression (`arr[i]++` is valid).
- Only defined for Integer and Float targets; anything else is a
  runtime error.
- Reads naturally in a `knead` counted-loop post-clause:
  `knead (i = 0; i < n; i++) { ... }`.

### 5.3 Ternary Conditional (`(|`, `|)`)

The two half-pizza glyphs turned out to fit a genuinely two-part
structure — a conditional with a "then" and an "else" — much better
than they fit a single fallback value, so that's what they are: a
matched pair forming one ternary expression, `cond (| then |) else`.

```
label = (n % 2 == 0) (| "even" |) "odd"
deliver(label)

// chains read like an else-if ladder — else-branch is right-associative
grade = (score >= 90) (| "A" |)
        (score >= 80) (| "B" |)
        (score >= 70) (| "C" |)
        "F"
```

- `cond` follows the exact same truthiness rule as `order`/`bake`
  (§6) — only `thin`/`nobox` are falsy, everything else picks the
  `then` branch. There's no special-cased "is this nobox" check here
  (that's Elvis, §5.4); ternary is a plain condition, same as an `if`.
- Exactly one of `then`/`else` is evaluated, never both — this is a
  real branch, not two eager expressions with one discarded.
- `then` (between `(|` and `|)`) can be any expression, including a
  nested ternary — the two distinct delimiter tokens bound it
  unambiguously, so no extra parens are needed there. `else` (after
  `|)`) is right-associative, which is what makes the chained
  else-if-ladder example above work without explicit grouping.
- `cond` itself is parsed one precedence level below `ternary`, so a
  bare ternary used *as* a condition needs parens:
  `(a (| b |) c) (| d |) e`.
- This is an **expression**, unlike `order`/`combo`/`special` (§4),
  which are statements. Use ternary for a single value pick
  (`x = cond (| a |) b`); use `order`/`special` when either branch
  needs to run more than one statement.

### 5.4 Elvis / Nil-Coalescing (`?:`)

```
name = maybeNil ?: "default"      // "default" only if maybeNil is nobox
value = a ?: b ?: c ?: "fallback" // chainable, right-associative
```

- `a ?: b` evaluates `a`; if it isn't `nobox`, that's the result and
  `b` is never evaluated. Only when `a` *is* `nobox` does it evaluate
  and return `b`. This is exactly `sauce(a, b)` (§7) as an operator —
  same short-circuiting, same nobox-only check, pick whichever reads
  better at the call site.
- **Tests for `nobox` specifically, not falsiness** — `thin ?:
  "fallback"` stays `thin`, and `0 ?: 99` stays `0`. A real value isn't
  an absence, so it's never replaced. Same principle as §6's
  truthiness rule (`0` isn't falsy), applied to a different operator.
- Sits just above ternary in precedence (§5's ladder) — below `or`, so
  `a with b ?: c` groups as `a with (b ?: c)`.
- Unlike ternary, `?:` involves no literal parens in its token, so it
  doesn't have the bracket-matching quirk noted below for `(|`/`|)`.

**Known trade-off (ternary only)**: because `(|` and `|)` contain a
literal `(` or `)`, editors that do generic bracket-matching/
rainbow-bracket highlighting (without actually understanding cRust's
grammar) will see an unmatched paren inside each token. It's purely a
cosmetic editor-support annoyance, not a language ambiguity — the
lexer treats each as one atomic token (see the Phase 2 lexing notes in
[ARCHITECTURE.md](./ARCHITECTURE.md#phase-2--lexer-internallexer)) —
but it's worth knowing about before it's confused for a typo. A
cRust-aware editor mode/grammar (Phase 6 stretch) would fix the
highlighting; nothing to do about it before then.

## 6. Truthiness & Coercion Rules

Pinned down explicitly so the evaluator has one rule to follow instead of
per-operator special cases:

- **Falsy values**: only `thin` and `nobox`. Everything else — including
  `0`, `0.0`, `""`, `[]`, and `toppings{}` — is truthy. (Chosen
  deliberately: AoC inputs routinely produce `0` as a legitimate value,
  and a language where `0` is falsy invites exactly that class of bug.)
- **Arithmetic**: `int OP int` stays an int for `+ - *`; mixing an int and
  a float widens the result to float. `/` always produces a float
  (Python-3-style true division); integer division is a builtin,
  `idiv(a, b)`, and `%` (modulo) requires two ints.
- **`+` on strings, Lists, or Tuples** concatenates (same-type only —
  `+` between two Lists or two Tuples always produces a **new** value,
  never mutating either operand, same as string `+`; mixing a List with
  a Tuple, or either with a non-matching type, is a type error). `+`
  between a string/List/Tuple and something else (e.g. a string and a
  number) is a type error rather than an implicit conversion — keeps
  type mistakes visible instead of silently stringifying or coercing.
- **Equality (`==`/`!=`)** compares by value (Lists/Tuples/Maps/Sets
  compare their contents, not identity — in order for Lists/Tuples,
  ignoring order for Sets); comparing across types (e.g. `1 == "1"`,
  or a List against a same-contents Tuple) is always `thin`, never a
  type error.
  Integer and Float are **one type for this purpose**, the same
  "number" category ordering (below) already groups them into — `1 ==
  1.0` is `stuffed`, not `thin`. A Function only equals itself (the
  same closure), never a separately-defined one with an identical body.
- **Ordering (`< > <= >=`)** requires both operands to be the same
  comparable type (number-vs-number, freely mixing Integer/Float, or
  string-vs-string, lexicographic); comparing mismatched types this way
  *is* a runtime error, unlike `==`.
- **Division by zero** (`/`, `%`, or the `idiv` builtin, either operand
  a Float or both Integers) is always a runtime error — never an
  implicit `Inf`/`NaN`/silent wraparound.
- **Indexing is not negative-wrappable.** `list[-1]`/`s[-1]` is an
  `index out of range` error, the same as any other out-of-bounds
  index — there's no Python-style "count from the end." Want the last
  element? Compute the index: `list[slices(list) - 1]`.
- **A missing Map key reads as `nobox`**, not an error — this is what
  makes the memoization pattern `cache[n] = cache[n] ?: computeFib(n)`
  (§5.4) work at all: `?:` needs something to fall through *from* on
  the very first lookup of a new key.
- **`with`/`or` always produce a strict Boolean** (`stuffed`/`thin`),
  never one of the two operand's own values — unlike Python's
  `and`/`or`, which return whichever operand's value decided the
  result. `a with b` is exactly "compute `isTruthy(a) && isTruthy(b)`
  and give me that as a Boolean," not "give me back `a` or `b`." Use
  `?:` (§5.4) for the "give me back the actual value, falling through
  on `nobox`" behavior instead — that's what it's for.
- **A `recipe` that runs off the end without hitting `serve`** — and a
  bare `serve` with no expression — both evaluate to `nobox`, the same
  as Python's implicit `None` return. There's no implicit
  last-expression-as-return-value the way Rust/Ruby have; `serve` (bare
  or with a value) is the only way a `recipe` produces something other
  than `nobox`.

## 7. Standard Library (Builtins)

Draft — this list covers what's already implied by §2–§6 plus input and
type conversion; the rest of Phase 5 (general string helpers —
contains, replace — and further math/collection coverage) fleshes this
out later. Math functions (`min`, `max`, and eventually `abs`, `pow`,
`sqrt`, `gcd`, `lcm`) keep their standard names — they're universal
vocabulary, and forcing a pizza pun onto them would cost clarity for no
gain; `str`/`int`/`float`/`bool` (type conversion, below) are named on
that same principle. Everything else below is genuinely part of the
pizza theme, either because the pun was too good to pass up or because
it's directly tied to the Set type this doc introduces.

| Builtin | Signature | Does |
|---|---|---|
| `deliver(...)` | `deliver(values...)` | print — send output out |
| `slices(x)` | `slices(x) -> Integer` | length/count of a String, List, Tuple, Map, or Set |
| `sauce(value, fallback)` | `(Any, Any) -> Any` | returns `value` unless it's `nobox`, in which case returns `fallback` — same job as the `?:` operator (§5.4), as a plain function |
| `chars(s)` | `(String) -> List` | splits a string into a List of one-character strings |
| `ints(s)` / `ints(list)` | `(String) -> List` / `(List) -> List` | `ints(s)` splits a string of digits into a List of single-digit Integers — the numeric-grid counterpart to `chars`; a non-digit character is a runtime error. `ints(list)` instead parses each String element of `list` as a full (possibly multi-digit) Integer, the same way `int(x)` parses a String — so `ints(split(line))` turns a line of numbers straight into a List of Integers |
| `map(iterable, fn)` | `(List \| Tuple, Function) -> List` | applies `fn` to every element of a List or Tuple, in order, collecting the results into a new List; `fn` can be a recipe or another builtin. Chaining more than one transform per element is already possible by passing a lambda that does both (`map(xs, recipe(x) { serve g(f(x)) })`) — `map` only takes one function, not a list of them |
| `find(iterable, fn)` | `(List \| Tuple, Function) -> Any` | the lowest-index element of a List or Tuple for which `fn(element)` is stuffed — the element itself, not its index or `fn`'s result. `nobox` if none matches (or the collection is empty). `fn` can be a recipe or another builtin, same as `map` |
| `push(list, item)` | `(List, Any) -> Nil` | appends `item` to `list` in place — for a new List instead of mutating, use `+` (§5/§6) |
| `min(a, b, ...)` / `min(list)` | `(Any, Any, ...) -> Any` / `(List \| Tuple) -> Any` | smallest of 2+ direct arguments, or of a List/Tuple's elements — same ordering as `<` (§6): numbers (Integer/Float freely mixed) or Strings, never a mix of both. Returns the winning element itself, unconverted |
| `max(a, b, ...)` / `max(list)` | `(Any, Any, ...) -> Any` / `(List \| Tuple) -> Any` | largest of 2+ direct arguments, or of a List/Tuple's elements — same rules as `min` |
| `combos(list, n)` | `(List \| Tuple, Integer) -> List` | every n-element combination of `list`'s elements, each as a Tuple, in lexicographic order of position — order within a group doesn't matter and no element is reused within one group ("n choose k", not permutations). `combos(xs, 2)` is every pair, `combos(xs, 3)` every triple, and so on for any `n`. `n` greater than `slices(list)` gives an empty List (not an error); `n < 0` is an error. Every element of `list` must be Hashable, same as an ordinary `(a, b)` Tuple literal |
| `enumerate(list)` | `(List \| Tuple) -> List` | pairs each element of `list` with its 0-based position, each pair a `(index, value)` Tuple — the easy way to write an enumerated loop: `knead pair in enumerate(xs) { i, x = pair; ... }` unpacks both using the tuple-unpack assignment sugar (§3.1). Every element of `list` must be Hashable, same requirement and same reason as `combos` |
| `grid(s)` | `(String) -> Grid` | parses `s` into a Grid at offset `(0, 0)` — one character per cell — the 2D counterpart to `lines` + `chars` combined. `grid(unbox(path))` turns a raw grid-puzzle input file straight into something `at`/`setAt`/`gridBounds`/`neighbors4`/`neighbors8` work with. See §2.4 |
| `newGrid()` | `() -> Grid` | an empty Grid, for building one up entirely through `setAt` rather than parsing one from text (e.g. a simulation that starts from a handful of live cells) |
| `at(g, pos)` | `(Grid \| List, Tuple) -> Any` | bounds-checked read from `g` at `(row, col)` `pos` — `g` is usually a Grid, but a plain List of row Lists/Tuples still works too. Out-of-range reads as `nobox` rather than erroring — unlike plain `g[row][col]` indexing (not valid on a Grid at all — see §2.4) — so a candidate neighbor near an edge can be checked with `==`/`?:` instead of a hand-written bounds check |
| `setAt(g, pos, value)` | `(Grid, Tuple, Any) -> Nil` | write into Grid `g` at `pos`, `at`'s mutating counterpart. Unlike `at`, `g` must be a real Grid, not a plain List — growing to include an out-of-range `pos` (in any direction, including negative) needs a persistent origin offset a plain List has no room to keep between calls. Never errors on an out-of-range `pos`; it grows `g` to include it instead |
| `gridBounds(g)` | `(Grid) -> Tuple \| Nil` | `g`'s current `(minRow, minCol, maxRow, maxCol)`, or `nobox` if `g` is empty. The only way to learn a Grid's bounds after any number of expanding `setAt` calls, since a Grid isn't directly indexable/iterable (§2.4) |
| `neighbors4(pos)` / `neighbors8(pos)` | `(Tuple) -> List` | the 4 orthogonal, or 8 orthogonal+diagonal, neighbor positions of `(row, col)` `pos`, each as a Tuple — pure coordinate arithmetic, no bounds checking against any grid. Pair with `at` (nobox on out-of-range) to filter to only the neighbors that actually exist |
| `idiv(a, b)` | `(Integer, Integer) -> Integer` | integer (floor) division — `/` always true-divides to a Float (§6), this is how you get an Integer result back |
| `gather(list)` | `(List) -> Set` | collects a List into a Set, dropping duplicates |
| `sprinkle(set, item)` | `(Set, Any) -> Nil` | adds `item` to `set` in place |
| `scrape(set, item)` | `(Set, Any) -> Nil` | removes `item` from `set` in place, no error if absent |
| `topped(set, item)` | `(Set, Any) -> Boolean` | membership test — is `item` in `set`? |
| `contains(collection, item)` | `(List \| Tuple \| Set \| Map, Any) -> Boolean` | general membership test: for a List/Tuple, is `item` equal (`==`) to any element; for a Set, the same question `topped` answers; for a Map, is `item` a *key* (not a value) |
| `combine(a, b)` | `(Set, Set) -> Set` | union |
| `shared(a, b)` | `(Set, Set) -> Set` | intersection |
| `strip(a, b)` | `(Set, Set) -> Set` | difference — items in `a` not in `b` |
| `unbox()` / `unbox(path)` | `() -> String` / `(String) -> String` | reads all of stdin, or a whole file at `path` — either way, the raw contents, trailing newline and all |
| `lines(s)` | `(String) -> List` | splits `s` into a List of lines (handles `\n` and `\r\n`; a trailing newline doesn't produce an extra blank entry) |
| `split(s)` / `split(s, delim)` | `(String) -> List` / `(String, String) -> List` | `split(s)` splits on runs of whitespace, no empty entries (irregular spacing collapses); `split(s, delim)` splits on the literal `delim` instead, preserving empty entries between consecutive delimiters — a real CSV-style split, not whitespace-collapsing. `delim` can't be `""` — use `chars(s)` for that |
| `join(list, sep)` | `(List, String) -> String` | joins a List of Strings with `sep` between each — the counterpart to `split`; every element must already be a String (`str()` first if not) |
| `trim(s)` | `(String) -> String` | removes leading/trailing whitespace — distinct from `strip` (Set difference, above) on purpose, to avoid the name collision; the usual reason to reach for this is a trailing newline off `unbox()`'s raw output |
| `str(x)` | `(Any) -> String` | converts any value to its String form (the same text `deliver` would print for it) |
| `int(x)` | `(String \| Integer \| Float) -> Integer` | parses a String (base-10; a non-integer string like `"3.5"` is a runtime error, not a silent truncation) or truncates a Float toward zero; an Integer passes through unchanged |
| `float(x)` | `(String \| Integer \| Float) -> Float` | parses a String or widens an Integer; a Float passes through unchanged |
| `bool(x)` | `(Any) -> Boolean` | normalizes any value to a strict Boolean using §6's truthiness rule (only `thin`/`nobox` are falsy) — the same rule `order`/`bake`/ternary already apply, exposed as a value |

Type conversion is **always explicit** — there's no implicit coercion
between types anywhere in the language. This isn't a Phase 5 add-on
choice; it's the same rule §6 already states for operators (`+`
between a String and a number is a type error, not a silent
stringify), applied consistently: `str`/`int`/`float`/`bool` exist
specifically so a conversion is a visible, deliberate function call at
the point it happens, not something that could happen implicitly
somewhere else and be missed while reading the code.

## 8. Grammar (EBNF)

```ebnf
program        = { statement } ;

statement      = simpleStmt terminator | recipeStmt | orderStmt
               | kneadStmt | bakeStmt | serveStmt | burntStmt
               | flipStmt | block ;

simpleStmt     = unpackAssign | assignStmt | incDecStmt | expression ;
assignStmt     = lvalue assignOp expression ;
lvalue         = identifier { index } ;
assignOp       = "=" | "+=" | "-=" | "*=" | "/=" | "%=" ;

unpackAssign   = identifier "," identifier { "," identifier } "=" expression ;
                 (* whether this unpacks List-style or Tuple-style
                    (§2.3) depends on expression's *runtime* type, not
                    anything visible in this grammar — expression can
                    be a tupleLiteral directly, or any other expression
                    that evaluates to one (a ternary, a call, ...) *)
incDecStmt     = lvalue ( "++" | "--" ) | ( "++" | "--" ) lvalue ;

recipeStmt     = "recipe" [ identifier ] "(" [ paramList ] ")" block ;
paramList      = identifier { "," identifier } ;

orderStmt      = "order" "(" expression ")" block
                 { "combo" "(" expression ")" block }
                 [ "special" block ] ;

kneadStmt      = "knead" ( countedHeader | forEachHeader ) block ;
countedHeader  = "(" [ simpleStmt ] ";" [ expression ] ";"
                 [ simpleStmt ] ")" ;
forEachHeader  = identifier "in" expression ;

bakeStmt       = "bake" "(" expression ")" block ;

serveStmt      = "serve" [ expression ] terminator ;
burntStmt      = "burnt" terminator ;
flipStmt       = "flip" terminator ;

block          = "{" { statement } "}" ;
terminator     = NEWLINE | ";" ;

(* A NEWLINE only appears here — it's swallowed as insignificant
   whitespace, not tokenized at all, while the innermost unclosed
   delimiter is "(" or "[". See the note below the grammar. *)

expression     = ternary ;
ternary        = elvis [ "(|" expression "|)" ternary ] ;
elvis          = logicalOr [ "?:" elvis ] ;
logicalOr      = logicalAnd { "or" logicalAnd } ;
logicalAnd     = equality { "with" equality } ;
equality       = comparison { ( "==" | "!=" ) comparison } ;
comparison     = range { ( "<" | ">" | "<=" | ">=" ) range } ;
range          = term [ ( ".." | ".<" ) term ] ;
term           = factor { ( "+" | "-" ) factor } ;
factor         = unary { ( "*" | "/" | "%" ) unary } ;
unary          = [ "hold" | "-" ] callOrIndex ;

callOrIndex    = primary { call | index } ;
call           = "(" [ argList ] ")" ;
argList        = expression { "," expression } ;
index          = "[" expression "]" ;

primary        = INT | FLOAT | STRING | "stuffed" | "thin" | "nobox"
               | identifier | "(" expression ")" | tupleLiteral
               | listLiteral | mapLiteral | setLiteral | recipeStmt ;

listLiteral    = "[" [ expression { "," expression } ] "]" ;
tupleLiteral   = "(" expression "," expression { "," expression } ")" ;
mapLiteral     = "{" [ pair { "," pair } ] "}" ;
pair           = expression ":" expression ;
setLiteral     = "toppings" "{" [ expression { "," expression } ] "}" ;
```

Note on `simpleStmt`: it's the shared building block for both an
ordinary statement (`x = 1` followed by a terminator) and a `knead`
counted-loop clause (`knead (i = 0; i < n; i++) { ... }`), where no
terminator follows the init/post clauses — the parens do that job
instead. `unpackAssign` is checked before plain `assignStmt` since both
start with an identifier; the parser only knows which one it's in once
it sees whether a `,` or an `assignOp` follows.

Note on `"(" expression ")"` vs. `tupleLiteral`: both start with `(`,
so the parser resolves them with one expression's worth of lookahead —
parse the first inner expression, then check what follows. A `,`
commits to `tupleLiteral` (§2.3); a bare `)` means it was ordinary
grouping all along, and parsing continues as a normal `expression` from
there (so trailing operators after the `)`, e.g. `(x)..y`, still work).
This resolution happens in one place (the shared `(`-prefix parse
function) regardless of where the `(...)` appears — a ternary branch,
an unpack-assignment's right-hand side, a function argument, anywhere
`expression` is valid — so `tupleLiteral` is a real, general
production, not restricted to one grammar position.

Note on `ternary`: the condition is parsed at `elvis` precedence, one
level below `ternary` itself, so a bare ternary used as a condition
needs parens (§5.3). The `then` branch is parsed as a full `expression`
because it's unambiguously bounded by `|)`; the `else` branch recurses
back into `ternary`, which is what makes else-if-style chains
right-associate without extra grouping.

Note on `recipeStmt`'s optional identifier: the same production covers
both a named statement (`recipe add(a, b) { serve a + b }`) and an
anonymous function literal in expression position
(`double = recipe(x) { serve x * 2 }`), which is what `primary`
reusing `recipeStmt` already implied (§2's Function row shows the
anonymous form as the example). A bare `recipe(a, b) { ... }` as a
whole statement — anonymous, name omitted, value immediately
discarded — is legal by this grammar and harmless, if pointless.

Note on line continuation inside `(` / `[`: a raw newline only becomes
a `terminator` while nothing is open, or while the innermost open
delimiter is `{`. Inside an unclosed `(` or `[` it's insignificant
whitespace instead — the same rule Python and most C-family languages
use — so call arguments, `list` literals, and grouped expressions can
wrap across lines freely:

```
total = sum(
    1,
    2,
    3
)

xs = [
    1, 2,
    3, 4
]
```

(No trailing comma before the closing `)`/`]` — `argList`/`listLiteral`
above don't allow one.)

What governs this is the *innermost* open delimiter, not "is anything
open at all" — a block nested inside a still-open `(` (an anonymous
`recipe` literal passed as a call argument, `apply(recipe(x) {
serve x * 2 })`) keeps its own body's newlines significant, because the
innermost delimiter at that point is the block's `{`, not the outer
`(`. `{` itself never enables continuation, for a more fundamental
reason than just "blocks need it off": at this stage of the grammar
`{` is ambiguous between a `block` and a `mapLiteral`/being the
`toppings` half of a `setLiteral`, and only a `block` could safely have
newlines suppressed inside it. Rather than resolve that ambiguity,
`mapLiteral`/`setLiteral` simply inherit the restriction and stay
single-line for now:

```
m = {"a": 1, "b": 2}          // fine
m = {
    "a": 1,                   // parse error — NEWLINE has no
    "b": 2                    // meaning here yet
}
```

## 9. Entry Points

A file with no `store`-family recipe runs exactly as it does today —
every top-level statement executes in order, start to finish (§8's
`program` production). This feature is purely additive; no existing
`examples/*.crust` file needs to change.

A bare `recipe store() { ... }` is cRust's answer to Python's
`if __name__ == "__main__":` / Go's `func main()` — "the store is
open" is where a solution's actual entry point lives. Top-level
statements (global bindings, other `recipe` declarations) still run
first, exactly like a Python module executing at import time; once the
whole file has been evaluated, the CLI calls the resolved entry-point
recipe with zero arguments.

Multiple entry points per file are named by suffix — `recipe
store_part1() { ... }`, `recipe store_part2() { ... }` — a plain
naming convention, not new grammar: `store_part1` is already an
ordinary, valid `identifier` (§8), so nothing about the grammar
changes. The interpreter's entry-point resolver just looks, after
parsing, for any top-level `recipe` whose name is exactly `store` or
matches `store_<name>`.

Selected via `crust run <file> --store=<name>`:
- `crust run day01.crust` — runs the bare `store`, if one exists.
- `crust run day01.crust --store=part1` — runs `store_part1`.
- No `store`-family recipe at all — `--store` is ignored (there's
  nothing for it to select) and the file just runs top-to-bottom, per
  the no-entry-point case above.
- `store_part1`/`store_part2` exist but there's no bare `store`, and
  `--store` wasn't given — this is an error listing the available
  names, rather than silently running the whole file top-to-bottom,
  since that's very unlikely to be what someone who bothered to define
  named parts actually wants.

If the puzzle needs input, `store`/`store_part1` reads it itself via
the stdlib input builtins (§7) — there's no implicit `argv`-style
parameter passed in.

Naming convention chosen over a dedicated annotation (e.g. `recipe
part1() impl store { ... }`) specifically to avoid a new keyword:
`impl` has no obvious pizza-jargon equivalent, and reusing the
existing `recipeStmt` production this way costs zero grammar/lexer
changes — the same reasoning behind how anonymous-vs-named `recipe`
already works (`FunctionLiteral.Name == nil` for anonymous, no
separate syntax needed for the distinction).

This section is documented ahead of Phase 4 (the interpreter) actually
existing to implement it — the same "design before code" order used
for everything else in this project.

## 10. Examples

### Hello, World

```
deliver("Hello, World!")
```

### AoC-shaped sample (find a pair summing to a target — AoC 2020 Day 1 shape)

```
recipe findPair(nums, target) {
    knead (i = 0; i < slices(nums); i++) {
        knead (j = i + 1; j < slices(nums); j++) {
            order (nums[i] + nums[j] == target) {
                serve [nums[i], nums[j]]
            }
        }
    }
    serve nobox
}

entries = [1721, 979, 366, 299, 675, 1456]
pair = findPair(entries, 2020)

order (pair == nobox) {
    deliver("no combo found")
} special {
    deliver(pair[0] * pair[1])
}
```

### Sets and for-each (AoC 2020 Day 6 shape — group answer union/intersection)

```
recipe lineSet(line) {
    people = toppings{}
    knead ch in chars(line) {
        sprinkle(people, ch)
    }
    serve people
}

group = ["abc", "abac", "abcd"]

everyone = nobox
anyone = nobox

knead line in group {
    people = lineSet(line)
    order (everyone == nobox) {
        everyone = people
        anyone = people
    } special {
        everyone = shared(everyone, people)
        anyone = combine(anyone, people)
    }
}

deliver(slices(everyone))   // questions everyone answered yes to
deliver(slices(anyone))     // questions anyone answered yes to
```

### Loop control (`bake`, `knead`, `burnt`, `flip`)

```
n = 10
bake (n > 0) {
    order (n == 5) {
        flip
    }
    order (n == 2) {
        burnt
    }
    deliver(n)
    n--
}
```

### Unpacking and ranges

```
recipe firstAndRest(nums) {
    first, rest = nums
    serve [first, rest]
}

result = firstAndRest(1..5)   // range literal: [1, 2, 3, 4, 5]
deliver(result[0])            // 1
deliver(result[1])            // [2, 3, 4, 5]

knead i in 1.<5 {             // exclusive range: [1, 2, 3, 4]
    deliver(i)
}
```

### Ternary conditional (`(|`, `|)`)

```
recipe parity(n) {
    serve (n % 2 == 0) (| "even" |) "odd"
}

deliver(parity(4))   // even
deliver(parity(7))   // odd

score = 82
grade = (score >= 90) (| "A" |)
        (score >= 80) (| "B" |)
        (score >= 70) (| "C" |)
        "F"
deliver(grade)        // B
```

### Elvis / nil-coalescing (`?:`)

```
recipe greet(name) {
    display = name ?: "stranger"
    deliver("Hello, " + display + "!")
}

greet("Ryan")   // Hello, Ryan!
greet(nobox)    // Hello, stranger!

// memoized fibonacci: cache[n] only gets computed once per n
cache = {}

recipe fib(n) {
    cache[n] = cache[n] ?: computeFib(n)
    serve cache[n]
}

recipe computeFib(n) {
    order (n < 2) {
        serve n
    }
    serve fib(n - 1) + fib(n - 2)
}

deliver(fib(10))
```

Note `?:`'s precedence is *lower* than `+`, so `a + b ?: c` groups as
`(a + b) ?: c`, not `a + (b ?: c)` — that's why `greet` assigns the
coalesced value to `display` first rather than trying to inline it into
the concatenation. When mixing `?:` with other operators, parenthesize
explicitly rather than relying on precedence to do what you mean.

All examples also live as runnable files under
[`examples/`](../examples) once the interpreter exists to run them.
