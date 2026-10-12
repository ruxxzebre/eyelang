# Prints the tokens of each file named on the command line, one a line, as
# `kind start end flags`, where flags adds 1 for space before the token, 2
# for a line break before it and 4 for a missing token, as syntax.TokenFlags
# do. interp's lexer_test.go compares them with what syntax/lexer.go finds.
# With --kinds it prints the token kinds instead, in order.

import fs
import lexer.{Kind, Token}

pub fn main(args: List(String)) -> Result((), Problem) uses {fs, stdio} {
  if args[1] == Some("--kinds") {
    print(Kind.variants() |> join(" "))
    return Ok(())
  }
  for path in args |> drop(1) {
    let src = fs.readText(path).context("reading {path}")?
    let lines = lexer.lex(src.bytes()) |> map(describe)
    print("file {path}\n{lines |> join("\n")}")
  }
  Ok(())
}

fn describe(t: Token) -> String {
  let flags = flag(t.spaceBefore, 1) + flag(t.newlineBefore, 2) + flag(t.missing, 4)
  "{t.kind} {t.start} {t.end} {flags}"
}

fn flag(set: Bool, value: Int) -> Int = if set then value else 0
