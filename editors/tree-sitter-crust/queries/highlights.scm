; cRust highlight queries — see docs/SPEC.md §4/§5/§7 for the keyword/
; operator/builtin vocabulary this maps to standard nvim-treesitter
; capture names.
;
; Ordering matters: nvim-treesitter (and tree-sitter query matching in
; general) lets a *later* pattern's capture override an *earlier* one
; for the same node, so generic fallback captures (@variable) are
; listed first and more specific overrides (a declared function's
; name, a builtin call) come after — verified against the actual
; `tree-sitter query` output, not assumed.

; --- Generic fallback (overridden below where more specific) -----------

(identifier) @variable

; --- Literals ------------------------------------------------------------

(comment) @comment

(string_literal) @string
(escape_sequence) @string.escape

(integer_literal) @number
(float_literal) @number.float

(boolean_literal) @boolean
(nil_literal) @constant.builtin

; --- Keywords (SPEC.md §4) ------------------------------------------------

["order" "combo" "special"] @keyword.conditional
["knead" "bake"] @keyword.repeat
"serve" @keyword.return
["burnt" "flip"] @keyword
"in" @keyword
"toppings" @keyword
"recipe" @keyword.function
["with" "or" "hold"] @keyword.operator

; --- Functions -------------------------------------------------------------

(function_literal name: (identifier) @function)
(parameter_list (identifier) @variable.parameter)
(call_expression function: (identifier) @function.call)

; Builtins (SPEC.md §7) — matched by name, since there's no separate
; builtin node type in the grammar (they're plain identifiers
; syntactically; SPEC.md §4 itself says user code can shadow them, so
; the grammar can't tell "builtin" from "ordinary call" structurally
; either — this is a best-effort, name-based highlight only).
((identifier) @function.builtin
  (#any-of? @function.builtin
    "deliver" "slices" "sauce" "chars" "idiv"
    "gather" "sprinkle" "scrape" "topped" "combine" "shared" "strip"))

; --- Operators (SPEC.md §5) ------------------------------------------------

[
  "=" "+=" "-=" "*=" "/=" "%="
  "+" "-" "*" "/" "%"
  "==" "!=" "<" ">" "<=" ">="
  "++" "--"
  "(|" "|)" "?:" ".." ".<"
] @operator

; --- Punctuation -----------------------------------------------------------

["{" "}" "(" ")" "[" "]"] @punctuation.bracket
["," ":" ";"] @punctuation.delimiter
