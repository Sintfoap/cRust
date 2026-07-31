" Filetype settings for cRust — comment string (used by both Vim's
" built-in gcc-style commenting in newer versions and plugins like
" tpope/vim-commentary) and basic indent expectations.
if exists("b:did_ftplugin")
  finish
endif
let b:did_ftplugin = 1

setlocal commentstring=//\ %s
setlocal comments=://

" SPEC.md's grammar is brace-delimited with C-family indent shape.
setlocal cindent
setlocal shiftwidth=4
setlocal softtabstop=4
setlocal expandtab
