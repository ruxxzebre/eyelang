package interp

import "testing"

// Benchmarks of the interpreter's constant factors: calls, records,
// variants and loops (ROADMAP R.6; the first four are from
// docs/internals/reviews/performance-and-packages-2026-10-12.md, §1.1).
// The times before and after R.6 are from a shared 4-vCPU machine, where
// runs differ by up to 30%.
//
//	go test ./interp -run '^$' -bench 'Fib|Record|Variant|ReturnBreak'

// BenchmarkFib computes fib(30): 2.7 million calls, each with an if, a
// comparison and two subtractions. Each call made five slices and a frame,
// and a closure for fib: 650 ms before R.6, 250 ms after.
func BenchmarkFib(b *testing.B) {
	runSource(b, checkSource(b, `
fn fib(n: Int) -> Int = if n < 2 then n else fib(n - 1) + fib(n - 2)

test "fib" {
  expect(fib(30) == 832040)
}
`))
}

// BenchmarkRecordWith makes a million records with `with`, which copied
// the record's names and values once per field set: 420 ms before R.6.
// Now the variable holds the only copy, which changes in place: 110 ms.
func BenchmarkRecordWith(b *testing.B) {
	runSource(b, checkSource(b, `
type Point = { x: Int, y: Int }

test "record with" {
  var p: Point = { x: 0, y: 0 }
  for i in 0..1000000 {
    p = { p with x: p.x + 1, y: i }
  }
  expect(p.x == 1000000 && p.y == 999999)
}
`))
}

// BenchmarkRecordAssign changes a record's fields a million times: 410 ms
// before R.6, 140 ms after.
func BenchmarkRecordAssign(b *testing.B) {
	runSource(b, checkSource(b, `
type Point = { x: Int, y: Int }

test "record assign" {
  var p: Point = { x: 0, y: 0 }
  for i in 0..1000000 {
    p.x += 1
    p.y += i
  }
  expect(p.x == 1000000 && p.y == 499999500000)
}
`))
}

// BenchmarkVariantEqual compares a variant with a fieldless variant a
// million times. Each use of Asleep made a new value, and == built two
// strings: 450 ms before R.6, 160 ms after.
func BenchmarkVariantEqual(b *testing.B) {
	runSource(b, checkSource(b, `
type Phase = | Awake | Asleep | Gone

test "variant equal" {
  var n = 0
  for i in 0..1000000 {
    let p = if i % 3 == 0 then Asleep else Awake
    if p == Asleep { n += 1 }
  }
  expect(n == 333334)
}
`))
}

// BenchmarkReturnBreak calls a function that leaves with return from inside
// a loop, and breaks out of a loop, 100,000 times each. Each return, break
// and continue was a panic: 550 ms before R.6, 85 ms after.
func BenchmarkReturnBreak(b *testing.B) {
	runSource(b, checkSource(b, `
fn firstOver(xs: List(Int), limit: Int) -> Int {
  for x in xs {
    if x > limit { return x }
  }
  0
}

test "return and break" {
  let xs = [1, 2, 3, 4, 5, 6, 7, 8]
  var total = 0
  for i in 0..100000 {
    total += firstOver(xs, 4)
    var j = 0
    while true {
      j += 1
      if j == 3 { break }
      if j == 1 { continue }
    }
    total += j
  }
  expect(total == 800000)
}
`))
}
