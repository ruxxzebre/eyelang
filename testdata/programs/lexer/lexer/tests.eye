# The lexer's own tests. interp's lexer_test.go compares it with
# syntax/lexer.go on every file of the conformance suite and the corpus.

## The kinds of the tokens of src that aren't trivia.
fn kinds(src: String) -> List(Kind) =
  lex(src.bytes()) |> filter(fn t: !isTrivia(t.kind)) |> map(fn t: t.kind)

## Each token's text.
fn texts(src: String) -> List(String) {
  let bytes = src.bytes()
  lex(bytes) |> map(fn t: bytes[t.start..t.end].toText() ?? "?")
}

test "the tokens give back the source" {
  let src = "fn main() \{\n  let s = \"a \{b} \{c + 1}\"  # note\n}\n"
  expect(texts(src) |> join("") == src)
  expect(texts("x.y ..= 'a'") == ["x", ".", "y", " ", "..=", " ", "'a'", ""])
}

test "reserved words are keywords, other keywords are names" {
  expect(kinds("let type = _ _x") == [KwLet, Ident, Assign, Underscore, Ident, EOF])
  expect(kinds("Some(x)") == [TypeIdent, LParen, Ident, RParen, EOF])
}

test "the longest match wins" {
  expect(kinds("2026-10-10 2026 - 10") == [Date, Int, Minus, Int, EOF])
  expect(kinds("07:40 23:59:30 7:40") == [Time, Time, Int, Colon, Int, EOF])
  expect(kinds("1_000 1.5 6.02e23 1.5e 1.x") == [
    Int,
    Float,
    Float,
    Float,
    Ident,
    Int,
    Dot,
    Ident,
    EOF,
  ])
  expect(kinds("a != b !c ..= .. . .\{ |> ?? &&") == [
    Ident,
    NotEq,
    Ident,
    Bang,
    Ident,
    DotDotEq,
    DotDot,
    Dot,
    DotBrace,
    PipeGt,
    QuestionQuestion,
    AndAnd,
    EOF,
  ])
}

test "an interpolation ends at the } that closes no record" {
  expect(kinds("\"a\{ \{ x: 1 } }b\"") == [
    StringStart,
    StringText,
    InterpOpen,
    LBrace,
    Ident,
    Colon,
    Int,
    RBrace,
    InterpClose,
    StringText,
    StringEnd,
    EOF,
  ])
  expect(kinds("\"\{\"\{x}\"}\"") == [
    StringStart,
    InterpOpen,
    StringStart,
    InterpOpen,
    Ident,
    InterpClose,
    StringEnd,
    InterpClose,
    StringEnd,
    EOF,
  ])
}

test "hexadecimal and binary integers" {
  expect(kinds("0xC2 0xdf_FF 0b1010 0x 0xG 0b2 0b1_") == [
    Int,
    Int,
    Int,
    Int,
    Ident,
    Int,
    Ident,
    Int,
    Ident,
    Int,
    Underscore,
    EOF,
  ])
  expect(texts("0x1F.5") == ["0x1F", ".", "5", ""])
}

test "bytes that aren't UTF-8 are illegal, one at a time" {
  # a, then 0xFF, the lead byte 0xC3 without its second byte, and 0xE0 0x80 0x80,
  # an overlong form, each byte on its own
  let src = toBytes([0x61, 0x20, 0xFF, 0xC3, 0x20, 0xE0, 0x80, 0x80])
  let tokens = lex(src) |> filter(fn t: !isTrivia(t.kind))
  expect(tokens |> map(fn t: (t.kind, t.end - t.start)) == [
    (Ident, 1),
    (Illegal, 1),
    (Illegal, 1),
    (Illegal, 1),
    (Illegal, 1),
    (Illegal, 1),
    (EOF, 0),
  ])
}

test "a multi-line string holds line breaks and quotes" {
  let src = "\"\"\"\n  say \"hi\" \{x}\n  \"\"\""
  expect(kinds(src) == [StringStart, StringText, InterpOpen, Ident, InterpClose, StringText, StringEnd, EOF])
}

test "escapes and characters" {
  expect(kinds("\"\\n\\u\{1F600}\\q\"") == [StringStart, StringEscape, StringEscape, Illegal, StringEnd, EOF])
  expect(kinds("'a' '\\n' '\\'' 'é' '' 'ab'") == [
    Char,
    Char,
    Char,
    Char,
    Illegal,
    Illegal,
    EOF,
  ])
}

test "an unterminated string is closed by a missing quote" {
  let tokens = lex("\"abc\nx".bytes())
  let closing = tokens |> find(fn t: t.kind == StringEnd)
  expect(closing |> map(fn t: (t.start, t.end, t.missing)) == Some((4, 4, true)))
  expect(kinds("\"a\{b") == [StringStart, StringText, InterpOpen, Ident, InterpClose, StringEnd, EOF])
}

test "a token knows the space and line breaks before it" {
  let tokens = lex("a b # c\n  d".bytes()) |> filter(fn t: !isTrivia(t.kind))
  let before = tokens |> map(fn t: (t.spaceBefore, t.newlineBefore))
  expect(before == [(false, false), (true, false), (true, true), (false, false)])
}

test "a character no token starts with is illegal, whole" {
  expect(texts("$ é") == ["$", " ", "é", ""])
  expect(kinds("a & b") == [Ident, Illegal, Ident, EOF])
}

test "the kinds are numbered as in syntax/token.go" {
  let all = Kind.variants()
  expect(all.first() == Some(EOF))
  expect(all.last() == Some(KwFalse))
  expect(all.size() == 74)  # syntax.KwFalse
  expect(KwFalse.index() + 1 == 74 && EOF.index() + 1 == 1)
}
