# cRust syntax highlighting — Vim / Neovim

Traditional regex-based Vim syntax files (`syntax/crust.vim`,
`ftdetect/crust.vim`, `ftplugin/crust.vim`). These work **identically in
Vim and Neovim** — this is the same runtimepath mechanism both editors
support, so there's no separate Neovim-only Tree-sitter grammar here.
(A Tree-sitter grammar would give more accurate, parser-driven
highlighting instead of regex heuristics, but it's a genuinely separate
project — its own grammar.js, generated C parser, and build tooling —
tracked as a further stretch goal in [TODO.md](../../TODO.md) rather
than attempted here.)

Highlights every keyword, builtin, literal, and operator in
[`docs/SPEC.md`](../../docs/SPEC.md) §4/§5/§7, including the
multi-character operators (`(|`/`|)`, `?:`, `..`/`.<`, `++`/`--`, the
compound assignment operators) as single atomic tokens rather than
their individual characters. Verified against a real Vim instance
(`vim -Nu NONE -es` + `synID()` checks over a file exercising every
category), not just read off the docs.

## Install

Pick whichever matches your setup. All of these pull down the whole
`cRust` repo (there's no way around that with a plugin manager unless
this ends up split into its own repo later) but only put
`editors/vim`'s three subdirectories on the runtimepath.

### Plugin manager (recommended)

**vim-plug** (Vim or Neovim) has a built-in option for exactly this —
a plugin whose Vim files live in a subdirectory rather than the repo
root — so it's the most reliable option to reach for:

```vim
Plug 'Sintfoap/cRust', { 'rtp': 'editors/vim' }
```

Neovim's newer plugin managers (`lazy.nvim`, `packer.nvim`) don't have
a first-class equivalent of vim-plug's `rtp` option as of this writing,
so getting the runtimepath right through them takes more manual setup
than is worth documenting here without a way to test it — the native
package or manual install below are the more reliably correct options
for a subdirectory-only plugin like this one until this gets split into
its own repo.

### Native package managers (no plugin manager)

Clone the repo somewhere, then symlink just the `editors/vim`
subdirectory into your package path (`:help packages`) — same idea for
both, just a different target directory:

**Neovim**:

```sh
git clone https://github.com/Sintfoap/cRust ~/somewhere/cRust
mkdir -p ~/.local/share/nvim/site/pack/crust/start
ln -s ~/somewhere/cRust/editors/vim ~/.local/share/nvim/site/pack/crust/start/crust-lang
```

**Vim 8+**:

```sh
git clone https://github.com/Sintfoap/cRust ~/somewhere/cRust
mkdir -p ~/.vim/pack/crust/start
ln -s ~/somewhere/cRust/editors/vim ~/.vim/pack/crust/start/crust-lang
```

### Manual (simplest, no package manager)

Copy the three subdirectories straight into your Vim config directory:

```sh
cp -r editors/vim/{syntax,ftdetect,ftplugin} ~/.vim/      # Vim, Unix
cp -r editors/vim/{syntax,ftdetect,ftplugin} ~/.config/nvim/   # Neovim
```

## What you get

- Syntax highlighting for `*.crust` files (auto-detected via
  `ftdetect/crust.vim`)
- `commentstring`/`comments` set to `// %s` (`ftplugin/crust.vim`), so
  `gcc`-style comment toggling (built-in in newer Vim/Neovim, or via
  `tpope/vim-commentary`) works out of the box
- Basic C-family indent settings (`cindent`, 4-space `shiftwidth`),
  matching cRust's brace-delimited grammar (`docs/SPEC.md` §8)

## Known limitation

`(|`/`|)` (the ternary operator, `docs/SPEC.md` §5.3) contain a literal
`(`/`)` character. Generic bracket-matching (`%` motion, rainbow-bracket
plugins) will see these as unmatched parens — this is a deliberate,
documented trade-off in the language design itself (see SPEC.md §5.3's
note), not a bug in this syntax file. The two tokens still highlight
correctly as one atomic operator either way.
