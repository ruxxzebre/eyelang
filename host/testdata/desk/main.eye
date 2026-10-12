# A program that a Go program embeds: its extern fns are host functions,
# which host_test.go registers (SPECS.md §18).

## Shows a line on the desk's screen.
@host("lab.desk")
extern fn showOnDesk(text: String) -> Result((), String)

@host("lab.add")
extern fn add(a: Int32, b: Int32) -> Int32

## A counter the Go side keeps.
extern type Counter

@host("lab.counter")
extern fn counter(start: Int) -> Counter

@host("lab.bump")
extern fn bump(c: Counter, by: Int) -> Int

@host("lab.sum")
extern fn sum(data: Bytes) -> Result(UInt64, String)

pub fn main(args: List(String)) -> Result((), String) uses {ffi, stdio} {
  showOnDesk("hello from Eyelang")?
  let c = counter(40)
  print("{add(2, 3)} {bump(c, 1)} {bump(c, 1)}")
  let total = sum(bytes("abc"))?
  print("{total}")
  showOnDesk("")
}

job shout(text: String) -> Result((), String) {
  showOnDesk!(text)
}

test "host functions answer in tests" {
  expect(add(-7, 3) == -4)
  expect(sum(bytes("")) == Err("nothing to sum"))
  let c = counter(1)
  expect(bump(c, 2) == 3)
  expect(bump(c, 2) == 5)
}

test "a host function in a job is a step" {
  shout.start("from a job")
}
