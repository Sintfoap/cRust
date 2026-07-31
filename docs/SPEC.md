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
| Map | `{"a": 1, "b": 2}` | string or integer keys, mutable |
| **Set** | `toppings{1, 2, 3}` | unordered, unique — a pizza's toppings never repeat and don't have an order, so that's the name; see §2.2 |
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
| Arithmetic | `+  -  *  /  %` | `+` also concatenates strings; `/` always true-divides to a Float (§6); `%` requires two Integers |
| Range | `..  .<` | Integer-only, produces a List — see §5.1 |
| Unary | `-x`, `hold x` | numeric negate, logical not |
| Comparison | `==  !=  <  >  <=  >=` | see §6 for cross-type rules |
| Logical | `with` (and), `or` (or), `hold` (not) | keyword operators, not symbols |
| Assignment | `=  +=  -=  *=  /=  %=` | statement-level only (§3), not usable as a sub-expression — like Python's `=`, unlike C's |
| Increment/decrement | `++  --` | either side of the target (`i++` and `++i` are identical); statement-level only, no return value — see §5.2 |
| Ternary | `(\|`, `\|)` | `cond (\| then \|) else` — the two half-pizza glyphs as a matched pair, one job each — see §5.3 |
| Elvis / nil-coalesce | `?:` | `a ?: b` — a if it isn't `nobox`, else `b` — see §5.4 |
| Indexing | `x[i]` | List (by position), Map (by key), String (by position); **not** valid on Set |
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
- **`+` on strings** concatenates; `+` between a string and a number is a
  type error rather than an implicit conversion — keeps type mistakes
  visible instead of silently stringifying.
- **Equality (`==`/`!=`)** compares by value (Lists/Maps/Sets compare
  their contents, not identity, and ignore order for Sets); comparing
  across types (e.g. `1 == "1"`) is always `thin`, never a type error.
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
type conversion; the rest of Phase 5 (general string helpers — split,
join, trim, contains, replace — and math/collection coverage) fleshes
this out later. Math functions (`abs`, `min`, `max`, `pow`, `sqrt`,
`gcd`, `lcm`) are expected to keep their standard names — they're
universal vocabulary, and forcing a pizza pun onto them would cost
clarity for no gain; `str`/`int`/`float`/`bool` (type conversion, below)
are named on that same principle. Everything else below is genuinely
part of the pizza theme, either because the pun was too good to pass up
or because it's directly tied to the Set type this doc introduces.

| Builtin | Signature | Does |
|---|---|---|
| `deliver(...)` | `deliver(values...)` | print — send output out |
| `slices(x)` | `slices(x) -> Integer` | length/count of a String, List, Map, or Set |
| `sauce(value, fallback)` | `(Any, Any) -> Any` | returns `value` unless it's `nobox`, in which case returns `fallback` — same job as the `?:` operator (§5.4), as a plain function |
| `chars(s)` | `(String) -> List` | splits a string into a List of one-character strings |
| `ints(s)` | `(String) -> List` | splits a string of digits into a List of single-digit Integers — the numeric-grid counterpart to `chars`; a non-digit character is a runtime error |
| `idiv(a, b)` | `(Integer, Integer) -> Integer` | integer (floor) division — `/` always true-divides to a Float (§6), this is how you get an Integer result back |
| `gather(list)` | `(List) -> Set` | collects a List into a Set, dropping duplicates |
| `sprinkle(set, item)` | `(Set, Any) -> Nil` | adds `item` to `set` in place |
| `scrape(set, item)` | `(Set, Any) -> Nil` | removes `item` from `set` in place, no error if absent |
| `topped(set, item)` | `(Set, Any) -> Boolean` | membership test — is `item` in `set`? |
| `combine(a, b)` | `(Set, Set) -> Set` | union |
| `shared(a, b)` | `(Set, Set) -> Set` | intersection |
| `strip(a, b)` | `(Set, Set) -> Set` | difference — items in `a` not in `b` |
| `unbox()` / `unbox(path)` | `() -> String` / `(String) -> String` | reads all of stdin, or a whole file at `path` — either way, the raw contents, trailing newline and all |
| `lines(s)` | `(String) -> List` | splits `s` into a List of lines (handles `\n` and `\r\n`; a trailing newline doesn't produce an extra blank entry) |
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
               | identifier | "(" expression ")"
               | listLiteral | mapLiteral | setLiteral | recipeStmt ;

listLiteral    = "[" [ expression { "," expression } ] "]" ;
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
