# SPECS.md section 8, example 1.
pub opaque type Lexer = { src: Bytes, pos: Int, tokens: List(Token) }

impl Lexer {
  pub fn new(src: Bytes) -> Lexer = { src, pos: 0, tokens: [] }
  pub fn peek(self) -> Byte = self.src[self.pos] ?? 0
  pub fn advance(mut self) { self.pos += 1 }
}

fn skipSpaces(mut lexer: Lexer) {
  while lexer.peek() == ' ' { lexer.advance() }
}

fn largest(xs: List(a)) -> Option(a) where a: Ord {
  var best = first(xs)?                      # ? on an Option returns None
  for x in xs {
    if x > best { best = x }
  }
  Some(best)
}
