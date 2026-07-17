# cRust 🍕

An interpreted programming language, written in Go, where every keyword is
pizza jargon. Built to solve [Advent of Code 2026](https://adventofcode.com/).

## Status

Language design (Phase 1) is done. See [TODO.md](./TODO.md) for the
roadmap and milestones, [docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md)
for the technical design behind each phase, and
[docs/SPEC.md](./docs/SPEC.md) for the actual language — keyword table,
grammar, and semantics. Next up: Phase 2, the lexer.

## Why

AoC rewards a language you enjoy typing at 6am with a puzzle timer running.
cRust is that language: a small, tree-walking interpreter with just enough
features (ints, floats, strings, lists, maps, sets, functions, control
flow) to comfortably solve AoC-style puzzles, wrapped in a
pizzeria-themed syntax. No `let`/`const` ceremony — variables just get
assigned, Python-style.

## A Taste

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
```

See [`examples/`](./examples) for runnable-once-the-interpreter-exists
sample programs.
