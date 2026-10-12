# A variable updated by itself changes in place when it holds the only
# reference, and copies first when anything else holds one: what another
# name saw stays as it was.

fn grown(n: Int) -> String {
  var out = ""
  for i in 0..n {
    out += "{i},"
  }
  out
}

fn counts(words: List(String)) -> Map(String, Int) {
  var m: Map(String, Int) = {}
  for w in words {
    m = m.insert(w, (m[w] ?? 0) + 1)
  }
  m
}

test "a string grows in place" {
  expect(grown(5) == "0,1,2,3,4,")
  expect(grown(20_000).size() == 108_890)
}

test "what another name holds doesn't change" {
  var s = "ab"
  let t = s
  s += "c"
  s = "{s}d"
  expect(s == "abcd" && t == "ab")
  var xs = [1]
  let ys = xs
  xs += [2]
  xs = xs.add(3)
  expect(xs == [1, 2, 3] && ys == [1])
  var m = { "a": 1 }
  let n = m
  m = m.insert("a", 2)
  m = m.insert("b", 3)
  expect(m == { "a": 2, "b": 3 } && n == { "a": 1 })
  var seen = toSet([1])
  let before = seen
  seen = seen.add(2)
  expect(seen == toSet([1, 2]) && before == toSet([1]))
}

test "a value appended to itself" {
  var xs = [1, 2]
  xs += xs
  expect(xs == [1, 2, 1, 2])
  var s = "ab"
  s += s
  s = "{s}{s}"
  expect(s == "abababab")
}

test "a map counts in place" {
  let words = "the cat and the hat and the bat".words()
  expect(counts(words) == { "the": 3, "cat": 1, "and": 2, "hat": 1, "bat": 1 })
  var many: List(String) = []
  for i in 0..30_000 {
    many = many.add("w{i % 7}")
  }
  expect(counts(many)["w0"] == Some(4286))
}
