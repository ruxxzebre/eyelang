# Byte-level code (SPECS.md open question 43): element assignment, hex and
# binary literals, string patterns over Bytes, Bytes that aren't text, and a
# variant's number. go test ./cgen runs these natively too.

import fs

type Spot = { x: Int, y: Int }

type Board = { rows: List(List(Int)), names: Map(String, Int) }

type Shade = Red | Green | Blue | Grey

fn bump(mut xs: List(Int), i: Int) {
  xs[i] += 10
}

test "an element of a var list is assigned" {
  var xs = [1, 2, 3]
  xs[0] = 7
  xs[2] += 5
  xs[1] *= 4
  expect(xs == [7, 8, 8])
  var words = ["a", "b"]
  words[1] += "c"
  expect(words == ["a", "bc"])
}

test "the list another name holds doesn't change" {
  var xs = [1, 2, 3]
  let before = xs
  xs[1] = 20
  expect(xs == [1, 20, 3] && before == [1, 2, 3])
}

test "a mut parameter's element changes the caller's list" {
  var xs = [1, 2, 3]
  bump(mut xs, 1)
  expect(xs == [1, 12, 3])
}

test "elements and fields on one path" {
  var points: List(Spot) = [{ x: 1, y: 2 }, { x: 3, y: 4 }]
  points[1].x = 30
  points[0].y += 1
  expect(points == [{ x: 1, y: 3 }, { x: 30, y: 4 }])
  var g: Board = { rows: [[0, 0], [0, 0]], names: {} }
  g.rows[1][0] = 5
  g.names["five"] = 5
  g.names["five"] = 55
  expect(g.rows == [[0, 0], [5, 0]])
  expect(g.names == { "five": 55 })
}

test "a map's value for a key is inserted or replaced" {
  var counts: Map(String, Int) = {}
  for w in ["a", "b", "a"] {
    counts[w] = (counts[w] ?? 0) + 1
  }
  expect(counts == { "a": 2, "b": 1 })
}

test "a list grown and changed in a loop" {
  var squares: List(Int) = []
  for i in 0..100 {
    squares += [0]
    squares[i] = i * i
  }
  expect(squares[99] == Some(9801) && squares.size() == 100)
}

test "hex and binary literals" {
  expect(0xFF == 255 && 0x7F_FF == 32767 && 0b1010 == 10 && 0b1111_0000 == 240)
  let b: Byte = 0xC2
  expect(b.toInt() == 194)
  let big: UInt64 = 0xFFFF_FFFF_FFFF_FFFF
  expect(big.toInt() == 18446744073709551615)
  expect(-0x80 == -128)
  let width = match b {
    0x00..=0x7F => 1
    0xC2..=0xDF => 2
    _ => 0
  }
  expect(width == 2)
}

fn keyword(name: Bytes) -> Option(Int) = match name {
  "let" => Some(1)
  "continue" => Some(2)
  "" => Some(0)
  _ => None
}

fn operator(next: Bytes) -> Int = match next {
  ["..=", ..] => 3
  ["..", ..] => 2
  ['.', ..] => 1
  [] => 0
  ["é", ..rest] => 10 + rest.size()
  _ => -1
}

test "string patterns over Bytes" {
  expect(keyword("let".bytes()) == Some(1))
  expect(keyword("continue".bytes()) == Some(2))
  expect(keyword("".bytes()) == Some(0))
  expect(keyword("lets".bytes()) == None)
  expect(operator("..=x".bytes()) == 3)
  expect(operator("..".bytes()) == 2)
  expect(operator(".5".bytes()) == 1)
  expect(operator("".bytes()) == 0)
  expect(operator("éab".bytes()) == 12)
  expect(operator("x".bytes()) == -1)
  let list: List(Byte) = [0x6C, 0x65, 0x74]
  let isLet = match list {
    "let" => true
    _ => false
  }
  expect(isLet)
}

test "Bytes from a list, and files of bytes" {
  var bytes: List(Byte) = [0xC3, 0xA9, 0xFF]
  expect(toBytes(bytes).toText() == None)
  bytes[2] = 0x21
  expect(toBytes(bytes).toText() == Some("é!"))
  expect(fs.writeBytes("data.bin", toBytes([0xFF, 0x00, 0x41])).isOk())
  match fs.readBytes("data.bin") {
    Ok(read) => expect(read.size() == 3 && read[0] == Some(0xFF) && read[2] == Some('A'))
    Err(_) => expect(false)
  }
  expect(fs.readText("data.bin").isErr())
  expect(fs.readBytes("missing.bin").isErr())
}

test "a variant's number is its place in variants()" {
  expect(Red.index() == 0 && Grey.index() == 3)
  for (i, k) in Shade.variants() |> enumerate {
    expect(k.index() == i)
  }
  let kinds = [Blue, Red]
  expect(kinds |> map(fn k: k.index()) == [2, 0])
}
