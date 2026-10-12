# What SPECS.md §8's lexer assumes (context=lexer): the tokens it makes.

pub type Token =
  | Arrow
  | Number(Int)
  | End
