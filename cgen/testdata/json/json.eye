# toJson, natively as the interpreter writes it (SPECS.md §17, "Values as
# JSON"): each test prints a value's JSON, and the compiled binary must
# print the same bytes.

type Level = | Calm | Busy | Swamped

type Reading = { level: Level, every: Duration, note: Option(String), at: Instant }

type Outcome =
  | Charged(id: String, at: Instant)
  | Refused(String)
  | Split(Int, Int)
  | Pending

type ReceiptId = | ReceiptId(String)

type Box(a) = { zebra: a, apple: List(a), mango: Option(a) }

type Big = { a: String, b: String, c: String, d: Map(String, Int), e: Set(Int) }

# Larger than 64 bytes, so boxed.
type Wide = { a: Int, b: Int, c: Int, d: Int, e: Int, f: Int, g: Int, h: Int, i: String }

type Shape =
  | Dot
  | Many(a: Int, b: Int, c: Int, d: Int, e: Int, f: Int, g: Int, h: Int, i: Int)

test "scalars" {
  print(42 |> toJson)
  print(-7 |> toJson)
  print(123456789012345678901234567890 |> toJson)
  print(-9223372036854775808 |> toJson)
  print(true |> toJson)
  print(false |> toJson)
  print("plain" |> toJson)
  print(1500ms |> toJson)
  print(2 h |> toJson)
  print(3 KiB |> toJson)
}

test "floats" {
  for f in [0.0, -0.0, 1.0, 1.5, 100.0, 1.0e20, 1.0e21, 123456789012345678901.0, 0.000001, 0.000000999, 1.0e-7, 1.23e-10, 3.14159, -2.5e300, 0.1, 2.0 / 3.0] {
    print(f |> toJson)
  }
}

test "strings are escaped as encoding/json escapes them" {
  print("quote \" backslash \\ slash /" |> toJson)
  print("<a href='x'>&amp;</a>" |> toJson)
  print("tab\tnewline\nreturn\r" |> toJson)
  print("bell \u{7} nul \u{0} esc \u{1b} del \u{7f}" |> toJson)
  print("line \u{2028} paragraph \u{2029}" |> toJson)
  print("café ünïcödé 日本 🎉" |> toJson)
  print("" |> toJson)
}

test "records' fields are in byte order" {
  let b: Box(Int) = { zebra: 1, apple: [2, 3], mango: None }
  print(b |> toJson)
  let s: Box(String) = { zebra: "z", apple: [], mango: Some("m") }
  print(s |> toJson)
  let big: Big = { a: "x", b: "y", c: "z", d: { "k": 1 }, e: toSet([3, 1, 2]) }
  print(big |> toJson)
}

test "variants" {
  let at = 2026-10-12 09:30 in "UTC"
  print(Charged("ch-1", at) |> toJson)
  print(Refused("no funds") |> toJson)
  print(Split(1, 2) |> toJson)
  print(Pending |> toJson)
  print([Calm, Busy, Swamped] |> toJson)
  print(ReceiptId("r-41") |> toJson)
  let wide: Wide = { a: 1, b: 2, c: 3, d: 4, e: 5, f: 6, g: 7, h: 8, i: "nine" }
  print(wide |> toJson)
  print([Dot, Many(1, 2, 3, 4, 5, 6, 7, 8, 9)] |> toJson)
}

test "Options and Results" {
  print(Some(3) |> toJson)
  let none: Option(Int) = None
  print(none |> toJson)
  let nested: Option(Option(Int)) = Some(None)
  print(nested |> toJson)
  let ok: Result(Int, String) = Ok(1)
  let err: Result(Int, String) = Err("bad")
  print(ok |> toJson)
  print(err |> toJson)
  let floats: List(Option(Float)) = [Some(1.5), None]
  print(floats |> toJson)
}

test "collections" {
  print([1, 2, 3] |> toJson)
  let empty: List(Int) = []
  print(empty |> toJson)
  print((1, "two", 3.0) |> toJson)
  let set = toSet(["b", "a"])
  print(set |> toJson)
  let none: Set(Int) = {}
  print(none |> toJson)
}

test "maps" {
  print({ "b": 1, "a": 2, "B": 3, "é": 4, "a\"": 5 } |> toJson)
  let empty: Map(String, Int) = {}
  print(empty |> toJson)
  print({ 3: "three", 1: "one" } |> toJson)
  print({ (Busy): 2, (Calm): 1 } |> toJson)
  let keys: Map(Option(String), Int) = { (Some("x")): 1, (None): 2 }
  print(keys |> toJson)
  let some: Map(Option(String), Int) = { (Some("y")): 1, (Some("x")): 2 }
  print(some |> toJson)
  print({ 2026-10-12: "monday", 2026-10-11: "sunday" } |> toJson)
  print({ ((1, 2)): "pair" } |> toJson)
}

test "times" {
  let at = 2026-10-12 09:30:05 in "UTC"
  print(at |> toJson)
  print(2026-07-01 09:30 in "Europe/Dublin" |> toJson)
  print(2026-01-01 08:30 in "Europe/Dublin" |> toJson)
  print(2026-01-01 08:30 in "America/St_Johns" |> toJson)
  print(2026-10-12 |> toJson)
  print([07:40, 14:30:15] |> toJson)
  let r: Reading = { level: Busy, every: 90s, note: None, at }
  print(r |> toJson)
}

test "NaN is a fault" {
  let zero = 0.0
  print([1.0, zero / zero] |> toJson)
}
