# SPECS.md section 5, example 1.
fn fnv1a(data: Bytes) -> UInt32 {
  var hash: UInt32 = 2166136261
  for b in data {
    hash = hash.xor(UInt32.from(b.toInt()) ?? 0).wrappingMul(16777619)
  }
  hash
}
