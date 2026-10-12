# Reference counting in the low IR (eye check --lir): where values are
# dup'ed and dropped, which parameters are owned, and which updates are in
# place (ROADMAP 7.5).

type Bag = { count: Int, items: List(Int) }

type Tag = { name: String, n: Int }

## Only reads its list: the parameter is borrowed, and nothing is counted.
fn total(xs: List(Int)) -> Int = xs |> sum

## Adds to its list: owned, so the caller's last reference is updated in
## place.
fn push(xs: List(Int), x: Int) -> List(Int) = xs.add(x)

## Keeps its string in a record: owned.
fn label(name: String) -> Tag = { name, n: 1 }

## Reads a default it may be left out for: owned, since it is set here.
fn greet(name: String = "you") -> String = "hello, {name}"

## An append through a helper: the list's last use is the call.
fn appended(n: Int) -> List(Int) {
  var xs: List(Int) = []
  for i in 0..n {
    xs = push(xs, i)
  }
  xs
}

## A map insert through a `let`: the insert is the map's last use.
fn inserted(n: Int) -> Map(Int, Int) {
  var m: Map(Int, Int) = {}
  for i in 0..n {
    let next = m.insert(i, i)
    m = next
  }
  m
}

## `with` whose new field is made from the old one: the field is taken out
## of the record, which is then updated in place.
fn bagged(n: Int) -> Bag {
  var p: Bag = { count: 0, items: [] }
  for i in 0..n {
    p = { p with count: p.count + 1, items: p.items.add(i) }
  }
  p
}

## A field set from itself, and a compound assignment: taken too.
fn filled(n: Int) -> Bag {
  var p: Bag = { count: 0, items: [] }
  for i in 0..n {
    p.items = p.items.add(i)
    p.items += [i]
  }
  p
}

## A field moved out of a record whose field is set next: taken, not
## copied.
fn swapped(p: Bag) -> List(Int) {
  var q = p
  let old = q.items
  q.items = []
  old + q.items
}

## A field kept while its record is replaced: dup'ed where it is read.
fn replaced(p: Bag) -> List(Int) {
  var q = p
  let old = q.items
  q = { count: 0, items: [7] }
  old + q.items
}

## A value live on one way out of a branch and not on the other: dropped on
## that edge, in a block of its own since its target has another way in.
fn small(xs: List(Int)) -> Bool {
  let ys = xs.add(1)
  ys.size() > 1 && ys.size() < 5
}

## A match takes an Option apart: what it binds is borrowed from it.
fn head(xs: List(String)) -> String = match xs.first() {
  Some(s) => s
  None => ""
}

## A `mut` parameter: the caller's variable, changed in place.
fn record(mut b: Bag, x: Int) {
  b.items = b.items.add(x)
  b.count += 1
}

## What isn't used on a path is dropped on the way into it.
fn pick(xs: List(Int), ys: List(Int), first: Bool) -> List(Int) {
  let a = xs.add(1)
  let b = ys.add(2)
  if first then a else b
}

## A lambda borrows its arguments; its captures belong to the closure.
fn scaled(xs: List(Int), k: Int) -> List(Int) = xs |> map(fn x: x * k)

fn uses() -> Int {
  var b: Bag = { count: 0, items: [] }
  record(mut b, 1)
  total(appended(3)) + inserted(3).size() + bagged(3).count + filled(2).count +
    swapped(b).size() + replaced(b).size() + pick([1], [2], true).size() + scaled([1, 2], 3).size() +
    label("x").n + greet().size() + head(["a"]).size() + (if small([1]) then 1 else 0)
}
