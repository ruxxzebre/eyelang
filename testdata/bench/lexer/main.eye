# The lexer benchmark, in Eyelang: testdata/programs/lexer's lexer on one big
# input, against syntax/lexer.go on the same input in main.go.
#
#   lexer TIMES FILE...
#
# joins the files' text, repeats it TIMES times, lexes the result and prints
# the number of tokens and a checksum of their kinds, spans and flags. main.go
# prints the same line, or the lexers disagree. For example, from the
# repository's root:
#
#   eye build -o /tmp/lexer-eye testdata/bench/lexer
#   /tmp/lexer-eye 100 $(git ls-files '*.eye')
#   go run testdata/bench/lexer/main.go 100 $(git ls-files '*.eye')

import fs
import lexer.{Kind, Token}

pub fn main(args: List(String)) -> Result((), Problem) uses {fs, stdio} {
  let times = parseInt(args[1] ?? "").context("usage: lexer TIMES FILE...")?
  var text = ""
  for path in args |> drop(2) {
    text += fs.readText(path).context("reading {path}")?
  }
  var input = ""
  for _ in 0..times {
    input += text
  }
  let tokens = lexer.lex(input.bytes())
  print("{tokens.size()} tokens, checksum {checksum(tokens)}")
  Ok(())
}

## A checksum of the tokens: their kinds, as syntax/token.go numbers them,
## spans and flags, mixed into a number below 1_000_000_007.
fn checksum(tokens: List(Token)) -> Int {
  var numbers: Map(Kind, Int) = {}
  for (i, kind) in Kind.variants() |> enumerate {
    numbers = numbers.insert(kind, i + 1)
  }
  var sum = 0
  for t in tokens {
    let flags = flag(t.spaceBefore, 1) + flag(t.newlineBefore, 2) + flag(t.missing, 4)
    sum = sum.mix(numbers[t.kind] ?? 0).mix(t.start).mix(t.end).mix(flags)
  }
  sum
}

fn mix(sum: Int, n: Int) -> Int = (sum * 31 + n) % 1_000_000_007

fn flag(set: Bool, value: Int) -> Int = if set then value else 0
