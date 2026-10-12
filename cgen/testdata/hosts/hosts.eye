# The standard modules' host functions that native code draws or reads as
# the interpreter does (interp/system.go): random numbers from the same seed
# in every test, and a file's lines.

import random

type Card = { suit: String, rank: Int }

test "random numbers stay in their range, and repeat from test to test" {
  var rolls: List(Int) = []
  for _ in 0..200 {
    rolls += [random.int(1..=6) ?? 0]
  }
  expect(rolls |> all(fn r: r >= 1 && r <= 6))
  expect(rolls |> toSet |> size == 6)
  expect(rolls |> take(5) == [5, 4, 5, 5, 2])  # the seed is the same in every test
  expect(random.int(3..3) == None)
  let f = random.float()
  expect(f >= 0.0 && f < 1.0)
  expect(random.pick([7]) == Some(7))
  let none: List(Int) = []
  expect(random.pick(none) == None)
  expect(random.shuffle([1, 2, 3]) |> sort == [1, 2, 3])
}

test "each test starts from the same seed" {
  expect(random.int(1..=6) == Some(5))
  expect(random.int(1..=6) == Some(4))
}

test "the numbers drawn are the interpreter's" {
  print("{random.int(0..10)} {random.int(-5..=5)} {random.int(0..8)} {random.int(1..1000000007)}")
  print("{random.int(-9223372036854775808..=9223372036854775807)} {random.int(0..=9223372036854775807)}")
  print("{random.int(5..=4)} {random.int(10..2)}")
  print("{random.float()} {random.float()} {random.float()}")
  print("{random.pick(["a", "b", "c", "d"])} {random.pick([1.5, 2.5])}")
  let cards: List(Card) = [{ suit: "hearts", rank: 1 }, { suit: "spades", rank: 12 }, { suit: "clubs", rank: 7 }]
  print("{random.pick(cards)}")
  print("{random.shuffle(cards)}")
  print("{random.shuffle((0..20) |> toList)}")
  print("{random.shuffle(["x"])} {random.shuffle(["only", "two"])}")
  let words = ["one", "two", "three", "four", "five", "six", "seven"]
  let shuffled = random.shuffle(words)
  expect(shuffled |> sort == words |> sort)
  expect(words == ["one", "two", "three", "four", "five", "six", "seven"])
  print("{shuffled}")
}

test "a range too large for 64 bits is a fault" {
  print("{random.int(0..100000000000000000000)}")
}
