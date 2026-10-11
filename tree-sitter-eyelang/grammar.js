/**
 * @file Eyelang 2 grammar for tree-sitter. This file is the definition of Eyelang's syntax;
 *       SPECS.md section 23 summarises it, and the compiler's hand-written parser in syntax/ must
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
  range: 6, // .. ..=
  pipe: 7, // |>
  add: 8, // + -
  multiply: 9, // * / %
  unary: 10, // -x !x
  postfix: 11, // f(x) f!(x) x.y x[i] x?
};

const DURATION_UNITS = ['ms', 's', 'min', 'h', 'day', 'days', 'week', 'weeks'];
const SIZE_UNITS = ['B', 'KiB', 'MiB', 'GiB'];

// Words that can never be names. Every other keyword (type, entity, state, job, view, test, with,
// where, dyn, extern, opaque, migrate, uses, command, … and the unit names) is a keyword only
// where the grammar expects it.
const RESERVED = [
  'let', 'var', 'mut', 'fn', 'pub', 'return', 'if', 'else', 'match', 'for', 'in', 'while',
  'break', 'continue', 'true', 'false', '_',
];

const ASSIGNMENT_OPERATORS = ['=', '+=', '-=', '*=', '/=', '%='];

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
    $._multiline_string_content,
  ],

  // ASCII whitespace only, as in the scanner and syntax/lexer.go.
  extras: $ => [/[ \t\r\n\f\v]/, $.comment],

  reserved: {
    global: _ => RESERVED,
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
      $.extern_declaration,
      $.extern_type_declaration,
      $.job_declaration,
      $.entity_declaration,
      $.view_declaration,
      $.type_declaration,
      $.trait_declaration,
      $.impl_declaration,
      $.contract_declaration,
      $.test_declaration,
      $.let_statement,
      // `pub let` exports a module-level value.
      alias($._pub_let, $.let_statement),
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
      optional(field('requirements', $.where_clause)),
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
      optional(field('requirements', $.where_clause)),
    ),

    // A function implemented by C or by the Go program that embeds Eyelang.
    extern_declaration: $ => seq(
      repeat($.attribute),
      optional('pub'),
      'extern',
      'fn',
      field('name', $.identifier),
      field('parameters', $.parameters),
      optional(seq('->', field('return_type', $._type))),
      optional(field('effects', $.uses_clause)),
      optional(field('requirements', $.where_clause)),
    ),

    // A type whose values foreign code owns: only extern functions make or take one.
    extern_type_declaration: $ => seq(
      repeat($.attribute),
      optional('pub'),
      'extern',
      'type',
      field('name', $.type_identifier),
      optional(field('type_parameters', $.type_parameters)),
    ),

    parameters: $ => seq('(', commaSep($.parameter), ')'),

    // `mut` lets the function change the caller's variable; `_` ignores an argument.
    parameter: $ => seq(
      optional('mut'),
      field('name', choice($.identifier, $.wildcard)),
      optional(seq(':', field('type', $._type))),
      optional(seq('=', field('default', $._expression))),
    ),

    uses_clause: $ => seq('uses', '{', commaSep($.effect), '}'),

    // `wait`, `net Llm.complete`, `entity World`, `secret telegramToken`.
    effect: $ => seq(
      field('kind', $.identifier),
      optional(field('target', $.path)),
    ),

    path: $ => seq($._name, repeat(seq('.', $._name))),

    // `where a: Ord + Show, b: Hash`.
    where_clause: $ => seq('where', commaSep1($.requirement)),

    requirement: $ => seq(
      field('name', $.identifier),
      ':',
      field('trait', $._type),
      repeat(seq('+', field('trait', $._type))),
    ),

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

    // `opaque` hides a public type's fields from other modules.
    type_declaration: $ => seq(
      repeat($.attribute),
      optional(seq('pub', optional('opaque'))),
      'type',
      field('name', $.type_identifier),
      optional(field('type_parameters', $.type_parameters)),
      '=',
      field('value', choice($._type, $.variant_list)),
    ),

    type_parameters: $ => seq('(', commaSep1($.identifier), ')'),

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

    // `impl Named for Player { … }` implements a trait; `impl Lexer { … }` gives a type methods.
    impl_declaration: $ => seq(
      repeat($.attribute),
      'impl',
      choice(
        seq(field('trait', $._type), 'for', field('type', $._type)),
        field('type', $._type),
      ),
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
      $._expression,
    ),

    let_statement: $ => seq('let', ...letRest($)),

    _pub_let: $ => seq('pub', 'let', ...letRest($)),

    var_statement: $ => seq(
      'var',
      field('name', $.identifier),
      optional(seq(':', field('type', $._type))),
      '=',
      field('value', $._expression),
    ),

    // `x = 1`, `c.hits += 1`. The checker makes sure the left side is a variable or a field
    // of one.
    assignment: $ => seq(
      field('left', $._expression),
      field('operator', choice(...ASSIGNMENT_OPERATORS)),
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

    // `return`, `break` and `continue` leave a function or loop, so they can stand wherever a
    // value is expected: `if c then return x`, `None => continue`.
    return_statement: $ => prec.right(PREC.lambda, seq('return', optional(field('value', $._expression)))),

    break_statement: _ => 'break',

    continue_statement: _ => 'continue',

    // ---------------------------------------------------------------- expressions

    _expression: $ => choice(
      $.identifier,
      $.type_identifier,
      $.placeholder,
      $.integer,
      $.float,
      $.char,
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
      $.tuple,
      $.empty_braces,
      $.record,
      $.record_update,
      $.map,
      $.parenthesized_expression,
      $.lambda,
      $.if_expression,
      $.match_expression,
      $.call_expression,
      $.waiting_call_expression,
      $.field_expression,
      $.index_expression,
      $.try_expression,
      $.unary_expression,
      $.binary_expression,
      $.range_expression,
      $.return_statement,
      $.break_statement,
      $.continue_statement,
    ),

    // `_` in an argument list: `2 |> sub(10, _)` calls `sub(10, 2)`.
    placeholder: _ => '_',

    parenthesized_expression: $ => seq('(', $._expression, ')'),

    tuple: $ => seq('(', $._expression, repeat1(seq(',', $._expression)), optional(','), ')'),

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
      field('key', choice($.string, $.integer, $.date, $.time, $.parenthesized_expression)),
      ':',
      field('value', $._expression),
    ),

    lambda: $ => prec.right(PREC.lambda, seq(
      'fn',
      optional(choice(
        field('parameter', choice($.identifier, $.wildcard)),
        field('parameters', $.parameters),
      )),
      choice(
        seq(':', field('body', $._expression)),
        field('body', $.block),
      ),
    )),

    // `if c { … } else { … }` takes blocks; `if c then a else b` takes expressions, so a branch
    // that is one expression needs no braces. `then` is a keyword only here.
    if_expression: $ => prec.right(seq(
      'if',
      field('condition', $._expression),
      choice(
        seq(
          field('consequence', $.block),
          optional(seq('else', field('alternative', choice($.block, $.if_expression)))),
        ),
        seq(
          'then',
          field('consequence', $._branch),
          optional(seq('else', field('alternative', $._branch))),
        ),
      ),
    )),

    match_expression: $ => seq(
      'match',
      field('value', $._expression),
      field('body', $.match_body),
    ),

    match_body: $ => seq('{', lines($.match_arm, $._arm_separators), '}'),

    _arm_separators: $ => repeat1(choice(',', $._newline)),

    // An arm body that starts with `{` is a block, unless the `{` is followed by `name:`, `name,` or
    // `name with`, which only a record can start (src/scanner.c looks ahead).
    match_arm: $ => seq(
      field('pattern', $._pattern),
      optional(seq('if', field('guard', $._expression))),
      '=>',
      field('value', choice($.block, $._branch)),
    ),

    // A branch that is one expression or one assignment needs no braces:
    // `Ok(text) => pages += [text]`, `if late then count += 1`.
    _branch: $ => choice($._expression, alias($._branch_assignment, $.assignment)),

    _branch_assignment: $ => prec.right(PREC.default, seq(
      field('left', $._expression),
      field('operator', choice(...ASSIGNMENT_OPERATORS)),
      field('right', $._expression),
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

    _argument: $ => choice($._expression, $.mut_argument, $.named_argument),

    // `mut x` passes a variable the call may change.
    mut_argument: $ => seq('mut', field('value', $._expression)),

    // `key:` alone passes the variable of that name: `send(key:)` is `send(key: key)`.
    named_argument: $ => seq(
      field('name', $._label),
      ':',
      optional(field('value', choice($._expression, $.mut_argument))),
    ),

    // `in` and `for` are keywords, but library functions use them as argument labels:
    // `daily(07:40, in: "Europe/Dublin")`, `lock!("ci", for: 30 min, fn { … })`.
    _label: $ => choice($.identifier, alias('in', $.identifier), alias('for', $.identifier)),

    field_expression: $ => prec(PREC.postfix, seq(
      field('value', $._expression),
      '.',
      field('field', $._name),
    )),

    // `xs[i]`, and slices `xs[a..b]`, `xs[..n]`, `xs[i..]`.
    index_expression: $ => prec(PREC.postfix, seq(
      field('value', $._expression),
      '[',
      field('index', choice($._expression, alias($._open_range, $.range_expression))),
      ']',
    )),

    _open_range: $ => choice(
      seq(field('operator', choice('..', '..=')), field('end', $._expression)),
      seq(field('start', $._expression), field('operator', '..')),
    ),

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

    // `0..n` stops before n; `0..=n` includes it.
    range_expression: $ => prec.left(PREC.range, seq(
      field('start', $._expression),
      field('operator', choice('..', '..=')),
      field('end', $._expression),
    )),

    // ---------------------------------------------------------------- literals

    integer: _ => /[0-9](_?[0-9])*/,

    float: _ => /[0-9](_?[0-9])*\.[0-9](_?[0-9])*([eE][+-]?[0-9]+)?/,

    // One Unicode scalar value: 'a', '\n', '\u{1F600}'.
    char: _ => /'([^'\\\n]|\\([nrt0\\'"{}]|u\{[0-9a-fA-F]{1,6}\}))'/,

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
      field('zone', $._zone),
    ),

    // The next time the clock in that zone shows this time: `07:45 in "Europe/Dublin"`.
    zoned_time: $ => seq(
      field('time', $.time),
      'in',
      field('zone', $._zone),
    ),

    // A zone's name, or a name for a Zone value: `in "Europe/Dublin"`, `in dublin`.
    _zone: $ => choice($.string, $.identifier),

    // A string stays on one line. A multi-line string starts with `"""` and a line break and ends
    // with `"""` on a line of its own; the checker strips the closing line's indentation from
    // every line.
    string: $ => choice(
      seq(
        '"',
        repeat(choice($.string_content, $.escape_sequence, $.interpolation)),
        token.immediate('"'),
      ),
      seq(
        '"""',
        repeat(choice(
          alias($._multiline_string_content, $.string_content),
          $.escape_sequence,
          $.interpolation,
        )),
        token.immediate('"""'),
      ),
    ),

    escape_sequence: _ => token.immediate(/\\([nrt0\\"{}]|u\{[0-9a-fA-F]{1,6}\})/),

    interpolation: $ => seq(token.immediate('{'), field('value', $._expression), '}'),

    // ---------------------------------------------------------------- patterns

    _pattern: $ => choice(
      $.identifier,
      $.wildcard,
      $.integer,
      $.float,
      $.char,
      $.string,
      $.boolean,
      $.unit,
      $.negative_literal,
      $.range_pattern,
      $.variant_pattern,
      $.tuple_pattern,
      $.record_pattern,
      $.list_pattern,
      $.or_pattern,
    ),

    wildcard: _ => '_',

    negative_literal: $ => seq('-', choice($.integer, $.float)),

    // `'a'..='z'`, `0..10`.
    range_pattern: $ => seq(
      field('start', $._range_bound),
      field('operator', choice('..', '..=')),
      field('end', $._range_bound),
    ),

    _range_bound: $ => choice($.integer, $.char, $.negative_literal),

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

    tuple_pattern: $ => seq('(', $._pattern, repeat1(seq(',', $._pattern)), optional(','), ')'),

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
      $.type_variable,
      $.generic_type,
      $.qualified_type,
      $.unit_type,
      $.tuple_type,
      $.record_type,
      $.function_type,
      $.dyn_type,
    ),

    // A lower-case name in a type: `List(a)`.
    type_variable: $ => $.identifier,

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

    tuple_type: $ => seq('(', $._type, repeat1(seq(',', $._type)), optional(','), ')'),

    record_type: $ => seq('{', commaSep($.field_declaration), '}'),

    // A function type without `uses` is pure: `fn(Event) uses {net Notify.send}` may call Notify.
    function_type: $ => prec.right(seq(
      'fn',
      field('parameters', $.function_type_parameters),
      optional(seq('->', field('return_type', $._type))),
      optional(field('effects', $.uses_clause)),
    )),

    function_type_parameters: $ => seq('(', commaSep($._type), ')'),

    // A value of some type that implements the trait, chosen at run time.
    dyn_type: $ => seq('dyn', field('trait', $._type)),

    // ---------------------------------------------------------------- names

    _name: $ => choice($.identifier, $.type_identifier),

    identifier: _ => /[a-z_][a-zA-Z0-9_]*/,

    // Names that start with an upper-case letter name types, variants, entities and contracts.
    type_identifier: _ => /[A-Z][a-zA-Z0-9_]*/,
  },
});

/**
 * What follows `let` (or `pub let`): `pattern: T = value else { … }`.
 *
 * @param {GrammarSymbols<string>} $
 * @returns {RuleOrLiteral[]}
 */
function letRest($) {
  return [
    field('pattern', $._pattern),
    optional(seq(':', field('type', $._type))),
    '=',
    field('value', $._expression),
    optional(seq('else', field('alternative', $.block))),
  ];
}

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
