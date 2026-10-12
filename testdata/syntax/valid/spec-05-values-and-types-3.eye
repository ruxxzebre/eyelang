# SPECS.md section 5, example 3.
fn utf8FromLatin1(text: Bytes) -> Bytes {
  var out: List(Byte) = []
  for b in text {
    out += if b < 0x80 then [b] else [b.shiftRight(6).or(0xC0), b.and(0x3F).or(0x80)]
  }
  toBytes(out)
}
