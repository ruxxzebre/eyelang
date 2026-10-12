# What SPECS.md §17's Shelves example assumes (context=shelves): the
# outcomes of its commands (shelves/outcomes.eye), the racks that hold a
# shelf's items, and a log of what was done.

import shelves.{Cleared, Burned}

entity Rack(id: String) {
  state items: List(String) = []
  state gone: Bool = false

  pub fn empty() -> Int {
    let n = items |> size
    items = []
    n
  }

  pub fn burn() -> Int {
    let n = items |> size
    items = []
    gone = true
    n
  }
}

entity Log(name: String) {
  state lines: List(String) = []

  pub fn add(line: String) { lines += [line] }
}
