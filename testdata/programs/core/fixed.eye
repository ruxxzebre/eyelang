# Fixed-width integers (SPECS.md §5).

test "Byte arithmetic wraps or is checked at 0 and 255" {
  let top: Byte = 255
  let zero: Byte = 0
  expect(top.wrappingAdd(1) == 0)
  expect(zero.wrappingSub(1) == 255)
  expect(top.wrappingMul(2) == 254)
  expect(top.checkedAdd(1) == None)
  expect(top.checkedSub(1) == Some(254))
  expect(zero.checkedSub(1) == None)
  let sixteen: Byte = 16
  expect(sixteen.checkedMul(16) == None)
  expect(sixteen.checkedMul(15) == Some(240))
  expect(sixteen.wrappingMul(16) == 0)
}

test "Byte bits" {
  let a: Byte = 12
  expect(a.and(10) == 8)
  expect(a.or(10) == 14)
  expect(a.xor(10) == 6)
  expect(a.not() == 243)
  let one: Byte = 1
  expect(one.shiftLeft(7) == 128)
  expect(one.shiftLeft(8) == 0)
  let three: Byte = 3
  expect(three.shiftLeft(7) == 128)  # the high bit is lost
  let top: Byte = 255
  expect(top.shiftRight(4) == 15)
  expect(top.shiftRight(8) == 0)
  expect(top.shiftRight(100) == 0)
}

test "Int32 wraps at its edges, keeping its sign" {
  let max: Int32 = 2147483647
  let min: Int32 = -2147483648
  expect(max.wrappingAdd(1) == -2147483648)
  expect(min.wrappingSub(1) == 2147483647)
  expect(max.wrappingMul(2) == -2)
  expect(max.checkedAdd(1) == None)
  expect(min.checkedSub(1) == None)
  expect(min.checkedAdd(1) == Some(-2147483647))
  let big: Int32 = 46341
  expect(big.checkedMul(big) == None)
  let fits: Int32 = 46340
  expect(fits.checkedMul(fits) == Some(2147395600))
}

test "Int32 bits are two's complement" {
  let minusOne: Int32 = -1
  let minusEight: Int32 = -8
  expect(minusOne.and(255) == 255)
  expect(minusEight.or(3) == -5)
  expect(minusEight.xor(-1) == 7)
  expect(minusOne.not() == 0)
  let one: Int32 = 1
  expect(one.shiftLeft(31) == -2147483648)
  expect(one.shiftLeft(32) == 0)
  expect(minusEight.shiftRight(1) == -4)
  expect(minusOne.shiftRight(40) == -1)
  let min: Int32 = -2147483648
  expect(min.shiftRight(31) == -1)
}

test "Int64 wraps at its edges" {
  let max: Int64 = 9223372036854775807
  let min: Int64 = -9223372036854775808
  expect(max.wrappingAdd(1) == min)
  expect(min.wrappingSub(1) == max)
  expect(max.wrappingMul(2) == -2)
  expect(max.checkedAdd(1) == None)
  expect(min.checkedSub(1) == None)
  let big: Int64 = 3037000500
  expect(big.checkedMul(big) == None)
  let fits: Int64 = 3037000499
  expect(fits.checkedMul(fits) == Some(9223372030926249001))
  let zero: Int64 = 0
  expect(zero.not() == -1)
  let one: Int64 = 1
  expect(one.shiftLeft(63) == min)
  expect(min.shiftRight(63) == -1)
  expect(max.xor(min) == -1)
}

test "UInt32 wraps at 0 and 4294967295" {
  let max: UInt32 = 4294967295
  let zero: UInt32 = 0
  expect(max.wrappingAdd(1) == 0)
  expect(zero.wrappingSub(1) == 4294967295)
  expect(max.wrappingMul(max) == 1)
  expect(max.checkedAdd(1) == None)
  expect(zero.checkedSub(1) == None)
  expect(zero.checkedAdd(7) == Some(7))
  expect(zero.not() == 4294967295)
  let one: UInt32 = 1
  expect(one.shiftLeft(31) == 2147483648)
  expect(max.shiftRight(31) == 1)  # no sign to keep
  expect(max.and(65535) == 65535)
}

test "UInt64 wraps at 0 and 18446744073709551615" {
  let max: UInt64 = 18446744073709551615
  let zero: UInt64 = 0
  expect(max.wrappingAdd(1) == 0)
  expect(zero.wrappingSub(1) == max)
  expect(max.wrappingMul(max) == 1)
  expect(max.checkedAdd(1) == None)
  expect(zero.checkedSub(1) == None)
  expect(max.checkedSub(1) == Some(18446744073709551614))
  expect(zero.not() == max)
  let one: UInt64 = 1
  expect(one.shiftLeft(63) == 9223372036854775808)
  expect(max.shiftRight(63) == 1)
  expect(max.and(255) == 255)
  expect(max.xor(max) == 0)
  expect(max.or(zero) == max)
}

test "from converts an Int when it fits, and toInt always does" {
  expect(Byte.from(255) == Some(255))
  expect(Byte.from(256) == None)
  expect(Byte.from(-1) == None)
  expect(Int32.from(2147483647) == Some(2147483647))
  expect(Int32.from(2147483648) == None)
  expect(Int32.from(-2147483648) == Some(-2147483648))
  expect(Int32.from(-2147483649) == None)
  expect(Int64.from(9223372036854775807) == Some(9223372036854775807))
  expect(Int64.from(9223372036854775808) == None)
  expect(UInt32.from(4294967295) == Some(4294967295))
  expect(UInt32.from(4294967296) == None)
  expect(UInt32.from(-1) == None)
  expect(UInt64.from(18446744073709551615) == Some(18446744073709551615))
  expect(UInt64.from(18446744073709551616) == None)
  let max: UInt64 = 18446744073709551615
  expect(max.toInt() + 1 == 18446744073709551616)
  let b: Byte = 200
  expect(b.toInt() * 2 == 400)
  expect([1, 300, 7] |> map(Byte.from) == [Some(1), None, Some(7)])
}

test "the methods are functions too" {
  let top: Byte = 255
  expect(wrappingAdd(top, 2) == 1)
  expect((top |> wrappingAdd(3)) == 2)
  expect((top |> checkedAdd(0)) == Some(255))
  expect(top.shiftRight(by: 4) == 15)
}

## FNV-1a, a hash made of xor and a multiplication that wraps.
fn fnv1a(data: Bytes) -> UInt32 {
  var hash: UInt32 = 2166136261
  for b in data {
    hash = hash.xor(UInt32.from(b.toInt()) ?? 0).wrappingMul(16777619)
  }
  hash
}

test "a hash over Bytes" {
  expect(fnv1a("".bytes()) == 2166136261)
  expect(fnv1a("a".bytes()) == 3826002220)
  expect(fnv1a("foobar".bytes()) == 3214735720)
}
