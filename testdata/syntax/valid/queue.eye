pub opaque type Queue(a) = { front: List(a), back: List(a) }

impl Queue(a) {
  pub fn empty() -> Queue(a) = { front: [], back: [] }

  pub fn push(mut self, x: a) {
    self.back = self.back |> prepend(x)
  }

  pub fn pop(mut self) -> Option(a) {
    if self.front == [] {
      self.front = self.back |> reverse
      self.back = []
    }
    let [x, ..rest] = self.front else { return None }
    self.front = rest
    Some(x)
  }
}
