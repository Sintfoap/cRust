# cRust Language Specification

This is the source of truth for cRust syntax and semantics. The parser's
structure should map onto the grammar here; when the two disagree, update
both in the same change.

## 1. Design Summary

- Dynamically typed, evaluated by a tree-walking interpreter (see
  [ARCHITECTURE.md](./ARCHITECTURE.md)).
- Brace-delimited blocks; statements end at a newline or `;`.
- Every structural keyword is pizza jargon — see [§3](#3-keyword-vocabulary).

## 2. Core Types

| Type | Example | Notes |
|---|---|---|
| Integer | `42`, `-7` | 64-bit signed |
| Float | `3.14`, `-0.5` | 64-bit |
| String | `"pepperoni"` | double-quoted, UTF-8 |
| Boolean | `stuffed`, `thin` | see §3 — no bare `true`/`false` |
| Nil | `nobox` | absence of a value |
| List | `[1, 2, 3]` | 0-indexed, heterogeneous, mutable |
| Map | `{"a": 1, "b": 2}` | string or integer keys |
| Function | `recipe(a, b) { ... }` | first-class, closes over defining scope |

## 3. Keyword Vocabulary

The whole language is framed as running a pizza kitchen: a `recipe` is a
procedure, a `topping` is something you add and can change, an `order`
either gets made or falls through to today's `special`.

| Keyword | Standard equivalent | Why |
|---|---|---|
| `topping` | `let` / mutable variable declaration | a topping is something you add to a pizza and can change |
| `sauce` | `const` / immutable declaration | the base layer — set once, doesn't change after |
| `recipe` | `func` — function definition | a reusable procedure for making something |
| `order` | `if` | an order comes in; it's handled if the condition holds |
| `combo` | `else if` | a chained alternative order |
| `special` | `else` | today's special — the fallback branch |
| `knead` | `for` (counted loop) | repetitive, bounded action, like kneading dough |
| `bake` | `while` (conditional loop) | keep going while the condition holds — "bake until done" |
| `burnt` | `break` | pull it out of the oven early |
| `flip` | `continue` | flip to the next iteration, skip the rest of this pass |
| `serve` | `return` | hand back the finished result |
| `stuffed` | `true` | crust is stuffed — full/true |
| `thin` | `false` | thin crust — empty/false |
| `nobox` | `nil` / `null` | an empty pizza box — nothing inside |
| `deliver(...)` | `print(...)` | send a result out (builtin, not a keyword) |
| `with` | `&&` / logical AND | "pepperoni **with** mushrooms" |
| `or` | `\|\|` / logical OR | plain English reads fine here, no jargon needed |
| `hold` | `!` / logical NOT | "**hold** the onions" |

Reserved for later phases, not yet implemented: `topping`-style module
import keyword (working name `delivery`) for Phase 5+ if a module system
gets built (see TODO.md stretch goals).

## 4. Truthiness & Coercion Rules

Pinned down explicitly so the evaluator has one rule to follow instead of
per-operator special cases:

- **Falsy values**: only `thin` and `nobox`. Everything else — including
  `0`, `0.0`, `""`, and `[]` — is truthy. (Chosen deliberately: AoC inputs
  routinely produce `0` as a legitimate value, and a language where `0` is
  falsy invites exactly that class of bug.)
- **Arithmetic**: `int OP int` stays an int for `+ - *`; mixing an int and
  a float widens the result to float. `/` always produces a float
  (Python-3-style true division); integer division is a builtin,
  `idiv(a, b)`, and `%` (modulo) requires two ints.
- **`+` on strings** concatenates; `+` between a string and a number is a
  type error rather than an implicit conversion — keeps type mistakes
  visible instead of silently stringifying.
- **Equality (`==`/`!=`)** compares by value; comparing across types
  (e.g. `1 == "1"`) is always `thin`, never a type error.

## 5. Grammar (EBNF)

```ebnf
program        = { statement } ;

statement      = toppingStmt | sauceStmt | recipeStmt | orderStmt
               | kneadStmt | bakeStmt | serveStmt | burntStmt
               | flipStmt | block | exprStmt ;

toppingStmt    = "topping" identifier "=" expression terminator ;
sauceStmt      = "sauce" identifier "=" expression terminator ;

recipeStmt     = "recipe" identifier "(" [ paramList ] ")" block ;
paramList      = identifier { "," identifier } ;

orderStmt      = "order" "(" expression ")" block
                 { "combo" "(" expression ")" block }
                 [ "special" block ] ;

kneadStmt      = "knead" "(" [ toppingStmt | exprStmt ] ";"
                 [ expression ] ";" [ expression ] ")" block ;

bakeStmt       = "bake" "(" expression ")" block ;

serveStmt      = "serve" [ expression ] terminator ;
burntStmt      = "burnt" terminator ;
flipStmt       = "flip" terminator ;

block          = "{" { statement } "}" ;
exprStmt       = expression terminator ;
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
               | listLiteral | mapLiteral | recipeStmt ;

listLiteral    = "[" [ expression { "," expression } ] "]" ;
mapLiteral     = "{" [ pair { "," pair } ] "}" ;
pair           = expression ":" expression ;
```

This maps directly onto the Pratt-parser precedence ladder in
[ARCHITECTURE.md](./ARCHITECTURE.md#phase-3--parser-internalparser):
`LOWEST < or < with < equality < comparison < sum < product < prefix <
call/index`.

## 6. Examples

### Hello, World

```
deliver("Hello, World!")
```

### AoC-shaped sample (find a pair summing to a target — AoC 2020 Day 1 shape)

```
recipe findPair(nums, target) {
    knead (topping i = 0; i < len(nums); i = i + 1) {
        knead (topping j = i + 1; j < len(nums); j = j + 1) {
            order (nums[i] + nums[j] == target) {
                serve [nums[i], nums[j]]
            }
        }
    }
    serve nobox
}

topping entries = [1721, 979, 366, 299, 675, 1456]
topping pair = findPair(entries, 2020)

order (pair == nobox) {
    deliver("no combo found")
} special {
    deliver(pair[0] * pair[1])
}
```

### Loop control (`bake`, `knead`, `burnt`, `flip`)

```
topping n = 10
bake (n > 0) {
    order (n == 5) {
        flip
    }
    order (n == 2) {
        burnt
    }
    deliver(n)
    n = n - 1
}
```

Both examples also live as runnable files under [`examples/`](../examples)
once the interpreter exists to run them.
