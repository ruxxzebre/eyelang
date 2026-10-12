# A callback, a function given straight to a call, reads a var, a mut
# parameter and a mut self method's receiver: their values at the call
# (SPECS.md §7). An entity's callbacks reading its state are in
# testdata/programs/entities, since core also compiles to C.

type Post = { links: List(String) }

type Shelf = { keep: Int, items: List(Int) }

impl Shelf {
  fn others(mut self) -> List(Int) = self.items |> filter(fn x: x != self.keep)
}

fn unseen(feed: List(Post)) -> List(String) {
  var seen: Set(String) = {}
  var found: List(String) = []
  for post in feed {
    let fresh = post.links |> filter(fn l: !(seen |> contains(l)))
    seen = seen + toSet(fresh)
    found += fresh
  }
  found
}

fn below(mut xs: List(Int), limit: Int) -> List(Int) =
  xs |> filter(fn x: x < limit + (xs |> size))

test "a callback reads a var's value at the call" {
  let feed = [{ links: ["a", "b"] }, { links: ["b", "c"] }, { links: ["a", "d"] }]
  expect(unseen(feed) == ["a", "b", "c", "d"])
  var limit = 2
  let small = [1, 2, 3] |> map(fn x: [1, 2, 3] |> filter(fn y: x + y <= limit))
  expect(small == [[1], [], []])
  limit += 2
  expect([1, 2, 3] |> filter(fn x: x < limit) == [1, 2, 3])
}

test "a callback reads a mut parameter and self" {
  var xs = [1, 5, 9]
  expect(below(mut xs, 3) == [1, 5])
  var shelf: Shelf = { keep: 2, items: [1, 2, 3] }
  expect(shelf.others() == [1, 3])
}
