# Regular expressions: the standard module regex (SPECS.md §5). Patterns are
# RE2's syntax, as Go's regexp reads it.

import regex.{Regex, RegexError}

## A date written 2026-10-12, with its parts named.
fn date() -> Regex = must("(?P<year>\\d\{4})-(?P<month>\\d\{2})-(?<day>\\d\{2})")

## The Regex of a pattern this file knows is right.
fn must(pattern: String) -> Regex = match regex.parse(pattern) {
  Ok(re) => re
  Err(e) => unreachable("{e}")
}

test "a pattern is read with parse, and a bad one is an error that says why" {
  expect(regex.parse("a+") |> isOk)
  expect(regex.parse("a(b") == Err(BadPattern(pattern: "a(b", reason: "missing closing ) at `a(b`")))
  expect(regex.parse("x**") == Err(BadPattern(pattern: "x**", reason: "invalid nested repetition operator at `**`")))
  expect(regex.parse("(a)\\1") |> isErr)  # no backreferences
  expect(regex.parse("a(?=b)") |> isErr)  # no lookaround
  let Err(e) = regex.parse("[z-a]") else { return unreachable("the range is backwards") }
  expect("{e}" == "bad regular expression \"[z-a]\": invalid character class range at `z-a`")
}

test "a Regex is its pattern: equal when written the same, and shown as it" {
  expect(must("a+") == must("a+"))
  expect(must("a+") != must("aa*"))
  expect("{must("<[^>]*>")}" == "<[^>]*>")
  expect(toSet([must("a"), must("a"), must("b")]) |> size == 2)
}

test "matches is the whole text; find != None is anywhere" {
  let digits = must("[0-9]+")
  expect(digits.matches("2026"))
  expect(!digits.matches("year 2026"))
  expect(digits.find("year 2026") != None)
  expect(must("a|ab").matches("ab"))  # the whole text, though the leftmost match is "a"
  expect(must("(?m)^x$").matches("x"))
  expect(!must("(?m)^x$").matches("x\nx"))
  expect(must("(?i)hello").matches("HeLLo"))
  expect(must("\\Q(a)").matches("(a)"))  # \Q quotes the rest of the pattern
}

test "find gives the first match, where it is, and what its groups took" {
  let text = "due 2026-10-12, paid 2026-11-01"
  let Some(m) = date().find(text) else { return unreachable("there is a date") }
  expect(m.text == "2026-10-12")
  expect(text.slice(m.range) == Some("2026-10-12"))
  expect(m.groups == [Some("2026"), Some("10"), Some("12")])
  expect(m.named == { "year": "2026", "month": "10", "day": "12" })
  expect(m.group(0) == Some("2026-10-12"))
  expect(m.group(2) == Some("10"))
  expect(m.group(4) == None)
  expect(m.group(-1) == None)
  expect(date().find("no date") == None)
}

test "a group that took no part is None" {
  let Some(m) = must("(a)|(b)").find("b") else { return unreachable("b matches") }
  expect(m.groups == [None, Some("b")])
  expect(m.group(1) == None)
  let Some(n) = must("(?P<x>a)?b").find("b") else { return unreachable("b matches") }
  expect(n.named == {})
}

test "findAll gives every match, left to right, none overlapping" {
  let words = must("[a-zé]+").findAll("un café, deux cafés")
  expect(words |> map(fn m: m.text) == ["un", "café", "deux", "cafés"])
  expect(must("aa").findAll("aaaaa") |> size == 2)
  expect(must("x*").findAll("axb") |> map(fn m: m.text) == ["", "x", ""])
  expect(must("z").findAll("abc") == [])
}

test "replace replaces every match, as a string's replace does; replaceFirst one" {
  let spaces = must("\\s+")
  expect(spaces.replace("a  b\t\nc", " ") == "a b c")
  expect(spaces.replaceFirst("a  b  c", "_") == "a_b  c")
  expect(must("<[^>]*>").replace("<p>Fish &amp; <b>chips</b></p>", "") == "Fish &amp; chips")
  expect(must("z").replace("abc", "y") == "abc")
}

test "a replacement names groups by number or by name" {
  let swap = must("(\\w+)@(\\w+)")
  expect(swap.replace("ann@home bo@work", "$2:$1") == "home:ann work:bo")
  expect(swap.replace("ann@home", "$<2>0") == "home0")
  expect(swap.replace("ann@home", "$1x") == "annx")  # group 1, then x
  expect(swap.replace("ann@home", "$$1 costs $") == "$1 costs $")
  expect(swap.replace("ann@home", "[$3] $<2") == "[] $<2")  # no group 3, and no closing >
  expect(date().replace("on 2026-10-12", "$<day>/$<month>/$<year>") == "on 12/10/2026")
  expect(date().replace("on 2026-10-12", "$<nope>") == "on ")
  expect(must("(a)|(b)").replace("ab", "[$1$2]") == "[a][b]")
}

test "split gives the parts between matches" {
  expect(must(",\\s*").split("a, b,c") == ["a", "b", "c"])
  expect(must(",").split("a,b,") == ["a", "b", ""])
  expect(must(",").split("") == [""])
  expect(must("\\s+").split("one two  three") == ["one", "two", "three"])
}

test "escape makes a pattern that matches text as it is" {
  expect(regex.escape("1.5 (x)") == "1\\.5 \\(x\\)")
  let price = must(regex.escape("$1.50"))
  expect(price.matches("$1.50"))
  expect(!price.matches("$1x50"))
}

test "matching takes time linear in the text, whatever the pattern" {
  # (a*)*b backtracks exponentially in engines with backreferences; RE2 doesn't.
  let text = "a" |> repeat(10_000)
  expect(!must("(a*)*b").matches(text))
  expect(must("(a|aa)*").matches(text))
}

fn repeat(s: String, n: Int) -> String {
  var out = ""
  for _ in 0..n { out += s }
  out
}

## The errors parse gives can be passed on with `?`.
fn digitsIn(text: String, pattern: String) -> Result(List(String), RegexError) {
  let re = regex.parse(pattern)?
  Ok(re.findAll(text) |> map(fn m: m.text))
}

test "? passes a RegexError on" {
  expect(digitsIn("a1b22", "\\d+") == Ok(["1", "22"]))
  expect(digitsIn("a1b22", "(") |> isErr)
}
