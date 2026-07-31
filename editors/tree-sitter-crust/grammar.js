// tree-sitter grammar for cRust — see docs/SPEC.md §8 for the
// authoritative EBNF grammar this tracks. This grammar is written for
// accurate *editor highlighting* (nvim-treesitter), not as a second
// validating implementation of the language: it deliberately treats
// newlines as insignificant whitespace rather than modeling SPEC.md
// §8's `terminator = NEWLINE | ";"` rule precisely (that needs an
// external scanner to track — real, but out of proportion to what
// highlighting needs). `;` is still handled explicitly as an optional
// statement separator, so semicolon-separated one-liners still parse
// correctly; only the newline-as-terminator half is simplified away.

const PREC = {
  TERNARY: 1,
  ELVIS: 2,
  OR: 3,
  WITH: 4,
  EQUALITY: 5,
  COMPARISON: 6,
  RANGE: 7,
  SUM: 8,
  PRODUCT: 9,
  UNARY: 10,
  CALL: 11,
};

function commaSep1(rule) {
  return seq(rule, repeat(seq(',', rule)));
}

module.exports = grammar({
  name: 'crust',

  extras: $ => [/\s/, $.comment],

  word: $ => $.identifier,

  conflicts: $ => [
    [$.block, $.map_literal],
    [$.serve_statement],
    [$._lvalue, $._expression],
  ],

  rules: {
    program: $ => repeat($._statement),

    comment: $ => token(seq('//', /.*/)),

    // --- Statements (SPEC.md §8) -------------------------------------

    _statement: $ => choice(
      prec(1, $.block),
      $.order_statement,
      $.knead_statement,
      $.bake_statement,
      $.serve_statement,
      $.burnt_statement,
      $.flip_statement,
      $._simple_statement,
    ),

    block: $ => seq('{', repeat($._statement), '}'),

    _simple_statement: $ => seq($._simple_statement_core, optional(';')),

    _simple_statement_core: $ => choice(
      $.unpack_assignment,
      $.assignment,
      $.inc_dec_statement,
      $._expression,
    ),

    unpack_assignment: $ => seq(
      field('target', $.identifier),
      repeat1(seq(',', field('target', $.identifier))),
      '=',
      field('value', $._expression),
    ),

    assignment: $ => seq(
      field('target', $._lvalue),
      field('operator', choice('=', '+=', '-=', '*=', '/=', '%=')),
      field('value', $._expression),
    ),

    _lvalue: $ => choice($.identifier, $.index_expression),

    inc_dec_statement: $ => choice(
      seq(field('target', $._lvalue), field('operator', choice('++', '--'))),
      seq(field('operator', choice('++', '--')), field('target', $._lvalue)),
    ),

    order_statement: $ => seq(
      'order', '(', field('condition', $._expression), ')', field('consequence', $.block),
      repeat($.combo_clause),
      optional(seq('special', field('alternative', $.block))),
    ),

    combo_clause: $ => seq(
      'combo', '(', field('condition', $._expression), ')', field('body', $.block),
    ),

    knead_statement: $ => seq(
      'knead', choice($.counted_header, $.for_each_header), field('body', $.block),
    ),

    counted_header: $ => seq(
      '(',
      optional(field('init', $._simple_statement_core)), ';',
      optional(field('condition', $._expression)), ';',
      optional(field('post', $._simple_statement_core)),
      ')',
    ),

    for_each_header: $ => seq(
      field('variable', $.identifier), 'in', field('collection', $._expression),
    ),

    bake_statement: $ => seq(
      'bake', '(', field('condition', $._expression), ')', field('body', $.block),
    ),

    serve_statement: $ => seq('serve', optional(field('value', $._expression)), optional(';')),
    burnt_statement: $ => seq('burnt', optional(';')),
    flip_statement: $ => seq('flip', optional(';')),

    // --- Expressions (SPEC.md §5, §8) --------------------------------
    // Precedence, low to high: ternary < elvis < or < with < equality
    // < comparison < range < sum < product < unary < call/index.

    _expression: $ => choice(
      $.ternary_expression,
      $.elvis_expression,
      $.binary_expression,
      $.unary_expression,
      $.range_expression,
      $.call_expression,
      $.index_expression,
      $.function_literal,
      $.list_literal,
      $.map_literal,
      $.set_literal,
      $.parenthesized_expression,
      $.identifier,
      $.integer_literal,
      $.float_literal,
      $.string_literal,
      $.boolean_literal,
      $.nil_literal,
    ),

    ternary_expression: $ => prec.right(PREC.TERNARY, seq(
      field('condition', $._expression),
      '(|',
      field('consequence', $._expression),
      '|)',
      field('alternative', $._expression),
    )),

    elvis_expression: $ => prec.right(PREC.ELVIS, seq(
      field('left', $._expression), '?:', field('right', $._expression),
    )),

    binary_expression: $ => choice(
      prec.left(PREC.OR, seq(field('left', $._expression), field('operator', 'or'), field('right', $._expression))),
      prec.left(PREC.WITH, seq(field('left', $._expression), field('operator', 'with'), field('right', $._expression))),
      prec.left(PREC.EQUALITY, seq(field('left', $._expression), field('operator', choice('==', '!=')), field('right', $._expression))),
      prec.left(PREC.COMPARISON, seq(field('left', $._expression), field('operator', choice('<', '>', '<=', '>=')), field('right', $._expression))),
      prec.left(PREC.SUM, seq(field('left', $._expression), field('operator', choice('+', '-')), field('right', $._expression))),
      prec.left(PREC.PRODUCT, seq(field('left', $._expression), field('operator', choice('*', '/', '%')), field('right', $._expression))),
    ),

    unary_expression: $ => prec(PREC.UNARY, seq(
      field('operator', choice('-', 'hold')), field('operand', $._expression),
    )),

    range_expression: $ => prec.left(PREC.RANGE, seq(
      field('start', $._expression), field('operator', choice('..', '.<')), field('end', $._expression),
    )),

    call_expression: $ => prec(PREC.CALL, seq(
      field('function', $._expression), '(', optional(field('arguments', $.argument_list)), ')',
    )),

    argument_list: $ => commaSep1($._expression),

    index_expression: $ => prec(PREC.CALL, seq(
      field('object', $._expression), '[', field('index', $._expression), ']',
    )),

    parenthesized_expression: $ => seq('(', $._expression, ')'),

    // --- Functions ------------------------------------------------------

    function_literal: $ => seq(
      'recipe',
      optional(field('name', $.identifier)),
      '(', optional(field('parameters', $.parameter_list)), ')',
      field('body', $.block),
    ),

    parameter_list: $ => commaSep1($.identifier),

    // --- Collections (SPEC.md §2, §2.2) ----------------------------------

    list_literal: $ => seq('[', optional(commaSep1($._expression)), ']'),

    map_literal: $ => seq('{', optional(commaSep1($.pair)), '}'),
    pair: $ => seq(field('key', $._expression), ':', field('value', $._expression)),

    set_literal: $ => seq('toppings', '{', optional(commaSep1($._expression)), '}'),

    // --- Literals (SPEC.md §2) --------------------------------------------

    integer_literal: $ => /[0-9]+/,
    float_literal: $ => /[0-9]+\.[0-9]+/,

    string_literal: $ => seq(
      '"',
      repeat(choice($.escape_sequence, /[^"\\\n]/)),
      '"',
    ),
    escape_sequence: $ => /\\[\\"ntr]/,

    boolean_literal: $ => choice('stuffed', 'thin'),
    nil_literal: $ => 'nobox',

    identifier: $ => /[A-Za-z_][A-Za-z0-9_]*/,
  },
});
