# cRust syntax highlighting — VSCode

A TextMate grammar (`syntaxes/crust.tmLanguage.json`) packaged as a
minimal VSCode language extension. Not published to the Marketplace —
install it locally with one of the options below.

Highlights every keyword, builtin, literal, and operator in
[`docs/SPEC.md`](../../docs/SPEC.md) §4/§5/§7, including the
multi-character operators (`(|`/`|)`, `?:`, `..`/`.<`, `++`/`--`, the
compound assignment operators) as single atomic tokens rather than
their individual characters — verified by tokenizing a file exercising
every category through the actual `vscode-textmate`/`vscode-oniguruma`
engine VSCode itself uses, not just read off the grammar. `deliver`,
`slices`, `sauce`, and the rest of the builtins (`docs/SPEC.md` §7)
only highlight as builtins when actually called (`deliver(...)`) — not
when shadowed as a plain identifier (`deliver = recipe(x) {...}`,
SPEC.md §4's "builtins are predeclared, not reserved" rule), matching
how the interpreter itself treats them.

## Install

### Option A — Developer: Install Extension from Location (easiest)

1. Clone this repo.
2. In VSCode: `Ctrl+Shift+P` / `Cmd+Shift+P` → **Developer: Install
   Extension from Location...** → select the `editors/vscode`
   directory.
3. Reload the window if prompted.

### Option B — Symlink into your extensions folder

```sh
git clone https://github.com/Sintfoap/cRust ~/somewhere/cRust
ln -s ~/somewhere/cRust/editors/vscode ~/.vscode/extensions/crust-language
```

(`~/.vscode/extensions` on Linux/macOS; `%USERPROFILE%\.vscode\extensions`
on Windows. Use `~/.vscode-server/extensions` instead for a remote/WSL/
Codespaces window.) Restart VSCode afterward.

### Option C — Package as a .vsix

If you'd rather have an installable package (e.g. to hand to someone
else, or install into VSCode without a symlink):

```sh
npm install -g @vscode/vsce
cd editors/vscode
vsce package
code --install-extension crust-language-0.1.0.vsix
```

## What you get

- Syntax highlighting for `.crust` files
- Line-comment toggling (`Ctrl+/`) using `//`
- Auto-closing/surrounding pairs for `{}`, `[]`, `()`, and `""`

## Known limitation

`(|`/`|)` (the ternary operator, `docs/SPEC.md` §5.3) contain a literal
`(`/`)` character. VSCode's generic bracket-matching/auto-close will
see these as unmatched parens — this is a deliberate, documented
trade-off in the language design itself (see SPEC.md §5.3's note), not
a bug in this grammar. The two tokens still highlight correctly as one
atomic operator either way; only bracket-pair *matching/colorization*
(not syntax highlighting) is affected.
