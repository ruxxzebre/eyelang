/**
 * @file Eyelang 2 grammar for tree-sitter. This file is the definition of Eyelang's syntax;
 *       SPECS.md section 18 summarises it, and the compiler's hand-written parser in syntax/ must
 *       agree with it on every file in testdata/syntax.
 */

/// <reference types="tree-sitter-cli/dsl" />
// @ts-check

// Binding strength, loosest first. Postfix operators bind tightest.
const PREC = {
  lambda: -1,
  default: 1, // ??
  or: 2, // ||
  and: 3, // &&
  equality: 4, // == !=
  compare: 5, // < <= > >=
  pipe: 6, // |>
  add: 7, // + -
  multiply: 8, // * / %
  unary: 9, // -x !x start
  postfix: 10, // f(x) f!(x) x.y x[i] x?
};

const DURATION_UNITS = ['ms', 's', 'min', 'h', 'day', 'days', 'week', 'weeks'];
const SIZE_UNITS = ['B', 'KiB', 'MiB', 'GiB'];

// Words that can never be identifiers. Contextual keywords (command, query, event, on, migrate,
// from, after, at, uses and the unit names) are keywords only where the grammar expects them.
const KEYWORDS = [
  'let', 'var', 'fn', 'pub', 'return', 'if', 'else', 'match', 'for', 'in', 'while', 'break',
  'continue', 'type', 'trait', 'impl', 'entity', 'state', 'job', 'view', 'contract', 'import',
  'test', 'with', 'start', 'true', 'false', '_',
];

module.exports = grammar({
  name: 'eyelang',

  word: $ => $.identifier,

  // The scanner (src/scanner.c) decides which newlines end a statement, tells a block's `{` from
  // a literal's `{`, and reads the text between a string's escapes and interpolations.
  externals: $ => [
    $._newline,
    $._block_open,
    $.string_content,
    $._error_sentinel,
  ],

  extras: $ => [/\s/, $.comment],

  reserved: {
    global: _ => KEYWORDS,
  },

  supertypes: $ => [
    $._expression,
    $._pattern,
    $._type,
  ],

  conflicts: $ => [
    // `type T = Name(A)` is an alias of a generic type; `type T = Name(A) | Other` is a variant
    // list. The tokens are the same until after the `)`.
    [$.type_arguments, $.variant_payload],
  ],

  rules: {
    source_file: $ => lines($._item, $._separators),

    _item: $ => choice(
      $.import_declaration,
      $.function_declaration,
      $.job_declaration,
      $.entity_declaration,
      $.view_declaration,
      $.type_declaration,
      $.trait_declaration,
      $.impl_declaration,
      $.contract_declaration,
      $.test_declaration,
      $.let_statement,
    ),

    _separators: $ => repeat1(choice(';', $._newline)),

    comment: _ => token(seq('#', /.*/)),

    // ---------------------------------------------------------------- declarations

    import_declaration: $ => seq(
      'import',
      field('path', $.import_path),
      optional(seq('.{', commaSep1(field('item', $._name)), '}')),
    ),

    import_path: $ => seq($.identifier, repeat(seq('.', $.identifier))),

    attribute: $ => seq(
      '@',
      field('name', $.identifier),
      optional(field('arguments', $.arguments)),
    ),

    function_declaration: $ => seq(
      repeat($.attribute),
      optional('pub'),
      'fn',
      field('name', $.identifier),
      field('parameters', $.parameters),
      optional(seq('->', field('return_type', $._type))),
      optional(field('effects', $.uses_clause)),
      choice(
        seq('=', field('body', $._expression)),
        field('body', $.block),
      ),
    ),

    // A function without a body, allowed only inside a trait.
    function_signature: $ => seq(
      repeat($.attribute),
      'fn',
      field('name', $.identifier),
      field('parameters', $.parameters),
      optional(seq('->', field('return_type', $._type))),
      optional(field('effects', $.uses_clause)),
    ),

    parameters: $ => seq('(', commaSep($.parameter), ')'),

    parameter: $ => seq(
      field('name', $.identifier),
      optional(seq(':', field('type', $._type))),
      optional(seq('=', field('default', $._expression))),
    ),

    uses_clause: $ => seq('uses', '{', commaSep($.effect), '}'),

    // `wait`, `net Llm.complete`, `entity World`, `secret telegramToken`.
    effect: $ => seq(
      field('kind', choice($.identifier, alias('entity', $.identifier))),
      optional(field('target', $.path)),
    ),

    path: $ => seq($._name, repeat(seq('.', $._name))),

    job_declaration: $ => seq(
      repeat($.attribute),
      optional('pub'),
      'job',
      field('name', $.identifier),
      field('parameters', $.parameters),
      optional(seq('->', field('return_type', $._type))),
      optional(field('effects', $.uses_clause)),
      field('body', $.block),
    ),

    entity_declaration: $ => seq(
      repeat($.attribute),
      optional('pub'),
      'entity',
      field('name', $.type_identifier),
      field('parameters', $.parameters),
      field('body', $.entity_body),
    ),

    entity_body: $ => seq(
      '{',
      lines(choice($.state_declaration, $.function_declaration, $.migration), $._separators),
      '}',
    ),

    state_declaration: $ => seq(
      repeat($.attribute),
      'state',
      field('name', $.identifier),
      ':',
      field('type', $._type),
      optional(seq('=', field('default', $._expression))),
    ),

    migration: $ => seq(
      repeat($.attribute),
      'migrate',
      'from',
      field('version', $.integer),
      '(',
      field('parameter', $.identifier),
      ')',
      field('body', $.block),
    ),

    view_declaration: $ => seq(
      repeat($.attribute),
      optional('pub'),
      'view',
      field('name', $.identifier),
      '=',
      field('value', $._expression),
    ),

    type_declaration: $ => seq(
      repeat($.attribute),
      optional('pub'),
      'type',
      field('name', $.type_identifier),
      optional(field('type_parameters', $.type_parameters)),
      '=',
      field('value', choice($._type, $.variant_list)),
    ),

    type_parameters: $ => seq('(', commaSep1($.type_identifier), ')'),

    // At least one `|`: `A | B`, or `| A` for a type with a single variant.
    variant_list: $ => choice(
      seq(optional('|'), $.variant, repeat1(seq('|', $.variant))),
      seq('|', $.variant),
    ),

    variant: $ => seq(
      field('name', $.type_identifier),
      optional(field('payload', $.variant_payload)),
    ),

    variant_payload: $ => seq('(', commaSep1(choice($.field_declaration, $._type)), ')'),

    field_declaration: $ => seq(
      field('name', $.identifier),
      ':',
      field('type', $._type),
    ),

    trait_declaration: $ => seq(
      repeat($.attribute),
      optional('pub'),
      'trait',
      field('name', $.type_identifier),
      optional(field('type_parameters', $.type_parameters)),
      field('body', $.trait_body),
    ),

    trait_body: $ => seq(
      '{',
      lines(choice($.function_signature, $.function_declaration), $._separators),
      '}',
    ),

    impl_declaration: $ => seq(
      repeat($.attribute),
      'impl',
      field('trait', $._type),
      'for',
      field('type', $._type),
      field('body', $.impl_body),
    ),

    impl_body: $ => seq('{', lines($.function_declaration, $._separators), '}'),

    contract_declaration: $ => seq(
      repeat($.attribute),
      optional('pub'),
      'contract',
      field('name', $.type_identifier),
      field('body', $.contract_body),
    ),

    contract_body: $ => seq(
      '{',
      lines(choice($.command_declaration, $.query_declaration, $.event_declaration), $._separators),
      '}',
    ),

    command_declaration: $ => seq(
      repeat($.attribute),
      'command',
      field('name', $.identifier),
      field('parameters', $.parameters),
      '->',
      field('return_type', $._type),
    ),

    query_declaration: $ => seq(
      repeat($.attribute),
      'query',
      field('name', $.identifier),
      field('parameters', $.parameters),
      '->',
      field('return_type', $._type),
    ),

    event_declaration: $ => seq(
      repeat($.attribute),
      'event',
      field('name', $.identifier),
      ':',
      field('type', $._type),
      'on',
      field('topic', $.string),
    ),

    test_declaration: $ => seq(
      repeat($.attribute),
      'test',
      field('name', $.string),
      field('body', $.block),
    ),

    // ---------------------------------------------------------------- statements

    block: $ => seq(alias($._block_open, '{'), lines($._statement, $._separators), '}'),

    _statement: $ => choice(
      $.let_statement,
      $.var_statement,
      $.assignment,
      $.for_statement,
      $.while_statement,
      $.with_statement,
      $.return_statement,
      $.break_statement,
      $.continue_statement,
      $._expression,
    ),

    let_statement: $ => seq(
      'let',
      field('pattern', $._pattern),
      optional(seq(':', field('type', $._type))),
      '=',
      field('value', $._expression),
      optional(seq('else', field('alternative', $.block))),
    ),

    var_statement: $ => seq(
      'var',
      field('name', $.identifier),
      optional(seq(':', field('type', $._type))),
      '=',
      field('value', $._expression),
    ),

    assignment: $ => seq(
      field('left', $.identifier),
      '=',
      field('right', $._expression),
    ),

    for_statement: $ => seq(
      'for',
      field('pattern', $._pattern),
      'in',
      field('value', $._expression),
      field('body', $.block),
    ),

    while_statement: $ => seq(
      'while',
      field('condition', $._expression),
      field('body', $.block),
    ),

    with_statement: $ => seq('with', field('value', $._expression)),

    return_statement: $ => prec.right(seq('return', optional(field('value', $._expression)))),

    break_statement: _ => 'break',

    continue_statement: _ => 'continue',

    // ---------------------------------------------------------------- expressions

    _expression: $ => choice(
      $.identifier,
      $.type_identifier,
      $.placeholder,
      $.integer,
      $.float,
      $.string,
      $.boolean,
      $.unit,
      $.duration,
      $.size,
      $.time,
      $.date,
      $.instant,
      $.zoned_time,
      $.list,
      $.empty_braces,
      $.record,
      $.record_update,
      $.map,
      $.parenthesized_expression,
      $.lambda,
      $.if_expression,
      $.match_expression,
      $.start_expression,
      $.call_expression,
      $.waiting_call_expression,
      $.field_expression,
      $.index_expression,
      $.try_expression,
      $.unary_expression,
      $.binary_expression,
    ),

    // `_` in an argument list: `2 |> sub(10, _)` calls `sub(10, 2)`.
    placeholder: _ => '_',

    parenthesized_expression: $ => seq('(', $._expression, ')'),

    list: $ => seq('[', commaSep($._expression), ']'),

    // `{}` is an empty record, map or set; the type decides which.
    empty_braces: _ => seq('{', '}'),

    record: $ => seq('{', commaSep1(choice($.field_initializer, $.shorthand_field)), '}'),

    field_initializer: $ => seq(
      field('name', $.identifier),
      ':',
      field('value', $._expression),
    ),

    // `{ name }` means `{ name: name }`.
    shorthand_field: $ => $.identifier,

    record_update: $ => seq(
      '{',
      field('base', $._expression),
      'with',
      commaSep1($.field_initializer),
      '}',
    ),

    // Map keys are literals or parenthesised expressions, so `{ name: x }` is always a record.
    map: $ => seq('{', commaSep1($.map_entry), '}'),

    map_entry: $ => seq(
      field('key', choice($.string, $.integer, $.parenthesized_expression)),
      ':',
      field('value', $._expression),
    ),

    lambda: $ => prec.right(PREC.lambda, seq(
      'fn',
      optional(choice(
        field('parameter', $.identifier),
        field('parameters', $.parameters),
      )),
      choice(
        seq(':', field('body', $._expression)),
        field('body', $.block),
      ),
    )),

    if_expression: $ => prec.right(seq(
      'if',
      field('condition', $._expression),
      field('consequence', $.block),
      optional(seq('else', field('alternative', choice($.block, $.if_expression)))),
    )),

    match_expression: $ => seq(
      'match',
      field('value', $._expression),
      field('body', $.match_body),
    ),

    match_body: $ => seq('{', lines($.match_arm, $._arm_separators), '}'),

    _arm_separators: $ => repeat1(choice(',', $._newline)),

    // An arm body that starts with `{` is a block; wrap a record in parentheses to return one.
    match_arm: $ => seq(
      field('pattern', $._pattern),
      optional(seq('if', field('guard', $._expression))),
      '=>',
      field('value', choice($.block, $._expression)),
    ),

    // `start job(args)`, optionally `after 20 min` or `at 07:40 in "Europe/Dublin"`.
    start_expression: $ => prec.right(PREC.unary, seq(
      'start',
      field('call', $.call_expression),
      optional(seq(
        field('timing', choice('after', 'at')),
        field('time', $._expression),
      )),
    )),

    call_expression: $ => prec(PREC.postfix, seq(
      field('function', $._expression),
      field('arguments', $.arguments),
    )),

    // `f!(x)`: the `!` and `(` follow the callee with no space between them.
    waiting_call_expression: $ => prec(PREC.postfix, seq(
      field('function', $._expression),
      token.immediate('!'),
      field('arguments', alias($._waiting_arguments, $.arguments)),
    )),

    arguments: $ => seq('(', commaSep($._argument), ')'),

    _waiting_arguments: $ => seq(token.immediate('('), commaSep($._argument), ')'),

    _argument: $ => choice($._expression, $.named_argument),

    named_argument: $ => seq(
      field('name', $._label),
      ':',
      field('value', $._expression),
    ),

    // `in` and `for` are keywords, but library functions use them as argument labels:
    // `daily(07:40, in: "Europe/Dublin")`, `lock!("ci", for: 30 min, fn { … })`.
    _label: $ => choice($.identifier, alias('in', $.identifier), alias('for', $.identifier)),

    field_expression: $ => prec(PREC.postfix, seq(
      field('value', $._expression),
      '.',
      field('field', $._name),
    )),

    index_expression: $ => prec(PREC.postfix, seq(
      field('value', $._expression),
      '[',
      field('index', $._expression),
      ']',
    )),

    try_expression: $ => prec(PREC.postfix, seq(
      field('value', $._expression),
      token.immediate('?'),
    )),

    unary_expression: $ => prec(PREC.unary, seq(
      field('operator', choice('-', '!')),
      field('operand', $._expression),
    )),

    binary_expression: $ => {
      const table = [
        [prec.right, PREC.default, '??'],
        [prec.left, PREC.or, '||'],
        [prec.left, PREC.and, '&&'],
        [prec.left, PREC.equality, choice('==', '!=')],
        [prec.left, PREC.compare, choice('<', '<=', '>', '>=')],
        [prec.left, PREC.pipe, '|>'],
        [prec.left, PREC.add, choice('+', '-')],
        [prec.left, PREC.multiply, choice('*', '/', '%')],
      ];
      return choice(...table.map(([assoc, level, operator]) =>
        // @ts-ignore
        assoc(level, seq(
          field('left', $._expression),
          field('operator', operator),
          field('right', $._expression),
        )),
      ));
    },

    // ---------------------------------------------------------------- literals

    integer: _ => /[0-9](_?[0-9])*/,

    float: _ => /[0-9](_?[0-9])*\.[0-9](_?[0-9])*([eE][+-]?[0-9]+)?/,

    boolean: _ => choice('true', 'false'),

    unit: _ => seq('(', ')'),

    duration: $ => seq(
      field('value', choice($.integer, $.float)),
      field('unit', choice(...DURATION_UNITS.map(u => alias(u, $.unit_name)))),
    ),

    size: $ => seq(
      field('value', choice($.integer, $.float)),
      field('unit', choice(...SIZE_UNITS.map(u => alias(u, $.unit_name)))),
    ),

    // Written without spaces; `2026 - 10 - 10` is subtraction.
    date: _ => /[0-9]{4}-[0-9]{2}-[0-9]{2}/,

    time: _ => /[0-9]{2}:[0-9]{2}(:[0-9]{2})?/,

    instant: $ => seq(
      field('date', $.date),
      field('time', $.time),
      'in',
      field('zone', $.string),
    ),

    // The next time the clock in that zone shows this time: `07:45 in "Europe/Dublin"`.
    zoned_time: $ => seq(
      field('time', $.time),
      'in',
      field('zone', $.string),
    ),

    string: $ => seq(
      '"',
      repeat(choice($.string_content, $.escape_sequence, $.interpolation)),
      token.immediate('"'),
    ),

    escape_sequence: _ => token.immediate(/\\([nrt0\\"{}]|u\{[0-9a-fA-F]{1,6}\})/),

    interpolation: $ => seq(token.immediate('{'), field('value', $._expression), '}'),

    // ---------------------------------------------------------------- patterns

    _pattern: $ => choice(
      $.identifier,
      $.wildcard,
      $.integer,
      $.float,
      $.string,
      $.boolean,
      $.unit,
      $.negative_literal,
      $.variant_pattern,
      $.record_pattern,
      $.list_pattern,
      $.or_pattern,
    ),

    wildcard: _ => '_',

    negative_literal: $ => seq('-', choice($.integer, $.float)),

    variant_pattern: $ => seq(
      field('name', $.type_identifier),
      optional(field('arguments', $.pattern_arguments)),
    ),

    pattern_arguments: $ => seq(
      '(',
      commaSep(choice($._pattern, $.field_pattern, $.rest_pattern)),
      ')',
    ),

    field_pattern: $ => seq(
      field('name', $.identifier),
      ':',
      field('pattern', $._pattern),
    ),

    record_pattern: $ => seq(
      '{',
      commaSep(choice($.field_pattern, $.shorthand_field_pattern, $.rest_pattern)),
      '}',
    ),

    shorthand_field_pattern: $ => $.identifier,

    list_pattern: $ => seq('[', commaSep(choice($._pattern, $.rest_pattern)), ']'),

    // `..` skips the remaining fields or elements; `..rest` binds the remaining elements.
    rest_pattern: $ => seq('..', optional(field('name', $.identifier))),

    or_pattern: $ => prec.left(seq(
      field('left', $._pattern),
      '|',
      field('right', $._pattern),
    )),

    // ---------------------------------------------------------------- types

    _type: $ => choice(
      $.type_identifier,
      $.generic_type,
      $.qualified_type,
      $.unit_type,
      $.record_type,
      $.function_type,
    ),

    generic_type: $ => seq(
      field('name', choice($.type_identifier, $.qualified_type)),
      field('arguments', $.type_arguments),
    ),

    type_arguments: $ => seq('(', commaSep1($._type), ')'),

    // A type from an imported module: `notify.Delivery`.
    qualified_type: $ => seq(
      field('module', $.identifier),
      '.',
      field('name', $.type_identifier),
    ),

    unit_type: _ => seq('(', ')'),

    record_type: $ => seq('{', commaSep($.field_declaration), '}'),

    function_type: $ => prec.right(seq(
      'fn',
      field('parameters', $.function_type_parameters),
      optional(seq('->', field('return_type', $._type))),
    )),

    function_type_parameters: $ => seq('(', commaSep($._type), ')'),

    // ---------------------------------------------------------------- names

    _name: $ => choice($.identifier, $.type_identifier),

    identifier: _ => /[a-z_][a-zA-Z0-9_]*/,

    // Names that start with an upper-case letter name types, variants, entities and contracts.
    type_identifier: _ => /[A-Z][a-zA-Z0-9_]*/,
  },
});

/**
 * Items separated by `separator`, with any number of separators before, between and after them.
 *
 * @param {RuleOrLiteral} rule
 * @param {RuleOrLiteral} separator
 * @returns {Rule}
 */
function lines(rule, separator) {
  return seq(
    optional(separator),
    optional(seq(rule, repeat(seq(separator, rule)), optional(separator))),
  );
}

/**
 * One or more `rule`s separated by commas, with an optional trailing comma.
 *
 * @param {RuleOrLiteral} rule
 * @returns {Rule}
 */
function commaSep1(rule) {
  return seq(rule, repeat(seq(',', rule)), optional(','));
}

/**
 * Zero or more `rule`s separated by commas, with an optional trailing comma.
 *
 * @param {RuleOrLiteral} rule
 * @returns {Rule}
 */
function commaSep(rule) {
  return optional(commaSep1(rule));
}
