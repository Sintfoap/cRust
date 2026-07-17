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
- Unpacking assignment, integer ranges (`..` / `.<`), `++`/`--`, and the
  nil-coalescing pair `(|`/`|)` round out the ergonomics that matter
  most for tight AoC loops — see [§3.1](#31-unpacking-assignment) and
  [§5](#5-operators).

## 2. Core Types

| Type | Example | Notes |
|---|---|---|
| Integer | `42`, `-7` | 64-bit signed |
| Float | `3.14`, `-0.5` | 64-bit |
| String | `"pepperoni"` | double-quoted, UTF-8 |
| Boolean | `stuffed`, `thin` | see §4 — no bare `true`/`false` |
| Nil | `nobox` | absence of a value |
| List | `[1, 2, 3]` | 0-indexed, ordered, heterogeneous, mutable |
| Map | `{"a": 1, "b": 2}` | string or integer keys, mutable |
| **Set** | `toppings{1, 2, 3}` | unordered, unique — a pizza's toppings never repeat and don't have an order, so that's the name; see §2.1 |
| Function | `recipe(a, b) { ... }` | first-class, closes over defining scope |

### 2.1 Sets (`toppings{...}`)

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
| `toppings` | Set literal/type | a pizza's toppings: no duplicates, no order — see §2.1 |
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
| Assignment | `=  +=  -=  *=  /=  %=  \|)` | statement-level only (§3), not usable as a sub-expression — like Python's `=`, unlike C's; `\|)` is conditional — see §5.3 |
| Increment/decrement | `++  --` | either side of the target (`i++` and `++i` are identical); statement-level only, no return value — see §5.2 |
| Nil-coalesce | `(\|` | expression form of "or this fallback" — see §5.3 |
| Indexing | `x[i]` | List (by position), Map (by key), String (by position); **not** valid on Set |
| Grouping | `( )` | expression grouping |

Set union/intersection/difference are deliberately **not** operators —
overloading `+`/`&`/`-` with a second meaning for one type isn't worth
the ambiguity. They're builtins instead: `combine`, `shared`, `strip`
(§7).

Precedence, low to high (unchanged shape, assignment/increment sit
outside this ladder as statement forms):

```
coalesce (|)  <  or  <  with  <  equality (== !=)  <  comparison (< > <= >=)
    <  range (.. .<)  <  sum (+ -)  <  product (* / %)
    <  unary (- hold)  <  call/index
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

### 5.3 Nil-Coalescing (`(|`, `|)`)

Two half-pizza glyphs, one job split across an expression form and a
statement form — mirroring how `sauce(value, fallback)` (§7) already
does this as a function call:

```
name = maybeNil (| "default"      // expression: use maybeNil, or "default" if it's nobox

cache[key] |) expensiveCompute()  // statement: fill cache[key] only if it's currently nobox
```

- **`a (| b`** (expression) evaluates `a`; if it isn't `nobox`, that's
  the result and `b` is never evaluated. If `a` *is* `nobox`, `b` is
  evaluated and becomes the result. Right-associative and chainable —
  `a (| b (| c` tries `a`, then `b`, then `c`. Usable anywhere an
  expression is: `deliver(x (| "n/a")`, `total += y (| 0`. This is
  exactly `sauce(a, b)` as an operator; pick whichever reads better at
  the call site.
- **`x |) expr`** (statement, one more entry in the assignment family
  from §3) assigns `expr` to `x` *only if* `x` is currently `nobox`;
  otherwise it's a no-op and, critically, **`expr` is never evaluated**
  — the same short-circuiting as `(|`, just landing in an assignment
  instead of producing a value. That's what makes
  `cache[key] |) expensiveCompute()` safe to write on every lookup: the
  expensive call only actually runs on a cache miss.
- **Both test for `nobox` specifically, not falsiness.** `thin (|
  "fallback"` stays `thin` — a real Boolean isn't an absence, so it's
  never replaced. This is the same principle behind §6's truthiness
  rule (`0` isn't falsy) applied to a different operator family: don't
  let "empty-ish" and "absent" collapse into the same check.
- Lowest precedence of any binary operator (§5's ladder) — a coalesce
  reads as "everything to my left, or this fallback," so it should
  never need parens to grab the whole preceding expression.
- The target of `|)` follows the same lvalue rule as `=`: identifier or
  index expression.

Why two glyphs instead of one: `(|` and `|)` read as the two cut halves
of a pizza, and splitting the job in half is exactly what they do —
`(|` hands back a value (it's the one that "opens" into an expression),
`|)` closes a box that was empty (it's the one that "seals" an
assignment). They're a matched pair by shape, not by grammar — each is
an independent binary operator, not an opening/closing delimiter pair
like `( )`.

**Known trade-off**: because these operators contain a literal `(` or
`)`, editors that do generic bracket-matching/rainbow-bracket
highlighting (without actually understanding cRust's grammar) will see
an unmatched paren inside `(|` and an unmatched paren inside `|)`. It's
purely a cosmetic editor-support annoyance, not a language ambiguity —
the lexer treats each as one atomic token (see the Phase 2 lexing notes
in [ARCHITECTURE.md](./ARCHITECTURE.md#phase-2--lexer-internallexer))
— but it's worth knowing about before it's confused for a typo. A
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
  their contents, not identity); comparing across types (e.g.
  `1 == "1"`) is always `thin`, never a type error.
- **Ordering (`< > <= >=`)** requires both operands to be the same
  comparable type (number-vs-number or string-vs-string, lexicographic);
  comparing mismatched types this way *is* a runtime error, unlike `==`.

## 7. Standard Library (Builtins)

Draft — this list covers what's already implied by §2–§6; the rest of
Phase 5 (full string/math/collection coverage, file and stdin input)
fleshes this out later. Math functions (`abs`, `min`, `max`, `pow`,
`sqrt`, `gcd`, `lcm`) are expected to keep their standard names —
they're universal vocabulary, and forcing a pizza pun onto them would
cost clarity for no gain. Everything below is genuinely part of the
pizza theme, either because the pun was too good to pass up or because
it's directly tied to the Set type this doc introduces.

| Builtin | Signature | Does |
|---|---|---|
| `deliver(...)` | `deliver(values...)` | print — send output out |
| `slices(x)` | `slices(x) -> Integer` | length/count of a String, List, Map, or Set |
| `sauce(value, fallback)` | `(Any, Any) -> Any` | returns `value` unless it's `nobox`, in which case returns `fallback` — same job as the `(|` operator (§5.3), as a plain function |
| `chars(s)` | `(String) -> List` | splits a string into a List of one-character strings |
| `gather(list)` | `(List) -> Set` | collects a List into a Set, dropping duplicates |
| `sprinkle(set, item)` | `(Set, Any) -> Nil` | adds `item` to `set` in place |
| `scrape(set, item)` | `(Set, Any) -> Nil` | removes `item` from `set` in place, no error if absent |
| `topped(set, item)` | `(Set, Any) -> Boolean` | membership test — is `item` in `set`? |
| `combine(a, b)` | `(Set, Set) -> Set` | union |
| `shared(a, b)` | `(Set, Set) -> Set` | intersection |
| `strip(a, b)` | `(Set, Set) -> Set` | difference — items in `a` not in `b` |

## 8. Grammar (EBNF)

```ebnf
program        = { statement } ;

statement      = simpleStmt terminator | recipeStmt | orderStmt
               | kneadStmt | bakeStmt | serveStmt | burntStmt
               | flipStmt | block ;

simpleStmt     = unpackAssign | assignStmt | incDecStmt | expression ;
assignStmt     = lvalue assignOp expression ;
lvalue         = identifier { index } ;
assignOp       = "=" | "+=" | "-=" | "*=" | "/=" | "%=" | "|)" ;

unpackAssign   = identifier "," identifier { "," identifier } "=" expression ;
incDecStmt     = lvalue ( "++" | "--" ) | ( "++" | "--" ) lvalue ;

recipeStmt     = "recipe" identifier "(" [ paramList ] ")" block ;
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

expression     = coalesce ;
coalesce       = logicalOr [ "(|" coalesce ] ;
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

Note on `"|)"` as an `assignOp`: the grammar treats it like any other
compound assignment, but it isn't evaluated like one — see §5.3.
`x += expr` always evaluates `expr` and writes the result; `x |) expr`
only evaluates and writes `expr` if `x` is currently `nobox`.

## 9. Examples

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

### Nil-coalescing (`(|`, `|)`)

```
recipe greet(name) {
    display = name (| "stranger"
    deliver("Hello, " + display + "!")
}

greet("Ryan")   // Hello, Ryan!
greet(nobox)    // Hello, stranger!

// memoized fibonacci: cache[n] only gets computed once per n
cache = {}

recipe fib(n) {
    cache[n] |) computeFib(n)
    serve cache[n]
}

recipe computeFib(n) {
    order (n < 2) {
        serve n
    }
    serve fib(n - 1) + fib(n - 2)
}
```

Note `(|`'s precedence is *lower* than `+`, so `a + b (| c` groups as
`(a + b) (| c)`, not `a + (b (| c)` — that's why `greet` assigns the
coalesced value to `display` first rather than trying to inline it into
the concatenation. When mixing `(|` with other operators, parenthesize
explicitly rather than relying on precedence to do what you mean.

All examples also live as runnable files under
[`examples/`](../examples) once the interpreter exists to run them.
