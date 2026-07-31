" Vim syntax file
" Language: cRust
" Latest Revision: see docs/SPEC.md for the source of truth this tracks
"
" Works unmodified in both Vim and Neovim — this is the traditional
" regex-based syntax mechanism both editors support identically; no
" separate Neovim/Tree-sitter grammar is required for highlighting to
" work in Neovim too.

if exists("b:current_syntax")
  finish
endif

" --- Strings (defined early: nothing else should match inside them) ------

syn match crustEscape /\\[\\"ntr]/ contained
syn region crustString start=/"/ skip=/\\"/ end=/"/ contains=crustEscape oneline

" --- Numbers -------------------------------------------------------------
" crustInteger is defined first so crustFloat (defined after) wins where
" they'd otherwise both match at the same start position — e.g. "5.5"
" would match crustInteger's "5" alone if it were defined last (see
" :help syn-priority: last-defined wins on overlap).

syn match crustInteger /\<\d\+\>/
syn match crustFloat   /\<\d\+\.\d\+\>/

" --- Keywords (SPEC.md §4) — split by semantic role so each gets the
" --- highlight group its meaning actually maps to, rather than one
" --- generic "keyword" bucket for everything -----------------------------

syn keyword crustConditional order combo special
syn keyword crustRepeat      knead bake
syn keyword crustStatement   serve burnt flip
syn keyword crustKeyword     in toppings
syn keyword crustBoolean     stuffed thin
syn keyword crustNil         nobox
syn keyword crustLogicalOp   with or hold

" `recipe` itself, plus the name right after it in a declaration
" (`recipe foo(...)`) — anonymous `recipe(...)` still gets the keyword
" highlight from crustDeclare alone, since there's no name to match.
syn keyword crustDeclare recipe nextgroup=crustFunctionName skipwhite
syn match   crustFunctionName /\<\h\w*\>/ contained

" --- Builtins (SPEC.md §7) — highlighted as functions, distinct from
" --- keywords, since user code is free to shadow them (SPEC.md §4) ------

syn keyword crustBuiltin deliver slices sauce chars idiv
syn keyword crustBuiltin gather sprinkle scrape topped combine shared strip

" --- Operators (SPEC.md §5) — multi-character tokens are matched as
" --- their own rule, defined after the single-character punctuation
" --- rules below so they take priority at the same start position (see
" --- :help syn-priority: the syntax item defined last wins on overlap).
" --- '(|' and '|)' in particular share characters with plain '(' / ')',
" --- which is a known cosmetic limitation for *any* editor doing
" --- regex-only bracket matching — see SPEC.md §5.3's note on this;
" --- highlighting them as one atomic operator token here is the most
" --- this syntax file can do about it. ---------------------------------

syn match crustDelimiter /[{}()\[\]]/
syn match crustSeparator /[,:;]/
syn match crustOperator  /[+\-*\/%<>=]/

syn match crustTernary /(|\||)/
syn match crustElvis   /?:/
syn match crustRange   /\.\.\|\.</
syn match crustIncDec  /++\|--/
syn match crustCompoundAssign /[+\-*\/%]=/
syn match crustCompare /==\|!=\|<=\|>=/

" --- Comments — defined *last*, deliberately, so it wins the priority
" --- tie-break against crustOperator's bare '/' (division) at a
" --- comment's opening "//": since crustOperator's single-'/' match and
" --- crustComment's "//..." match both start at the same column, the
" --- item defined later wins (:help syn-priority) — this has to be the
" --- comment, or every "//" would highlight as two division operators
" --- instead of a comment. Confirmed by testing against a real Vim
" --- instance, not just read off the docs. ------------------------------

syn match crustComment "//.*$"

" --- Highlight group links ----------------------------------------------

hi def link crustComment        Comment
hi def link crustString         String
hi def link crustEscape         SpecialChar
hi def link crustFloat          Float
hi def link crustInteger        Number
hi def link crustConditional    Conditional
hi def link crustRepeat         Repeat
hi def link crustStatement      Statement
hi def link crustKeyword        Keyword
hi def link crustBoolean        Boolean
hi def link crustNil            Constant
hi def link crustLogicalOp      Keyword
hi def link crustDeclare        Keyword
hi def link crustFunctionName   Function
hi def link crustBuiltin        Function
hi def link crustDelimiter      Delimiter
hi def link crustSeparator      Delimiter
hi def link crustOperator       Operator
hi def link crustTernary        Operator
hi def link crustElvis          Operator
hi def link crustRange          Operator
hi def link crustIncDec         Operator
hi def link crustCompoundAssign Operator
hi def link crustCompare        Operator

let b:current_syntax = "crust"
