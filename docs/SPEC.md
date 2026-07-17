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
| Unary | `-x`, `hold x` | numeric negate, logical not |
| Comparison | `==  !=  <  >  <=  >=` | see §6 for cross-type rules |
| Logical | `with` (and), `or` (or), `hold` (not) | keyword operators, not symbols |
| Assignment | `=  +=  -=  *=  /=  %=` | statement-level only (§3), not usable as a sub-expression — like Python's `=`, unlike C's |
| Indexing | `x[i]` | List (by position), Map (by key), String (by position); **not** valid on Set |
| Grouping | `( )` | expression grouping |

Set union/intersection/difference are deliberately **not** operators —
overloading `+`/`&`/`-` with a second meaning for one type isn't worth
the ambiguity. They're builtins instead: `combine`, `shared`, `strip`
(§7).

Precedence, low to high (unchanged shape, assignment sits outside this
ladder as a statement form):

```
or  <  with  <  equality (== !=)  <  comparison (< > <= >=)
    <  sum (+ -)  <  product (* / %)  <  unary (- hold)  <  call/index
```

This maps directly onto the Pratt-parser dispatch tables in
[ARCHITECTURE.md](./ARCHITECTURE.md#phase-3--parser-internalparser).

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
| `sauce(value, fallback)` | `(Any, Any) -> Any` | returns `value` unless it's `nobox`, in which case returns `fallback` — the base layer under a possibly-missing value |
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

simpleStmt     = assignStmt | expression ;
assignStmt     = lvalue assignOp expression ;
lvalue         = identifier { index } ;
assignOp       = "=" | "+=" | "-=" | "*=" | "/=" | "%=" ;

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

expression     = logicalOr ;
logicalOr      = logicalAnd { "or" logicalAnd } ;
logicalAnd     = equality { "with" equality } ;
equality       = comparison { ( "==" | "!=" ) comparison } ;
comparison     = term { ( "<" | ">" | "<=" | ">=" ) term } ;
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
counted-loop clause (`knead (i = 0; i < n; i += 1) { ... }`), where no
terminator follows the init/post clauses — the parens do that job
instead.

## 9. Examples

### Hello, World

```
deliver("Hello, World!")
```

### AoC-shaped sample (find a pair summing to a target — AoC 2020 Day 1 shape)

```
recipe findPair(nums, target) {
    knead (i = 0; i < slices(nums); i += 1) {
        knead (j = i + 1; j < slices(nums); j += 1) {
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
    n -= 1
}
```

All three examples also live as runnable files under
[`examples/`](../examples) once the interpreter exists to run them.
