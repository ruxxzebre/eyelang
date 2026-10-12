## Eyelang's lexer, written in Eyelang. It splits a file's bytes into tokens,
## trivia included, exactly as `syntax/lexer.go` does in Go: a test in
## `interp` runs both on the conformance suite and the tree-sitter corpus and
## compares every token, its span and what comes before it.
##
## Lexing never stops at an error. A character that no token starts with, or a
## bad escape, is an `Illegal` token, and a string left open gets a zero-width
## `StringEnd` marked missing. The longest match wins, as in the tree-sitter
## grammar's lexer: `2026-10-10` is a date and `2026 - 10` a subtraction,
## `07:40` is a time, and `!=` is one token. Which line breaks end a statement
## is for the parser to decide; the lexer only says where they are.

## The tokens of src, ending with an `EOF` token.
pub fn lex(src: Bytes) -> List(Token) {
  var lexer = Lexer.new(src)
  var tokens: List(Token) = []
  while true {
    let token = lexer.next()
    tokens += [token]
    if token.kind == EOF then return tokens
  }
}

## A position in a source, and the strings open there.
pub opaque type Lexer = {
  src: Bytes,
  pos: Int,
  ## The strings being lexed, innermost last.
  strings: List(OpenString),
  ## Whitespace or a comment has come since the last token that isn't trivia.
  space: Bool,
  ## So has a line break.
  newline: Bool,
}

## A string being lexed: the lexer is either in its text or in one of its
## interpolations.
type OpenString = {
  ## Opened with `"""`, so it may span lines.
  multiline: Bool,
  ## In an interpolation, `{…}`, rather than in the string's text.
  interp: Bool,
  ## How many `{` are open in the interpolation, so that the `}` that ends
  ## it can be told from the end of a record.
  depth: Int,
}

impl Lexer {
  ## A lexer at the start of src.
  pub fn new(src: Bytes) -> Lexer = { src, pos: 0, strings: [], space: false, newline: false }

  ## The token at the lexer's position, which moves past it. At the end of the
  ## source it closes the strings left open, and then gives `EOF` each time.
  pub fn next(mut self) -> Token = match self.strings.last() {
    Some(s) if !s.interp => self.inString(s)
    Some(s) if self.pos >= self.src.size() => {
      # The source ends in an interpolation: close it, and then its string.
      self.replaceString({ s with interp: false })
      self.missing(InterpClose)
    }
    _ if self.pos >= self.src.size() => self.take(EOF, self.pos)
    _ => self.inCode()
  }

  ## A token outside the text of strings.
  fn inCode(mut self) -> Token {
    let src = self.src
    let start = self.pos
    match src[start..start + 3] {
      [c, ..] if isSpace(c) => self.whitespace()
      ['#', ..] => self.trivia(Comment, skip(src, start, fn c: c != '\n'))
      ['a'..='z' | '_', ..] => self.name()
      ['A'..='Z', ..] => self.take(TypeIdent, skip(src, start, isNameByte))
      ['0'..='9', ..] => self.number()
      ['"', '"', '"'] => self.openString(multiline: true)
      ['"', ..] => self.openString(multiline: false)
      ['\'', ..] => self.char()
      ['{', ..] => self.openBrace()
      ['}', ..] => self.closeBrace()
      next => match punctuation(next) {
        Some((kind, width)) => self.take(kind, start + width)
        None => self.take(Illegal, start + runeSize(src[start..start + 4]))
      }
    }
  }

  ## The token of this kind from the lexer's position up to end, which moves
  ## the position to end.
  fn take(mut self, kind: Kind, end: Int) -> Token {
    let token: Token = {
      kind,
      start: self.pos,
      end,
      spaceBefore: self.space,
      newlineBefore: self.newline,
      missing: false,
    }
    self.pos = end
    self.space = false
    self.newline = false
    token
  }

  ## A zero-width token that the source leaves out, to close a string.
  fn missing(mut self, kind: Kind) -> Token = { self.take(kind, self.pos) with missing: true }

  ## Whitespace or a comment up to end. The next token that isn't trivia has
  ## space before it.
  fn trivia(mut self, kind: Kind, end: Int) -> Token {
    let token: Token = {
      kind,
      start: self.pos,
      end,
      spaceBefore: false,
      newlineBefore: false,
      missing: false,
    }
    self.pos = end
    self.space = true
    token
  }

  ## Spaces, tabs and line breaks.
  fn whitespace(mut self) -> Token {
    let end = skip(self.src, self.pos, isSpace)
    if self.src[self.pos..end] |> contains('\n') then self.newline = true
    self.trivia(Whitespace, end)
  }

  ## A name, or the reserved word it spells.
  fn name(mut self) -> Token {
    let end = skip(self.src, self.pos, isNameByte)
    self.take(keyword(self.src[self.pos..end]) ?? Ident, end)
  }

  ## An integer, float, date or time, whichever is longest:
  ##
  ##     integer  [0-9](_?[0-9])*
  ##     float    integer "." [0-9](_?[0-9])* ([eE][+-]?[0-9]+)?
  ##     date     [0-9]{4} "-" [0-9]{2} "-" [0-9]{2}
  ##     time     [0-9]{2} ":" [0-9]{2} (":" [0-9]{2})?
  fn number(mut self) -> Token {
    let src = self.src
    let start = self.pos
    if isDate(src[start..start + 10]) then return self.take(DateLiteral, start + 10)
    let time = timeLength(src[start..start + 8])
    if time > 0 then return self.take(TimeLiteral, start + time)
    let end = digits(src, start)
    match src[end..end + 2] {
      ['.', '0'..='9'] => self.take(FloatLiteral, exponent(src, digits(src, end + 1)))
      _ => self.take(IntLiteral, end)
    }
  }

  ## A character literal: one character or escape between single quotes.
  ## Anything else, up to the next quote on the line, is one Illegal token.
  fn char(mut self) -> Token {
    let src = self.src
    let start = self.pos
    match charEnd(src, start + 1) {
      Some(end) if src[end] == Some('\'') => self.take(CharLiteral, end + 1)
      _ => {
        let end = skip(src, start + 1, fn c: c != '\n' && c != '\'')
        self.take(Illegal, if src[end] == Some('\'') then end + 1 else end)
      }
    }
  }

  ## The opening quote of a string, `"` or `"""`.
  fn openString(mut self, multiline: Bool) -> Token {
    self.strings += [{ multiline, interp: false, depth: 0 }]
    self.take(StringStart, self.pos + (if multiline then 3 else 1))
  }

  ## A token of a string's text: text, an escape, the start of an
  ## interpolation, or the closing quote. A string's text stays on one line,
  ## unless it opened with `"""`; only `"""` ends that one.
  fn inString(mut self, s: OpenString) -> Token {
    let start = self.pos
    match self.src[start..start + 3] {
      [] => self.endString(width: 0)
      ['\n', ..] if !s.multiline => self.endString(width: 0)
      ['"', '"', '"'] if s.multiline => self.endString(width: 3)
      ['"', ..] if !s.multiline => self.endString(width: 1)
      ['{', ..] => {
        self.replaceString({ s with interp: true, depth: 0 })
        self.take(InterpOpen, start + 1)
      }
      ['\\', ..] => self.escape()
      _ => self.take(StringText, textEnd(self.src, start, s.multiline))
    }
  }

  ## The end of the innermost string: its closing quote, width bytes long, or
  ## a missing one when the string isn't closed.
  fn endString(mut self, width: Int) -> Token {
    self.strings = self.strings |> take(self.strings.size() - 1)
    if width == 0 then self.missing(StringEnd) else self.take(StringEnd, self.pos + width)
  }

  ## An escape in a string. A backslash that starts none is an Illegal token,
  ## with the character after it unless that ends the line.
  fn escape(mut self) -> Token {
    let start = self.pos
    match escapeEnd(self.src, start) {
      Some(end) => self.take(StringEscape, end)
      None => match self.src[start + 1..start + 5] {
        [] | ['\n', ..] => self.take(Illegal, start + 1)
        next => self.take(Illegal, start + 1 + runeSize(next))
      }
    }
  }

  ## A `{`, which in an interpolation must be closed before the
  ## interpolation is.
  fn openBrace(mut self) -> Token {
    match self.strings.last() {
      Some(s) => self.replaceString({ s with depth: s.depth + 1 })
      None => ()
    }
    self.take(LBrace, self.pos + 1)
  }

  ## A `}`, or the end of an interpolation when no `{` is open in it.
  fn closeBrace(mut self) -> Token = match self.strings.last() {
    Some(s) if s.depth == 0 => {
      self.replaceString({ s with interp: false })
      self.take(InterpClose, self.pos + 1)
    }
    Some(s) => {
      self.replaceString({ s with depth: s.depth - 1 })
      self.take(RBrace, self.pos + 1)
    }
    None => self.take(RBrace, self.pos + 1)
  }

  ## Puts s in place of the innermost string.
  fn replaceString(mut self, s: OpenString) {
    self.strings = (self.strings |> take(self.strings.size() - 1)) + [s]
  }
}

## The first position from `from` on whose byte keep rejects, or the end of src.
fn skip(src: Bytes, from: Int, keep: fn(Byte) -> Bool) -> Int {
  var i = from
  while i < src.size() && keep(src[i] ?? 0) { i += 1 }
  i
}

## Where the digits that start at from end: `[0-9](_?[0-9])*`.
fn digits(src: Bytes, from: Int) -> Int {
  var i = from + 1
  while true {
    match src[i..i + 2] {
      ['0'..='9', ..] => i += 1
      ['_', '0'..='9'] => i += 2
      _ => return i
    }
  }
}

## Where a float's exponent, `[eE][+-]?[0-9]+`, ends if one starts at `at`;
## otherwise `at`.
fn exponent(src: Bytes, at: Int) -> Int = match src[at..at + 3] {
  ['e' | 'E', '+' | '-', '0'..='9'] => skip(src, at + 2, isDigit)
  ['e' | 'E', '0'..='9', ..] => skip(src, at + 1, isDigit)
  _ => at
}

## Whether the bytes start with a date: `2026-10-10`.
fn isDate(next: Bytes) -> Bool = match next {
  [y1, y2, y3, y4, '-', m1, m2, '-', d1, d2, ..] => [y1, y2, y3, y4, m1, m2, d1, d2] |> all(isDigit)
  _ => false
}

## How long the time of day that the bytes start with is, `07:40` or
## `23:59:30`; 0 when they start with none.
fn timeLength(next: Bytes) -> Int = match next {
  [h1, h2, ':', m1, m2, ':', s1, s2, ..] if [h1, h2, m1, m2, s1, s2] |> all(isDigit) => 8
  [h1, h2, ':', m1, m2, ..] if [h1, h2, m1, m2] |> all(isDigit) => 5
  _ => 0
}

## Where the character or escape of a character literal that starts at `at`
## ends; `None` at a quote, a line break or the end of the source.
fn charEnd(src: Bytes, at: Int) -> Option(Int) = match src[at..at + 2] {
  ['\\', '\''] => Some(at + 2)
  ['\\', ..] => escapeEnd(src, at)
  [] | ['\'', ..] | ['\n', ..] => None
  _ => Some(at + runeSize(src[at..at + 4]))
}

## Where the escape that starts at `at` ends: `\n \r \t \0 \\ \" \{ \}`, or
## `\u{…}` with 1 to 6 hex digits. `None` when the backslash starts none.
fn escapeEnd(src: Bytes, at: Int) -> Option(Int) = match src[at..at + 3] {
  ['\\', 'n' | 'r' | 't' | '0' | '\\' | '"' | '{' | '}', ..] => Some(at + 2)
  ['\\', 'u', '{'] => {
    let end = min(skip(src, at + 3, isHex), at + 9)
    if end > at + 3 && src[end] == Some('}') then Some(end + 1) else None
  }
  _ => None
}

## Where the text of a string that starts at from ends: at a quote, an escape,
## an interpolation or a line break; in a multi-line string, at an escape, an
## interpolation or `"""`.
fn textEnd(src: Bytes, from: Int, multiline: Bool) -> Int {
  if !multiline then return skip(src, from, fn c: c != '"' && c != '\\' && c != '{' && c != '\n')
  var i = from
  while i < src.size() {
    match src[i..i + 3] {
      ['{', ..] | ['\\', ..] | ['"', '"', '"'] => return i
      _ => i += 1
    }
  }
  i
}

## How many bytes the UTF-8 character that the bytes start with takes, as Go's
## `utf8.DecodeRune` counts them: 1 for a byte that starts no character.
fn runeSize(next: Bytes) -> Int = match next {
  [0..=127, ..] => 1
  # 0xC2..=0xDF, then 0x80..=0xBF
  [194..=223, 128..=191, ..] => 2
  # 0xE0 0xA0..=0xBF, 0xE1..=0xEC or 0xEE..=0xEF 0x80..=0xBF, 0xED 0x80..=0x9F
  [224, 160..=191, 128..=191, ..] => 3
  [225..=236 | 238..=239, 128..=191, 128..=191, ..] => 3
  [237, 128..=159, 128..=191, ..] => 3
  # 0xF0 0x90..=0xBF, 0xF1..=0xF3 0x80..=0xBF, 0xF4 0x80..=0x8F
  [240, 144..=191, 128..=191, 128..=191, ..] => 4
  [241..=243, 128..=191, 128..=191, 128..=191, ..] => 4
  [244, 128..=143, 128..=191, 128..=191, ..] => 4
  _ => 1
}

fn isSpace(c: Byte) -> Bool = match c {
  # \u{b} and \u{c} are the vertical tab and the form feed
  ' ' | '\t' | '\n' | '\r' | '\u{b}' | '\u{c}' => true
  _ => false
}

fn isDigit(c: Byte) -> Bool = c >= '0' && c <= '9'

fn isHex(c: Byte) -> Bool = match c {
  '0'..='9' | 'a'..='f' | 'A'..='F' => true
  _ => false
}

fn isNameByte(c: Byte) -> Bool = match c {
  'a'..='z' | 'A'..='Z' | '0'..='9' | '_' => true
  _ => false
}
