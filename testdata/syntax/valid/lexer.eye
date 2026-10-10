# The start of Eyelang's lexer, written in Eyelang.

pub type Kind =
  | Name
  | Number
  | Text
  | Plus
  | Minus
  | Arrow
  | Illegal

pub type Token = { kind: Kind, start: Int, end: Int }
pub type Problem = { start: Int, end: Int, message: String }

pub opaque type Lexer = { src: Bytes, pos: Int, tokens: List(Token), problems: List(Problem) }

impl Lexer {
  pub fn new(src: Bytes) -> Lexer = { src, pos: 0, tokens: [], problems: [] }

  fn peek(self, ahead: Int = 0) -> Byte = self.src[self.pos + ahead] ?? 0

  fn emit(mut self, kind: Kind, start: Int) {
    self.tokens.push({ kind, start, end: self.pos })
  }

  fn skipWhile(mut self, keep: fn(Byte) -> Bool) {
    while keep(self.peek()) {
      self.pos += 1
    }
  }

  fn string(mut self, start: Int) {
    self.pos += 1
    while self.peek() != '"' && self.peek() != '\n' && self.pos < size(self.src) {
      self.pos += 1
    }
    if self.peek() == '"' {
      self.pos += 1
    } else {
      self.problems.push({ start, end: self.pos, message: "unterminated string" })
    }
    self.emit(Text, start)
  }

  pub fn step(mut self) {
    let start = self.pos
    match self.src[self.pos..] {
      ['-', '>', ..] => { self.pos += 2; self.emit(Arrow, start) }
      ['+', ..] => { self.pos += 1; self.emit(Plus, start) }
      ['-', ..] => { self.pos += 1; self.emit(Minus, start) }
      ['"', ..] => self.string(start)
      ['0'..='9', ..] => {
        self.skipWhile(isDigit)
        self.emit(Number, start)
      }
      ['a'..='z' | '_', ..] => {
        self.skipWhile(isNameByte)
        self.emit(Name, start)
      }
      _ => { self.pos += 1; self.emit(Illegal, start) }
    }
  }
}

fn isDigit(c: Byte) -> Bool = c >= '0' && c <= '9'
fn isNameByte(c: Byte) -> Bool = isDigit(c) || (c >= 'a' && c <= 'z') || c == '_'

pub fn lex(src: Bytes) -> List(Token) {
  var lexer = Lexer.new(src)
  while lexer.pos < size(src) {
    lexer.step()
  }
  lexer.tokens
}
