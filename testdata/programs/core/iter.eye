# Lazy iteration: `xs.iter()` gives an Iter, whose pipelines build no lists in between
# (SPECS.md §5).

fn evens(xs: List(Int)) -> Iter(Int) = xs.iter() |> filter(fn n: n % 2 == 0)

test "an Iter takes only what it needs from a very large source" {
  # A trillion squares would never finish; the pipeline computes four.
  let squares = (1..1_000_000_000_000).iter() |> map(fn n: n * n) |> filter(fn n: n % 3 == 1)
  expect(squares |> take(3) |> toList == [1, 4, 16])
  expect(squares |> first == Some(1))
  expect(squares |> any(fn n: n > 100))
  expect(!(squares |> all(fn n: n < 100)))
  expect(squares |> find(fn n: n > 50) == Some(64))
  expect((0..1_000_000_000_000).iter() |> drop(5) |> take(2) |> toList == [5, 6])
  expect((0..1_000_000_000_000).iter() |> enumerate |> take(2) |> toList == [(0, 0), (1, 1)])
  let pairs = zip((1..1_000_000_000_000).iter(), ["a", "b"])
  expect(pairs |> toList == [(1, "a"), (2, "b")])
}

test "every collection gives an Iter" {
  expect([3, 1, 2].iter() |> toList == [3, 1, 2])
  expect(toSet([1, 2]).iter() |> size == 2)
  expect({ "a": 1, "b": 2 }.iter() |> map(fn (k, v): "{k}{v}") |> toList == ["a1", "b2"])
  expect((1..=3).iter() |> sum == 6)
  expect("añb".iter() |> toList == ['a', 'ñ', 'b'])
  expect("añb".chars() |> size == 3)
  expect("hi".bytes().iter() |> toList == "hi".bytes() |> toList)
  let it = [1, 2].iter()
  expect(it.iter() |> toList == [1, 2])
}

test "for walks an Iter" {
  var seen: List(Int) = []
  for n in evens([1, 2, 3, 4, 5, 6]) |> map(fn n: n * 10) {
    seen += [n]
  }
  expect(seen == [20, 40, 60])
  var found: List((Int, Char)) = []
  for (i, c) in "abc".chars() |> enumerate {
    if c == 'c' then break
    found += [(i, c)]
  }
  expect(found == [(0, 'a'), (1, 'b')])
}

test "an Iter is a value: walking it again gives the same elements" {
  let it = [1, 2, 3].iter() |> map(fn n: n + 1)
  expect(it |> toList == [2, 3, 4])
  expect(it |> toList == [2, 3, 4])
  let more = it |> filter(fn n: n > 2)
  expect(more |> toList == [3, 4])
  expect(it |> size == 3)
}

test "the functions that walk an Iter" {
  let it = [4, 1, 3].iter()
  expect(it |> reduce(0, fn (acc, n): acc + n) == 8)
  expect(it |> sort == [1, 3, 4])
  expect(it |> reverse == [3, 1, 4])
  expect(it |> last == Some(3))
  expect(it.count(fn n: n > 1) == 2)
  expect(it |> contains(3))
  expect(!(it |> isEmpty))
  expect(it |> filter(fn n: n > 9) |> isEmpty)
  expect(it |> join(", ") == "4, 1, 3")
  expect(it |> toSet == toSet([1, 3, 4]))
  expect(it |> flatMap(fn n: [n, n]) |> take(3) |> toList == [4, 4, 1])
  expect(it |> parallelMap(fn n: n * 2) == [8, 2, 6])
}
