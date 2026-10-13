# Saved values: every module-level let here is saved by compiled code and
# read by the interpreter, and saved by the interpreter and read by
# compiled code, and each side must write the other's bytes exactly
# (cgen/save_test.go). The test prints them, so the cross-check compares how
# they show too.

type Level = | Calm | Busy | Swamped

type Point = { x: Int, y: Int }

type Shape =
  | Dot
  | Circle(center: Point, radius: Float)
  | Poly(List(Point))

# Larger than 64 bytes, so boxed.
type Wide = { a: Int, b: Int, c: Int, d: Int, e: Int, f: Int, g: Int, h: Int, label: String }

type Tree = | Leaf | Node(Tree, Int, Tree)

type Id = | Id(String)

type Box(a) = { item: a, more: List(a), maybe: Option(a) }

type Fields = { zone: Zone, key: Key, when: Instant, day: Date, at: Time }

let small = 42
let negative = -7
let large = 4611686018427387904
let huge = -123456789012345678901234567890
let float = 3.25
let negativeZero = -0.0
let text = "héllo\nworld"
let empty = ""
let yes = true
let no = false
let letter = 'é'
let unit = ()
let byte: Byte = 200
let int32: Int32 = -5
let uint32: UInt32 = 4000000000
let int64: Int64 = -9223372036854775808
let uint64: UInt64 = 18446744073709551615
let duration = 90 s
let size = 3 KiB
let date = 2026-10-12
let time = 14:30:15
let instant = 2026-10-12 09:30 in "Europe/Dublin"
let utc = 2026-10-12 09:30:07 in "UTC"
let bytes = "abc".bytes()
let list = [1, 2, 3]
let none: List(Int) = []
let set = toSet(["b", "a"])
let map = { "b": 2, "a": 1 }
let emptyMap: Map(String, Int) = {}
let emptySet: Set(Int) = {}
let pairs = { 3: [1.5], 1: [] }
let tuple = (1, "two", 3.0)
let point: Point = { x: 1, y: -2 }
let level = Busy
let shapes = [Dot, Circle({ x: 0, y: 0 }, 1.5), Poly([{ x: 1, y: 1 }, { x: 2, y: 3 }])]
let wide: Wide = { a: 1, b: 2, c: 3, d: 4, e: 5, f: 6, g: 7, h: 8, label: "wide" }
let tree = Node(Node(Leaf, 1, Leaf), 2, Node(Leaf, 3, Leaf))
let id = Id("r-41")
let boxed: Box(String) = { item: "a", more: ["b", "c"], maybe: None }
let boxedPoint: Box(Point) = { item: { x: 1, y: 2 }, more: [], maybe: Some({ x: 3, y: 4 }) }
let maybe = Some(3)
let nothing: Option(String) = None
let nested: Option(Option(Int)) = Some(None)
let maybeFloat: Option(Float) = Some(2.5)
let ok: Result(Int, String) = Ok(1)
let err: Result(Int, String) = Err("bad")
let range = 1..5
let through = 1..=5
let offsets = "héllo".start().."héllo".end()
let positive = nonzero(5)
let fields: Fields = { zone: "Europe/Dublin", key: "k-1", when: 2026-01-01 08:00 in "Europe/Dublin", day: 2026-02-28, at: 07:40 }

test "the saved values" {
  print(small, negative, large, huge, float, negativeZero, text, empty, yes, no, letter, unit)
  print(byte, int32, uint32, int64, uint64, duration, size, date, time, instant, utc)
  print(bytes, list, none, set, map, emptyMap, emptySet, pairs, tuple)
  print(point, level, shapes, wide, tree, id, boxed, boxedPoint)
  print(maybe, nothing, nested, maybeFloat, ok, err, positive, fields)
}

test "the saved ranges" {
  var total = 0
  for i in range {
    total += i
  }
  for i in through {
    total += i
  }
  expect(total == 25)
  expect("héllo".slice(offsets) == Some("héllo"))
}
