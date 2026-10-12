# Tests that fail, each in its own way: a compiled binary must say what
# failed, where, and what the test printed, exactly as eye test does.

type Pair = { left: Int, right: String }

fn pick(xs: List(Int), i: Int) -> Int = match xs.get(i) {
  Some(x) => x
  None => unreachable("index {i} of {xs}")
}

fn ratio(a: Int, b: Int) -> Int = match nonzero(b) {
  Some(n) => a / n
  None => 0
}

test "a comparison says what its left side was" {
  print("before the failure")
  let p: Pair = { left: 1, right: "one" }
  expect(p.left == 2)
}

test "a condition says its text" {
  let xs = [1, 2, 3]
  expect(xs |> any(fn x: x > 5))
}

test "unreachable says why" {
  log.info("looking", 7)
  expect(pick([1, 2], 5) == 0)
}

test "a passing test between failures" {
  expect(ratio(7, 2) == 3)
  expect(ratio(7, 0) == 0)
}

test "an exact integer fault" {
  let xs = [10, 20]
  let [a, b, c] = xs else { unreachable("not three: {xs}") }
  expect(a + b + c == 0)
}

test "an element out of range is a fault" {
  var xs = [1, 2, 3]
  xs[1] = 20
  xs[3] = 40
  expect(xs.size() == 4)
}
