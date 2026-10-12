impl Lexer {
  fn update(mut self, s: OpenString) {
    self.strings[self.strings.size() - 1] = s
    xs[i] += 1
    grid[row][col] = '#'
    counts[word] = (counts[word] ?? 0) + 1
    points[0].x -= 1
    if late then xs[0] = 0
  }
}

fn keyword(name: Bytes) -> Option(Kind) = match name {
  "let" => Some(KwLet)
  ["..=", ..] => Some(DotDotEq)
  _ => numbers[kind.index()]
}
