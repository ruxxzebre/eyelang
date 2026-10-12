# Updates whose last use is a call, a `let` or a record that still holds
# the old value: the slow loops of the native plan
# (docs/internals/reviews/native-plan-2026-10-12.md, section 2). The tests
# check the results on both engines; TestSlowLoops in cgen times the
# compiled main at two sizes and records how each loop grows. Until 7.5
# each step copies the whole value natively, so these loops are quadratic
# there and linear in the interpreter.

type Bag = { count: Int, items: List(Int) }

fn push(xs: List(Int), x: Int) -> List(Int) = xs.add(x)

# An append in place, as 7.4 compiles it: the loop the others are
# measured against.
fn added(n: Int) -> List(Int) {
  var xs: List(Int) = []
  for i in 0..n {
    xs = xs.add(i)
  }
  xs
}

# An append through a helper: the list's last use is a call.
fn appended(n: Int) -> List(Int) {
  var xs: List(Int) = []
  for i in 0..n {
    xs = push(xs, i)
  }
  xs
}

# A map insert through a `let`: the map's last use is the insert, but the
# result goes back through another name.
fn inserted(n: Int) -> Map(Int, Int) {
  var m: Map(Int, Int) = {}
  for i in 0..n {
    let next = m.insert(i, i)
    m = next
  }
  m
}

# `{ p with … }` whose new field is made from one of p's own: p still holds
# the old list while the new one is made.
fn bagged(n: Int) -> Bag {
  var p: Bag = { count: 0, items: [] }
  for i in 0..n {
    p = { p with count: p.count + 1, items: p.items.add(i) }
  }
  p
}

# The size of what a loop made, for timing it from the command line:
# `updates append 40000`.
fn run(loop: String, n: Int) -> Int {
  match loop {
    "add" => added(n).size()
    "append" => appended(n).size()
    "insert" => inserted(n).size()
    "with" => bagged(n).items.size()
    _ => -1
  }
}

pub fn main(args: List(String)) uses {stdio} {
  let n = match parseInt(args[2] ?? "") {
    Ok(n) => n
    Err(_) => 1000
  }
  print(run(args[1] ?? "", n))
}

test "an append in place" {
  let xs = added(3000)
  expect(xs.size() == 3000 && xs[2999] == Some(2999))
}

test "an append through a helper" {
  let xs = appended(3000)
  expect(xs.size() == 3000)
  expect(xs[0] == Some(0) && xs[2999] == Some(2999))
  expect(appended(0) == [])
}

test "a map insert through a let" {
  let m = inserted(3000)
  expect(m.size() == 3000)
  expect(m[1234] == Some(1234) && m[3000] == None)
}

test "a record updated from its own field" {
  let p = bagged(3000)
  expect(p.count == 3000 && p.items.size() == 3000)
  expect(p.items[2999] == Some(2999))
}

test "the command line names a loop" {
  expect(run("add", 10) == 10)
  expect(run("append", 10) == 10)
  expect(run("insert", 10) == 10)
  expect(run("with", 10) == 10)
  expect(run("other", 10) == -1)
}
