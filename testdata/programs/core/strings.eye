# Strings, characters and the numbers read from text.

test "strings are searched and sliced by position" {
  let city = "Málaga"
  expect(city |> size == 6)  # characters
  expect(city.slice(city.start()..city.end()) == Some("Málaga"))
  let Some(l) = city.indexOf("l") else { return unreachable("Málaga has an l") }
  expect(city.slice(city.start()..l) == Some("Má"))
  expect(city.slice(l..city.end()) == Some("laga"))
  expect(city.slice(l..=l) == Some("l"))
  let Some(a) = city |> indexOf("a", from: l) else { return unreachable("an a after the l") }
  expect(city.slice(l..a) == Some("l"))
  expect(city.indexOf("zz") == None)
  expect(city.start() < l && l < a && a < city.end())
  expect("{l}" == "byte 3")
}

test "positions step over characters" {
  let city = "Málaga"
  let Some(second) = city.after(city.start()) else { return unreachable("M") }
  expect(city.charAt(second) == Some('á'))
  expect(city.after(second) == city.indexOf("l"))  # á is two bytes
  expect(city |> before(city.end(), by: 2) == city.indexOf("ga"))
  expect(city.after(city.end()) == None)
  expect(city.before(city.start()) == None)
  expect(city.charAt(city.end()) == None)
  # A position from another string may fall inside a character.
  let Some(one) = "ab".after("ab".start()) else { return unreachable("a") }
  expect("é".slice("é".start()..one) == None)
  expect("é".charAt(one) == None)
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
