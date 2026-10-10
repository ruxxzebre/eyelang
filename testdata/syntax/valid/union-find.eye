# Union-find with path compression, as a type checker uses it.

pub opaque type Subst = { parent: Map(Int, Int), rank: Map(Int, Int) }

impl Subst {
  pub fn new() -> Subst = { parent: {}, rank: {} }

  pub fn find(mut self, v: Int) -> Int {
    match self.parent |> get(v) {
      Some(p) if p != v => {
        let root = self.find(p)
        self.parent = self.parent |> insert(v, root)
        root
      }
      _ => v
    }
  }

  pub fn union(mut self, a: Int, b: Int) {
    let (ra, rb) = (self.find(a), self.find(b))
    if ra == rb { return }
    let (low, high) = if (self.rank |> get(ra) ?? 0) < (self.rank |> get(rb) ?? 0) { (ra, rb) } else { (rb, ra) }
    self.parent = self.parent |> insert(low, high)
    self.rank = self.rank |> insert(high, (self.rank |> get(high) ?? 0) + 1)
  }
}

test "find after union" {
  var s = Subst.new()
  s.union(1, 2)
  s.union(2, 3)
  expect(s.find(1) == s.find(3))
}
