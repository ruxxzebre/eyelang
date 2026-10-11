# Strings, characters and the numbers read from text.

test "strings are searched and sliced by byte offsets" {
  let s = "héllo world"
  expect(s |> size == 11)     # characters
  expect(s.byteSize() == 12)  # bytes: é takes two
  expect(s.indexOf("o") == Some(5))
  expect(s |> indexOf("o", from: 6) == Some(8))
  expect(s.indexOf("zz") == None)
  expect(s.slice(0..3) == Some("hé"))
  expect(s.slice(0..2) == None)  # would split the é
  expect(s.slice(7..100) == Some("world"))
  expect(s.slice(7..=7) == Some("w"))
  let start = s.indexOf("w") ?? 0
  expect(s.slice(start..s.byteSize()) == Some("world"))
}

test "take and drop count characters" {
  expect("héllo" |> take(2) == "hé")
  expect("héllo" |> drop(2) == "llo")
  expect("hi" |> take(5) == "hi")
}

test "replace, words and the ends of strings" {
  expect("a-b-c".replace("-", "+") == "a+b+c")
  expect("aaa".replace("a", "aa") == "aaaaaa")
  expect("abc".replace("", "x") == "abc")
  expect("  one\ttwo\n three ".words() == ["one", "two", "three"])
  expect("".words() == [])
  expect("report.pdf".endsWith(".pdf"))
  expect(!"report.pdf".endsWith(".txt"))
}

test "a string's characters" {
  var codes: List(Int) = []
  for c in "A€".chars() {
    codes += [c.code()]
  }
  expect(codes == [65, 8364])
  expect(fromCode(8364) == Some('€'))
  expect(fromCode(-1) == None)
  expect(fromCode(55296) == None)  # a surrogate isn't a character
}

test "numbers read from text" {
  expect(parseInt(" 42 ") == Ok(42))
  expect(parseInt("ff", base: 16) == Ok(255))
  expect(parseInt("1_0") |> isErr)
  expect(parseFloat("1.5") == Ok(1.5))
  expect(parseFloat("-6.02e23") == Ok(-6.02e23))
  expect(parseFloat("inf") |> isErr)
  expect(parseFloat("") |> isErr)
}

test "Floats round to whole numbers" {
  expect(round(2.5) == 3.0)
  expect(round(-2.5) == -3.0)
  expect(floor(1.7) == 1.0)
  expect(ceil(1.2) == 2.0)
  expect(2.7.toInt() == Some(2))
  expect((-2.7).round().toInt() == Some(-3))
  expect((1.0 / 0.0).toInt() == None)
  expect(toFloat(3) == 3.0)
  expect(toFloat(9007199254740993) == 9007199254740992.0)
}
