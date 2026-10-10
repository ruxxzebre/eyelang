// External scanner for the Eyelang grammar.
//
// It produces three tokens that a context-free lexer can't:
//
//   _newline        a line break that ends a statement. A line break is only a separator where
//                   the parser could end a statement, member or match arm, and only when the next
//                   line doesn't continue the expression (see continues_expression).
//   _block_open     the `{` that opens a block. Where both a block and an expression are allowed
//                   (a match arm's body), a `{` opens a block, never a record or map literal.
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
      return true;
    }
  }

  return false;
}
