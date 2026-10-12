# Tokens: their kinds, and the words and operators the lexer knows.

## What a token is. The kinds are declared in the order of `syntax/token.go`,
## after its `Invalid`, so a kind's place in `Kind.variants()`, counting from
## 1, is its number there. The literals are named `IntLiteral` and so on,
## since a variant named `Int` would hide the type.
pub type Kind =
  | EOF
  | Whitespace
  | Comment
  ## A character or escape that no token starts with.
  | Illegal
  ## A value, function, field or module name: `world`, `markAsleep`.
  | Ident
  ## A type, variant, entity or contract name: `World`, `Some`.
  | TypeIdent
  | IntLiteral
  | FloatLiteral
  | DateLiteral
  | TimeLiteral
  | CharLiteral
  | StringStart
  ## Text between a string's escapes and interpolations.
  | StringText
  | StringEscape
  ## The closing quote; zero width and missing when the string is unterminated.
  | StringEnd
  ## A `{` inside a string.
  | InterpOpen
  ## The `}` that ends an interpolation.
  | InterpClose
  | LParen
  | RParen
  | LBracket
  | RBracket
  | LBrace
  | RBrace
  | Comma
  | Colon
  | Semicolon
  | Dot
  | DotDot
  | DotDotEq
  | DotBrace
  | At
  | Arrow
  | FatArrow
  | Assign
  | PlusEq
  | MinusEq
  | StarEq
  | SlashEq
  | PercentEq
  | Eq
  | NotEq
  | Lt
  | LtEq
  | Gt
  | GtEq
  | Plus
  | Minus
  | Star
  | Slash
  | Percent
  | Bang
  | Question
  | QuestionQuestion
  | Pipe
  | PipeGt
  | OrOr
  | AndAnd
  | Underscore
  | KwLet
  | KwVar
  | KwMut
  | KwFn
  | KwPub
  | KwReturn
  | KwIf
  | KwElse
  | KwMatch
  | KwFor
  | KwIn
  | KwWhile
  | KwBreak
  | KwContinue
  | KwTrue
  | KwFalse

## One token: its kind, the bytes it covers, `start` up to but not including
## `end`, and what comes before it. Concatenating the tokens' bytes gives back
## the source.
pub type Token = {
  kind: Kind,
  start: Int,
  end: Int,
  ## Whitespace or a comment comes between this token and the previous one.
  spaceBefore: Bool,
  ## A line break comes between this token and the previous one.
  newlineBefore: Bool,
  ## The token isn't in the source: the lexer added it, zero wide, to close
  ## a string that wasn't closed.
  missing: Bool,
}

## Whether the kind is whitespace or a comment, which the parser skips.
pub fn isTrivia(kind: Kind) -> Bool = match kind {
  Whitespace | Comment => true
  _ => false
}

## The reserved word a name spells, if it is one. Every other keyword, such as
## `type` or `with`, is a name to the lexer, and the parser recognises it.
fn keyword(name: Bytes) -> Option(Kind) = match name {
  ['l', 'e', 't'] => Some(KwLet)
  ['v', 'a', 'r'] => Some(KwVar)
  ['m', 'u', 't'] => Some(KwMut)
  ['f', 'n'] => Some(KwFn)
  ['p', 'u', 'b'] => Some(KwPub)
  ['r', 'e', 't', 'u', 'r', 'n'] => Some(KwReturn)
  ['i', 'f'] => Some(KwIf)
  ['e', 'l', 's', 'e'] => Some(KwElse)
  ['m', 'a', 't', 'c', 'h'] => Some(KwMatch)
  ['f', 'o', 'r'] => Some(KwFor)
  ['i', 'n'] => Some(KwIn)
  ['w', 'h', 'i', 'l', 'e'] => Some(KwWhile)
  ['b', 'r', 'e', 'a', 'k'] => Some(KwBreak)
  ['c', 'o', 'n', 't', 'i', 'n', 'u', 'e'] => Some(KwContinue)
  ['t', 'r', 'u', 'e'] => Some(KwTrue)
  ['f', 'a', 'l', 's', 'e'] => Some(KwFalse)
  ['_'] => Some(Underscore)
  _ => None
}

## The operator or delimiter the bytes start with, and how many bytes it
## takes, or `None` when they start with none. The longest match wins: `..=`
## before `..`, and `..` before `.`. Braces are the lexer's own, since they
## open and close interpolations.
fn punctuation(next: Bytes) -> Option((Kind, Int)) = match next {
  ['.', '.', '=', ..] => Some((DotDotEq, 3))
  ['.', '.', ..] => Some((DotDot, 2))
  ['.', '{', ..] => Some((DotBrace, 2))
  ['-', '>', ..] => Some((Arrow, 2))
  ['-', '=', ..] => Some((MinusEq, 2))
  ['=', '=', ..] => Some((Eq, 2))
  ['=', '>', ..] => Some((FatArrow, 2))
  ['!', '=', ..] => Some((NotEq, 2))
  ['<', '=', ..] => Some((LtEq, 2))
  ['>', '=', ..] => Some((GtEq, 2))
  ['+', '=', ..] => Some((PlusEq, 2))
  ['*', '=', ..] => Some((StarEq, 2))
  ['/', '=', ..] => Some((SlashEq, 2))
  ['%', '=', ..] => Some((PercentEq, 2))
  ['?', '?', ..] => Some((QuestionQuestion, 2))
  ['|', '>', ..] => Some((PipeGt, 2))
  ['|', '|', ..] => Some((OrOr, 2))
  ['&', '&', ..] => Some((AndAnd, 2))
  ['(', ..] => Some((LParen, 1))
  [')', ..] => Some((RParen, 1))
  ['[', ..] => Some((LBracket, 1))
  [']', ..] => Some((RBracket, 1))
  [',', ..] => Some((Comma, 1))
  [':', ..] => Some((Colon, 1))
  [';', ..] => Some((Semicolon, 1))
  ['@', ..] => Some((At, 1))
  ['.', ..] => Some((Dot, 1))
  ['-', ..] => Some((Minus, 1))
  ['=', ..] => Some((Assign, 1))
  ['!', ..] => Some((Bang, 1))
  ['<', ..] => Some((Lt, 1))
  ['>', ..] => Some((Gt, 1))
  ['+', ..] => Some((Plus, 1))
  ['*', ..] => Some((Star, 1))
  ['/', ..] => Some((Slash, 1))
  ['%', ..] => Some((Percent, 1))
  ['?', ..] => Some((Question, 1))
  ['|', ..] => Some((Pipe, 1))
  _ => None
}
