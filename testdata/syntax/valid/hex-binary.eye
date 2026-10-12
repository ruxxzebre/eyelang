fn literals() {
  let hex = [0x0, 0xC2, 0xdf, 0xFFFF_FFFF, 0x7f_ff]
  let binary = [0b0, 0b1010, 0b1111_0000]
  let byte: Byte = 0xE0
  let utf8 = match b {
    0x00..=0x7F => 1
    0xC2..=0xDF => 2
    _ => 0
  }
  let size = 0x10 KiB
  let mixed = 0xff + 0b1 - 10
  let negative = -0x80
}
