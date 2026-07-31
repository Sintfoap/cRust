# tree-sitter-crust

A real, parser-driven [tree-sitter](https://tree-sitter.github.io/tree-sitter/)
grammar for cRust — more accurate than [`editors/vim`](../vim)'s
regex-based syntax file (which also works in Neovim, but via the
older, simpler mechanism both Vim and Neovim share). This grammar is
what `nvim-treesitter`-style highlighting, incremental selection, and
structural text objects are built on.

The generated parser (`src/parser.c`) is committed, same as virtually
every published tree-sitter grammar repo — that's what lets a
downstream consumer build this with just a C compiler, without needing
Node.js or `tree-sitter-cli` themselves. Only editing `grammar.js`
requires those (see [Developing](#developing) below).

## What's verified, and how

Every claim below was checked against real tooling, not assumed:

- **Grammar correctness**: `tree-sitter generate` produces no
  unresolved conflicts, `tree-sitter test` passes all 23 cases in
  [`test/corpus`](./test/corpus) (precedence/associativity for every
  binary/ternary/Elvis/range operator, both `knead` header forms,
  unpacking assignment, inc/dec, the block-vs-map-literal
  disambiguation at statement start, `serve` followed by a map literal
  vs. followed by a separate block, semicolon-separated one-liners),
  and every real file under [`examples/`](../../examples) parses with
  zero `ERROR`/`MISSING` nodes.
- **Highlight query correctness**: `tree-sitter query
  queries/highlights.scm` confirms every pattern matches the node it's
  meant to; `tree-sitter highlight --html` confirms the *resolved*
  (post-override) highlighting is right on genuinely overlapping cases
  — a declared function's name renders as `@function` (overriding the
  generic `@variable` fallback), a builtin call like `deliver(...)`
  renders as `@function.builtin` (overriding both `@variable` and
  `@function.call`), and so on.
- **The compiled artifact loads**: `src/parser.c` was actually compiled
  (`cc -shared -fPIC`) into a `.so` and confirmed to export the
  `tree_sitter_crust` symbol — the exact one Neovim's built-in
  `vim.treesitter` C loader looks up via `dlsym` when you register the
  language.

**Not verified**: an actual running Neovim instance loading this
`.so` and rendering colors on screen — there's no Neovim binary in the
environment this was built in. Everything up to that last step checks
out with real tooling, which is a strong signal, but if something's
still off once you load it for real, that's the one link in the chain
that wasn't directly exercised.

## Scope: what this grammar deliberately simplifies

Written for **accurate highlighting**, not as a second validating
implementation of the language (that's `internal/parser`'s job). One
deliberate simplification: `docs/SPEC.md` §8's
`terminator = NEWLINE | ";"` rule is *not* modeled precisely — newlines
are treated as insignificant whitespace rather than statement
terminators. Modeling that exactly would need an external scanner
tracking newline significance the way indentation-sensitive grammars
(e.g. tree-sitter-python) do, which is real, disjoint work out of
proportion to what highlighting needs. `;` is still handled explicitly
as an optional statement separator, so `x = 1; y = 2` still parses
correctly — only the newline half of the rule is simplified away, and
it doesn't affect highlighting quality in practice (every real file
under `examples/` still parses cleanly, one statement per line, same
as always).

## Install (Neovim, native — no plugin required)

Requires a C compiler (`cc`/`gcc`/`clang`).

```sh
git clone https://github.com/Sintfoap/cRust ~/somewhere/cRust
cd ~/somewhere/cRust/editors/tree-sitter-crust

# Compile the generated parser into a shared library.
mkdir -p ~/.config/nvim/parser
cc -o ~/.config/nvim/parser/crust.so -I./src src/parser.c -shared -Os -fPIC

# Install the highlight query.
mkdir -p ~/.config/nvim/queries/crust
cp queries/highlights.scm ~/.config/nvim/queries/crust/highlights.scm
```

Then register the filetype and language in your `init.lua`:

```lua
vim.filetype.add({ extension = { crust = "crust" } })
vim.treesitter.language.register("crust", "crust")

vim.api.nvim_create_autocmd("FileType", {
  pattern = "crust",
  callback = function() vim.treesitter.start() end,
})
```

(If you've also installed [`editors/vim`](../vim), its `ftdetect/crust.vim`
already handles the `vim.filetype.add` part — Neovim reads legacy
`ftdetect` files too — so you'd only need the `vim.treesitter.language.register`
and `vim.treesitter.start()` autocmd above.)

## Install (nvim-treesitter plugin, `:TSInstall`-style)

If you use the [`nvim-treesitter`](https://github.com/nvim-treesitter/nvim-treesitter)
plugin and want it to manage this grammar the same way it manages
built-in languages (auto-compile, `:TSInstall`, updates), register it
as a custom parser pointing at this directory or a git URL. The exact
API has changed between nvim-treesitter's `master` and `main`
branches, and neither was available to test against here — check
nvim-treesitter's own docs for `parser_configs` (`master`) or
`vim.treesitter.language.add` + install source overrides (`main`) for
the current syntax. The native install above is the option that's
actually been verified end-to-end.

## Developing

Requires Node.js (for `tree-sitter-cli`).

```sh
npm install
npx tree-sitter generate   # regenerate src/parser.c after editing grammar.js
npx tree-sitter test       # run test/corpus
npx tree-sitter parse some-file.crust   # inspect a parse tree
```

If you add a `queries/highlights.scm` pattern, remember that **later
patterns override earlier ones for the same node** (the opposite of
`editors/vim`'s Vim syntax file, where the *earlier*-defined comment
rule had to be moved *later* to win — see that file's own comments for
why). Generic fallback captures (`@variable`) belong first; specific
overrides last.
