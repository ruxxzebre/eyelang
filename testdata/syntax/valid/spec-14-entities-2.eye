# SPECS.md section 14, example 2.
entity Ping(id: String) {
  state n: Int = 0

  pub fn ping() -> Int {
    n += 1
    let before = n   # 1
    Pong(id).pong()  # comes back to bump, which sees n == 1 and makes it 2
    n                # 2; before is still 1
  }

  pub fn bump() { n += 1 }
}

entity Pong(id: String) {
  pub fn pong() { Ping(id).bump() }
}
