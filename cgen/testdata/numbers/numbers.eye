# Division, remainders and numbers as text, where the C back end has fast
# paths: a constant divisor, small and big Ints, fixed-width integers, and
# variants without fields.

type Light = Red | Amber | Green

type Tree = Leaf | Node(Tree, Tree)

fn depth(t: Tree) -> Int = match t {
  Leaf => 0
  Node(l, r) => 1 + max(depth(l), depth(r))
}

fn build(d: Int) -> Tree = if d == 0 then Leaf else Node(build(d - 1), build(d - 1))

fn divide(a: Int, b: Int) -> (Int, Int) = match nonzero(b) {
  Some(n) => (a / n, a % n)
  None => (0, 0)
}

test "division by a constant floors, and the remainder has the divisor's sign" {
  let xs = [-9, -7, -1, 0, 1, 7, 9, 4_611_686_018_427_387_903, -4_611_686_018_427_387_904]
  for x in xs {
    expect((x / 2, x % 2) == divide(x, 2))
    expect((x / -2, x % -2) == divide(x, -2))
    expect((x / 7, x % 7) == divide(x, 7))
    expect((x / -7, x % -7) == divide(x, -7))
    expect((x / 1, x % 1) == divide(x, 1))
    expect((x / 1000, x % 1000) == divide(x, 1000))
  }
  expect(-7 / 2 == -4)
  expect(-7 % 2 == 1)
  expect(7 % -2 == -1)
  expect(-4_611_686_018_427_387_904 / -1 == 4_611_686_018_427_387_904)
}

test "big Ints divide by a constant" {
  let big = 1_000_000_000_000_000_000_000
  expect(big / 7 == 142_857_142_857_142_857_142)
  expect(big % 7 == 6)
  expect(-big / 7 == -142_857_142_857_142_857_143)
  expect(-big % 7 == 1)
  print("{big / 3} {big % 3} {-big}")
}

test "numbers as text" {
  let small = -1234567
  let edge = 4_611_686_018_427_387_903
  let over = edge + 1
  let i32: Int32 = -2147483648
  let i64: Int64 = -9223372036854775807
  let u64: UInt64 = 18446744073709551615
  let b: Byte = 200
  print("{small} {edge} {over} {-over - 1} {0} {i32} {i64} {u64} {b}")
  expect("{small}" == "-1234567")
  expect("{over}" == "4611686018427387904")
  expect("{u64}" == "18446744073709551615")
  print("{[1, -2, 300]} {(i32, b)} {"text"}")
}

test "variants without fields" {
  let lights = Light.variants()
  expect(lights == [Red, Amber, Green])
  expect(Red == Red)
  expect(Red != Green)
  print("{lights} {Amber} {Some(Red)} {None ?? Green}")
  let t = build(5)
  expect(depth(t) == 5)
  expect(build(3) == build(3))
  expect(Leaf == Leaf)
  print("{build(1)}")
}
