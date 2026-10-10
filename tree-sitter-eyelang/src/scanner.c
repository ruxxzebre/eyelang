// External scanner for the Eyelang grammar.
//
// It produces three tokens that a context-free lexer can't:
//
//   _newline        a line break that ends a statement. A line break is only a separator where
//                   the parser could end a statement, member or match arm, and only when the next
//                   line doesn't continue the expression (see continues_expression).
//   _block_open     the `{` that opens a block. Where both a block and an expression are allowed
//                   (a match arm's body), a `{` opens a block unless it is followed by `name:`,
//                   `name,` or `name with`, which only a record can start (see starts_record).
//   string_content  the text of a string between its quotes, escapes and interpolations.
//
// syntax/lexer.go applies the same rules, and testdata/syntax checks that the two agree.

#include "tree_sitter/parser.h"

#include <stdbool.h>
#include <stdint.h>

enum TokenType {
  NEWLINE,
  BLOCK_OPEN,
  STRING_CONTENT,
  ERROR_SENTINEL,
};

void *tree_sitter_eyelang_external_scanner_create(void) { return NULL; }

void tree_sitter_eyelang_external_scanner_destroy(void *payload) { (void)payload; }

unsigned tree_sitter_eyelang_external_scanner_serialize(void *payload, char *buffer) {
  (void)payload;
  (void)buffer;
  return 0;
}

void tree_sitter_eyelang_external_scanner_deserialize(void *payload, const char *buffer,
                                                      unsigned length) {
  (void)payload;
  (void)buffer;
  (void)length;
}

static void advance(TSLexer *lexer) { lexer->advance(lexer, false); }

static void skip(TSLexer *lexer) { lexer->advance(lexer, true); }

static bool is_space(int32_t c) {
  return c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\f' || c == '\v';
}

static bool is_horizontal_space(int32_t c) {
  return c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v';
}

static bool is_identifier_char(int32_t c) {
  return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9');
}

// Reports whether the next line, ignoring blank lines and comment lines, starts with a token that
// can only continue an expression: a binary operator, `.`, `?`, `,`, `:`, `=`, `->`, an assignment
// operator or `else`.
// The lexer is only peeking here; the token has already been marked.
static bool continues_expression(TSLexer *lexer) {
  for (;;) {
    while (is_space(lexer->lookahead)) {
      advance(lexer);
    }
    if (lexer->lookahead != '#') {
      break;
    }
    while (lexer->lookahead != '\n' && !lexer->eof(lexer)) {
      advance(lexer);
    }
  }
  if (lexer->eof(lexer)) {
    return false;
  }

  switch (lexer->lookahead) {
    case '.':
    case ',':
    case ':':
    case '=':
    case '?':
    case '|':
    case '&':
    case '+':
    case '*':
    case '/':
    case '%':
    case '<':
    case '>':
      return true;
    case '!':
      // `!=` continues; `!x` starts a new statement.
      advance(lexer);
      return lexer->lookahead == '=';
    case '-':
      // `->` and `-=` continue; `-x` starts a new statement.
      advance(lexer);
      return lexer->lookahead == '>' || lexer->lookahead == '=';
    case 'e': {
      static const char rest[] = "lse";
      advance(lexer);
      for (const char *c = rest; *c; c++) {
        if (lexer->lookahead != *c) {
          return false;
        }
        advance(lexer);
      }
      return !is_identifier_char(lexer->lookahead);
    }
    default:
      return false;
  }
}

static bool scan_string_content(TSLexer *lexer) {
  bool has_content = false;
  for (;;) {
    int32_t c = lexer->lookahead;
    if (c == '"' || c == '\\' || c == '{' || c == '\n' || lexer->eof(lexer)) {
      break;
    }
    advance(lexer);
    has_content = true;
  }
  lexer->mark_end(lexer);
  lexer->result_symbol = STRING_CONTENT;
  return has_content;
}

// skip_space_and_comments moves past whitespace, line breaks and comments.
static void skip_space_and_comments(TSLexer *lexer) {
  for (;;) {
    while (is_space(lexer->lookahead)) {
      advance(lexer);
    }
    if (lexer->lookahead != '#') {
      return;
    }
    while (lexer->lookahead != '\n' && !lexer->eof(lexer)) {
      advance(lexer);
    }
  }
}

static bool is_name_start(int32_t c) { return (c >= 'a' && c <= 'z') || c == '_'; }

static bool is_name_char(int32_t c) {
  return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_';
}

// starts_record reports whether the text after a `{` begins a record: `name:`, `name,` or
// `name with`. No statement starts that way, so the `{` can't open a block. The lexer has already
// marked the end of the token, so looking further ahead doesn't consume anything.
static bool starts_record(TSLexer *lexer) {
  skip_space_and_comments(lexer);
  if (!is_name_start(lexer->lookahead)) {
    return false;
  }
  while (is_name_char(lexer->lookahead)) {
    advance(lexer);
  }
  skip_space_and_comments(lexer);
  if (lexer->lookahead == ':' || lexer->lookahead == ',') {
    return true;
  }
  const char *with = "with";
  for (int i = 0; with[i]; i++) {
    if (lexer->lookahead != with[i]) {
      return false;
    }
    advance(lexer);
  }
  return !is_name_char(lexer->lookahead);
}

bool tree_sitter_eyelang_external_scanner_scan(void *payload, TSLexer *lexer,
                                               const bool *valid_symbols) {
  (void)payload;

  // During error recovery every symbol is valid; let the internal lexer handle the input.
  if (valid_symbols[ERROR_SENTINEL]) {
    return false;
  }

  if (valid_symbols[STRING_CONTENT]) {
    return scan_string_content(lexer);
  }

  while (is_horizontal_space(lexer->lookahead)) {
    skip(lexer);
  }

  if (valid_symbols[NEWLINE] && lexer->lookahead == '\n') {
    advance(lexer);
    lexer->mark_end(lexer);
    lexer->result_symbol = NEWLINE;
    // A continued line is ordinary whitespace, which the internal lexer skips.
    return !continues_expression(lexer);
  }

  if (valid_symbols[BLOCK_OPEN]) {
    while (is_space(lexer->lookahead)) {
      skip(lexer);
    }
    if (lexer->lookahead == '{') {
      advance(lexer);
      lexer->mark_end(lexer);
      lexer->result_symbol = BLOCK_OPEN;
      // Returning false lets the internal lexer read the `{` as a literal's.
      return !starts_record(lexer);
    }
  }

  return false;
}
